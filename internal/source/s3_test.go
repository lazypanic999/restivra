package source

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type fakeS3 struct {
	pages             []*s3.ListObjectsV2Output
	calls             int
	head              *s3.HeadObjectOutput
	body              *s3.GetObjectOutput
	headErr, errorGet error
	input             *s3.GetObjectInput
}

func (f *fakeS3) ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	f.calls++
	if f.calls > len(f.pages) {
		return nil, errors.New("unexpected page request")
	}
	return f.pages[f.calls-1], nil
}
func (f *fakeS3) HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return f.head, f.headErr
}
func (f *fakeS3) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	f.input = in
	return f.body, f.errorGet
}
func object(key string, at time.Time) types.Object {
	return types.Object{Key: aws.String(key), LastModified: &at, ETag: aws.String(`"tag"`), Size: aws.Int64(12)}
}

func TestLatestIncludesAllPagesAndDeterministicTies(t *testing.T) {
	at := time.Now().UTC()
	f := &fakeS3{pages: []*s3.ListObjectsV2Output{{Contents: []types.Object{object("daily/a.dump", at), object("daily/new.dump.partial", at.Add(time.Hour))}, IsTruncated: aws.Bool(true), NextContinuationToken: aws.String("page2")}, {Contents: []types.Object{object("daily/z.dump", at), object("daily/old.dump", at.Add(-time.Hour))}}}}
	selected, err := Latest(context.Background(), f, "backups", "daily/*.dump")
	if err != nil || aws.ToString(selected.Key) != "daily/z.dump" || f.calls != 2 {
		t.Fatal("incomplete/deterministically wrong selection")
	}
	f = &fakeS3{pages: []*s3.ListObjectsV2Output{{IsTruncated: aws.Bool(true), NextContinuationToken: aws.String("same")}, {IsTruncated: aws.Bool(true), NextContinuationToken: aws.String("same")}}}
	_, err = Latest(context.Background(), f, "backups", "*.dump")
	var coded *Error
	if !errors.As(err, &coded) || coded.Code != "source_selection_limit" {
		t.Fatal("non-progressing pagination accepted")
	}
	f = &fakeS3{pages: []*s3.ListObjectsV2Output{{Contents: []types.Object{object("other.dump", at)}}}}
	_, err = Latest(context.Background(), f, "backups", "daily/*.dump")
	if !errors.As(err, &coded) || coded.Code != "source_not_found" {
		t.Fatal("missing matches did not fail")
	}
}

func fixtureS3() (*fakeS3, []byte) {
	data := []byte("PGDMPfixture")
	at := time.Now().UTC().Truncate(time.Second)
	return &fakeS3{pages: []*s3.ListObjectsV2Output{{Contents: []types.Object{object("daily/current.dump", at)}}}, head: &s3.HeadObjectOutput{ContentLength: aws.Int64(int64(len(data))), LastModified: &at, ETag: aws.String(`"tag"`), VersionId: aws.String("immutable-version")}, body: &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(data)), ContentLength: aws.Int64(int64(len(data))), LastModified: &at, ETag: aws.String(`"tag"`), VersionId: aws.String("immutable-version")}}, data
}

func TestSnapshotUsesConditionalVersionAndVerifiedBytes(t *testing.T) {
	f, data := fixtureS3()
	// Listing uses ISO timestamps with milliseconds; HTTP HEAD dates don't.
	listedTime := f.pages[0].Contents[0].LastModified.Add(323 * time.Millisecond)
	f.pages[0].Contents[0].LastModified = &listedTime
	directory := t.TempDir()
	selection := Selection{Kind: "s3", Profile: "offsite", Bucket: "backups"}
	snapshot, b, err := Snapshot(context.Background(), f, selection, "daily/*.dump", directory, 1024)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	defer os.Remove(snapshot.Name())
	if aws.ToString(f.input.IfMatch) != `"tag"` || aws.ToString(f.input.VersionId) != "immutable-version" {
		t.Fatal("conditional/version headers omitted")
	}
	actual, err := io.ReadAll(snapshot)
	if err != nil || !bytes.Equal(actual, data) {
		t.Fatal("snapshot differs")
	}
	hash := sha256.Sum256(data)
	if b.SHA256 != hex.EncodeToString(hash[:]) || b.Source.Key != "daily/current.dump" || b.Source.VersionID != "immutable-version" {
		t.Fatal("source fingerprint evidence differs")
	}
	for _, test := range []struct {
		name   string
		mutate func(*fakeS3)
		max    int64
		code   string
	}{
		{"too large", func(f *fakeS3) {}, 5, "source_too_large"},
		{"changed since listing", func(f *fakeS3) { f.head.ETag = aws.String(`"new"`) }, 1024, "source_changed"},
		{"changed GET", func(f *fakeS3) { f.body.ETag = aws.String(`"new"`) }, 1024, "source_changed"},
		{"wrong version", func(f *fakeS3) { f.body.VersionId = aws.String("other") }, 1024, "source_changed"},
		{"short body", func(f *fakeS3) { f.body.Body = io.NopCloser(strings.NewReader("PGDMP")) }, 1024, "source_download_failed"},
		{"bad archive", func(f *fakeS3) { f.body.Body = io.NopCloser(strings.NewReader("not-an-archi")) }, 1024, "source_download_failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, _ := fixtureS3()
			test.mutate(f)
			dir := t.TempDir()
			_, _, err := Snapshot(context.Background(), f, selection, "daily/*.dump", dir, test.max)
			var coded *Error
			if !errors.As(err, &coded) || coded.Code != test.code {
				t.Fatalf("wrong safe failure: %v", err)
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 0 {
				t.Fatal("partial snapshot leaked")
			}
		})
	}
}

func TestProviderDiagnosticsAreSanitized(t *testing.T) {
	for _, status := range []int{301, 403, 404, 412, 503} {
		raw := &smithyhttp.ResponseError{Response: &smithyhttp.Response{Response: &http.Response{StatusCode: status}}, Err: errors.New("https://private-key:private-secret@provider/secret-query")}
		err := safeError(raw)
		if strings.Contains(err.Error(), "private-") || strings.Contains(err.Error(), "provider/") {
			t.Fatal("upstream diagnostics leaked")
		}
		if status == 403 && err.(*Error).Code != "source_access_denied" {
			t.Fatal("access denied not classified")
		}
	}
}

func TestCancellationStopsHTTPDownloadAndRemovesPartialSnapshot(t *testing.T) {
	started := make(chan struct{})
	closed := make(chan struct{})
	at := time.Now().UTC().Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Query().Get("list-type") == "2" {
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>daily/current.dump</Key><LastModified>%s</LastModified><ETag>"fixture"</ETag><Size>1048576</Size></Contents></ListBucketResult>`, at.Format(time.RFC3339))
			return
		}
		w.Header().Set("ETag", `"fixture"`)
		w.Header().Set("Last-Modified", at.Format(http.TimeFormat))
		w.Header().Set("Content-Length", "1048576")
		if r.Method == "HEAD" {
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("PGDMP"))
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
		close(closed)
	}))
	defer server.Close()
	p := Profile{Endpoint: server.URL, Region: "us-east-1", AccessKeyID: "test-key", SecretAccessKey: "test-secret", PathStyle: true, AllowHTTP: true}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	directory := t.TempDir()
	done := make(chan error, 1)
	go func() {
		_, _, err := Snapshot(ctx, p.Client(), Selection{Kind: "s3", Profile: "test", Bucket: "backups"}, "daily/*.dump", directory, 2<<20)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("download did not start")
	}
	cancel()
	select {
	case err := <-done:
		var coded *Error
		if !errors.As(err, &coded) || coded.Code != "cancelled" {
			t.Fatal("download cancellation not classified safely")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("download ignored cancellation")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("cancelled HTTP stream stayed open")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatal("cancelled partial snapshot remained")
	}
}
