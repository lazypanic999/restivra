package verify

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The TOC fits in a small read, but the full archive exceeds stdin/attach buffers.
// This exercises the real Docker CLI, not a fake commander that consumes stdin.
func TestIntegrationLargeArchiveListing(t *testing.T) {
	if os.Getenv("RESTIVRA_INTEGRATION") != "1" {
		t.Skip("set RESTIVRA_INTEGRATION=1; docker pull " + Image)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	id := make([]byte, 8)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	name := "restivra-listing-test-" + hex.EncodeToString(id)
	d := Docker{}
	defer func() {
		clean, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		if _, err := d.Run(clean, nil, "rm", "--force", "--volumes", name); err != nil {
			t.Error("listing fixture cleanup failed")
		}
	}()
	if _, err := d.Run(ctx, nil, "run", "--detach", "--name", name, "--network", "none", "--memory", "512m", "--cpus", "1", "--pids-limit", "128", "--label", "restivra.listing-test=true", "--env", "POSTGRES_HOST_AUTH_METHOD=trust", Image); err != nil {
		t.Fatal("listing fixture start failed")
	}
	for {
		if _, err := d.Run(ctx, nil, "exec", name, "sh", "-c", `test "$(cat /proc/1/comm)" = postgres && pg_isready -q -U postgres`); err == nil {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("listing fixture readiness timed out")
		}
		time.Sleep(250 * time.Millisecond)
	}
	sql := `CREATE TABLE listing_payload AS SELECT i, string_agg(md5((i::bigint*32+g)::text),'') AS payload FROM generate_series(1,10000) i CROSS JOIN generate_series(1,32) g GROUP BY i`
	if _, err := d.Run(ctx, nil, "exec", name, "psql", "-X", "-U", "postgres", "-v", "ON_ERROR_STOP=1", "-c", sql); err != nil {
		t.Fatal("listing fixture data failed")
	}
	if _, err := d.Run(ctx, nil, "exec", name, "pg_dump", "-U", "postgres", "-Fc", "-f", "/tmp/listing.dump", "postgres"); err != nil {
		t.Fatal("listing fixture dump failed")
	}
	archive := filepath.Join(t.TempDir(), "large.dump")
	if _, err := d.Run(ctx, nil, "cp", name+":/tmp/listing.dump", archive); err != nil {
		t.Fatal("listing fixture copy failed")
	}
	file, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || stat.Size() < 4<<20 {
		t.Fatal("fixture must exceed attach/stdin buffering")
	}
	t.Logf("listing fixture: %d bytes; 20 valid listings, corrupt input and cancellation", stat.Size())
	before := sha256.New()
	if _, err := io.Copy(before, file); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		listing, err := d.Run(ctx, file, "exec", "--interactive", name, "sh", "-c", archiveListScript)
		if err != nil || major(archiveVersion(listing, "Dumped by pg_dump version:")) != "17" {
			t.Fatalf("valid large listing %d failed: %v", i, err)
		}
	}
	// cat succeeds after pg_restore rejects this header. Its exit must not mask it.
	bad := strings.NewReader("PGDMPinvalid" + strings.Repeat("x", 8<<20))
	if _, err := d.Run(ctx, bad, "exec", "--interactive", name, "sh", "-c", archiveListScript); err == nil {
		t.Fatal("draining input masked a corrupt archive")
	}
	t.Run("cancel while draining", func(t *testing.T) {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		child, stop := context.WithCancel(ctx)
		defer stop()
		reached := make(chan struct{})
		input := &holdArchiveEOF{Reader: file, ctx: child, reached: reached}
		done := make(chan error, 1)
		go func() {
			_, err := d.Run(child, input, "exec", "--interactive", name, "sh", "-c", archiveListScript)
			done <- err
		}()
		select {
		case <-reached:
		case <-time.After(15 * time.Second):
			t.Fatal("large input was not forwarded")
		}
		stop()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("cancelled listing passed")
			}
		case <-time.After(6 * time.Second):
			t.Fatal("cancelled listing did not stop within command cleanup budget")
		}
	})
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	after := sha256.New()
	if _, err := io.Copy(after, file); err != nil || !strings.EqualFold(hex.EncodeToString(before.Sum(nil)), hex.EncodeToString(after.Sum(nil))) {
		t.Fatal("listing changed the source archive")
	}
}

type holdArchiveEOF struct {
	io.Reader
	ctx     context.Context
	reached chan struct{}
}

func (r *holdArchiveEOF) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF {
		close(r.reached)
		<-r.ctx.Done()
		return n, r.ctx.Err()
	}
	return n, err
}
