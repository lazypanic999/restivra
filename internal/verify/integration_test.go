package verify

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in because this test starts real PostgreSQL containers and needs an image.
func TestIntegrationPostgres17(t *testing.T) {
	if os.Getenv("RESTIVRA_INTEGRATION") != "1" {
		t.Skip("set RESTIVRA_INTEGRATION=1; docker pull " + Image)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	d := Docker{}
	id := make([]byte, 8)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	name := "restivra-fixture-" + hex.EncodeToString(id)
	defer func() {
		clean, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		if _, err := d.Run(clean, nil, "rm", "--force", "--volumes", name); err != nil {
			t.Error("fixture cleanup failed")
		}
	}()
	if _, err := d.Run(ctx, nil, "run", "--detach", "--name", name, "--network", "none", "--memory", "512m", "--cpus", "1", "--pids-limit", "128", "--env", "POSTGRES_HOST_AUTH_METHOD=trust", Image); err != nil {
		t.Fatal("fixture start failed; ensure Docker and image are available")
	}
	for {
		_, err := d.Run(ctx, nil, "exec", name, "sh", "-c", `test "$(cat /proc/1/comm)" = postgres && pg_isready -q -U postgres`)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("fixture readiness timed out")
		}
		time.Sleep(250 * time.Millisecond)
	}
	if _, err := d.Run(ctx, nil, "exec", name, "psql", "-X", "-U", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "CREATE TABLE customers(id integer PRIMARY KEY, name text NOT NULL); INSERT INTO customers VALUES (1, 'Ada'), (2, 'Deniz');"); err != nil {
		t.Fatal("fixture schema failed")
	}
	// This small fixture is below the command output cap; binary bytes are preserved.
	dump, err := d.Run(ctx, nil, "exec", name, "pg_dump", "-U", "postgres", "-Fc", "postgres")
	if err != nil {
		t.Fatal("fixture dump failed")
	}
	dir := t.TempDir()
	backup := filepath.Join(dir, "fixture.dump")
	if err := os.WriteFile(backup, []byte(dump), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, sql, status, check string }{
		{"valid restore", "SELECT count(*) = 2 FROM public.customers", "passed", "passed"},
		{"business failure", "SELECT count(*) = 3 FROM public.customers", "failed", "failed"},
		{"query error", "SELECT count(*) > 0 FROM public.missing_table", "failed", "error"},
		{"null rejected", "SELECT NULL::boolean", "failed", "error"},
		{"write blocked", "SELECT lo_create(12345) > 0", "failed", "error"},
		{"query timeout", "SELECT pg_sleep(15) IS NULL", "failed", "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.TimeoutSeconds = 60
			cfg.Checks = []Check{{Name: tc.name, SQL: tc.sql}}
			report := (Runner{WorkDir: filepath.Join(dir, "snapshots")}).Run(ctx, backup, cfg)
			if report.Status != tc.status || report.Checks[0].Status != tc.check || report.RestoreStatus != "passed" || report.CleanupStatus != "passed" {
				t.Fatalf("unexpected result: %+v", report)
			}
			ids, err := d.Run(ctx, nil, "ps", "--all", "--quiet", "--filter", "label=restivra.run="+report.RunID)
			if err != nil || strings.TrimSpace(ids) != "" {
				t.Fatal("test container leaked")
			}
		})
	}
	t.Run("corrupt archive", func(t *testing.T) {
		bad := filepath.Join(dir, "bad.dump")
		if err := os.WriteFile(bad, []byte("PGDMPcorrupt"), 0600); err != nil {
			t.Fatal(err)
		}
		report := (Runner{WorkDir: filepath.Join(dir, "snapshots")}).Run(ctx, bad, DefaultConfig())
		if report.Status != "failed" || report.Stage != "archive" || report.CleanupStatus != "passed" {
			t.Fatalf("unexpected corrupt result: %+v", report)
		}
	})
	t.Run("run deadline", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.TimeoutSeconds = 5
		cfg.Checks = []Check{{Name: "slow", SQL: "SELECT pg_sleep(30) IS NULL"}}
		report := (Runner{WorkDir: filepath.Join(dir, "snapshots")}).Run(ctx, backup, cfg)
		if report.Status != "failed" || report.Error != "run exceeded its time limit" || report.CleanupStatus != "passed" {
			t.Fatalf("unexpected timeout result: %+v", report)
		}
	})
	entries, err := os.ReadDir(filepath.Join(dir, "snapshots"))
	if err != nil || len(entries) != 0 {
		t.Fatal("snapshot files leaked")
	}
}
