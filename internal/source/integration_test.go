package source

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/lazypanic999/restivra/internal/verify"
)

// This is a real S3-compatible object server and real pg_dump/pg_restore, not
// an HTTP mock. The pinned server is test infrastructure, not a bundled service.
const IntegrationImage = "rustfs/rustfs:1.0.5"

func testID(t *testing.T) string {
	t.Helper()
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(id)
}

func TestIntegrationS3Restore(t *testing.T) {
	if os.Getenv("RESTIVRA_INTEGRATION") != "1" {
		t.Skip("set RESTIVRA_INTEGRATION=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	d := verify.Docker{}
	storageName := "restivra-s3-test-" + testID(t)
	databaseName := "restivra-s3-dump-" + testID(t)
	access, secret := "qa"+testID(t), testID(t)+testID(t)
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		for _, name := range []string{storageName, databaseName} {
			if _, err := d.Run(cleanup, nil, "rm", "--force", "--volumes", name); err != nil {
				t.Error("could not clean S3 fixture")
			}
		}
	}()
	if _, err := d.Run(ctx, nil, "run", "--detach", "--name", storageName, "--publish", "127.0.0.1::9000", "--memory", "1024m", "--cpus", "1", "--pids-limit", "256", "--tmpfs", "/data:rw,nosuid,nodev,size=128m,mode=0777", "--env", "RUSTFS_ACCESS_KEY="+access, "--env", "RUSTFS_SECRET_KEY="+secret, "--env", "RUSTFS_CONSOLE_ENABLE=false", "--env", "RUSTFS_OBS_LOGGER_LEVEL=error", IntegrationImage, "/data"); err != nil {
		t.Fatal("could not start pinned S3-compatible storage fixture")
	}
	port, err := d.Run(ctx, nil, "port", storageName, "9000/tcp")
	if err != nil {
		t.Fatal("could not discover S3 fixture port")
	}
	_, number, err := net.SplitHostPort(strings.TrimSpace(port))
	if err != nil {
		t.Fatal("invalid S3 fixture binding")
	}
	profile := Profile{ID: "qa-offsite", Name: "QA storage", Endpoint: "http://127.0.0.1:" + number, Region: "us-east-1", AccessKeyID: access, SecretAccessKey: secret, AllowHTTP: true, PathStyle: true}
	client := profile.Client()
	ready := false
	for attempt := 0; attempt < 50; attempt++ {
		request, stop := context.WithTimeout(ctx, time.Second)
		_, err = client.ListBuckets(request, &s3.ListBucketsInput{})
		stop()
		if err == nil {
			ready = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !ready {
		t.Fatal("S3 fixture did not become ready")
	}
	bucket := "restivra-qa-" + testID(t)
	if _, err = client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal("could not create synthetic bucket")
	}
	if _, err = d.Run(ctx, nil, "run", "--detach", "--name", databaseName, "--network", "none", "--memory", "512m", "--env", "POSTGRES_HOST_AUTH_METHOD=trust", verify.Image); err != nil {
		t.Fatal("could not start dump producer")
	}
	ready = false
	for attempt := 0; attempt < 90; attempt++ {
		_, err = d.Run(ctx, nil, "exec", databaseName, "sh", "-c", `test "$(cat /proc/1/comm)" = postgres && pg_isready -q -U postgres`)
		if err == nil {
			ready = true
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !ready {
		t.Fatal("dump producer not ready")
	}
	if _, err = d.Run(ctx, nil, "exec", databaseName, "psql", "-X", "-U", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "CREATE TABLE customers(id int PRIMARY KEY); INSERT INTO customers VALUES(1),(2);"); err != nil {
		t.Fatal("source schema failed")
	}
	dump, err := d.Run(ctx, nil, "exec", databaseName, "pg_dump", "-U", "postgres", "-Fc", "postgres")
	if err != nil {
		t.Fatal("source dump failed")
	}
	put := func(key string, data []byte) {
		t.Helper()
		if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: bytes.NewReader(data), ContentLength: aws.Int64(int64(len(data)))}); err != nil {
			t.Fatal("synthetic object upload failed")
		}
	}
	put("daily/current.dump", []byte(dump))
	// An unfinished extension must not win selection over a completed dump.
	put("daily/new.dump.partial", []byte("unfinished"))
	selection := Selection{Kind: "s3", Profile: profile.ID, Bucket: bucket}
	directory := t.TempDir()
	config := verify.DefaultConfig()
	config.Checks = []verify.Check{{Name: "restored customer rows", SQL: "SELECT count(*)=2 FROM public.customers"}}
	runner := verify.Runner{WorkDir: directory, SnapshotBackup: func(c context.Context, pattern, work string, max int64) (*os.File, verify.Backup, error) {
		return Snapshot(c, client, selection, pattern, work, max)
	}}
	report := runner.Run(ctx, "daily/*.dump", config)
	if report.Status != "passed" || report.CleanupStatus != "passed" || report.Backup.Source == nil || report.Backup.Source.Key != "daily/current.dump" {
		listed, _ := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket), Prefix: aws.String("daily/current.dump")})
		head, _ := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("daily/current.dump")})
		if listed != nil && len(listed.Contents) > 0 && head != nil {
			t.Logf("synthetic metadata list time=%v head time=%v list tag=%q head tag=%q list bytes=%d head bytes=%d", listed.Contents[0].LastModified, head.LastModified, aws.ToString(listed.Contents[0].ETag), aws.ToString(head.ETag), aws.ToInt64(listed.Contents[0].Size), aws.ToInt64(head.ContentLength))
		}
		t.Fatalf("real S3 restore failed at %s: %s", report.Stage, report.Error)
	}
	expected := sha256.Sum256([]byte(dump))
	if report.Backup.SHA256 != hex.EncodeToString(expected[:]) {
		t.Fatal("downloaded source fingerprint differs")
	}
	entries, _ := os.ReadDir(directory)
	if len(entries) != 0 {
		t.Fatal("private S3 snapshot leaked")
	}
	// Real conditional GET must reject an overwrite after HEAD.
	changing := &overwriteClient{S3Client: client, beforeGet: func() { put("daily/current.dump", append([]byte(dump), 'x')) }}
	_, _, err = Snapshot(ctx, changing, selection, "daily/current.dump", directory, config.MaxBackupBytes)
	coded, ok := err.(*Error)
	if !ok || coded.Code != "source_changed" {
		t.Fatal("provider did not enforce immutable conditional selection")
	}
	denied := profile
	denied.SecretAccessKey = "wrong-" + testID(t)
	_, err = Latest(ctx, denied.Client(), bucket, "daily/*.dump")
	coded, ok = err.(*Error)
	if !ok || coded.Code != "source_access_denied" || strings.Contains(err.Error(), secret) {
		t.Fatal("bad credentials were not safely rejected")
	}
	t.Log("real S3-compatible storage, PostgreSQL restore, SQL check, SHA-256, conditional overwrite rejection, denied credentials and cleanup passed")
}

type overwriteClient struct {
	S3Client
	beforeGet func()
}

func (c *overwriteClient) GetObject(ctx context.Context, input *s3.GetObjectInput, options ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	c.beforeGet()
	return c.S3Client.GetObject(ctx, input, options...)
}
