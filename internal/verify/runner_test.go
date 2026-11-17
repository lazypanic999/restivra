package verify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakeDocker struct {
	calls           [][]string
	queryOutput     string
	queryError      error
	restoreError    error
	createError     bool
	removeError     bool
	containerExists bool
	cancel          context.CancelFunc
}

func (d *fakeDocker) Run(ctx context.Context, in io.Reader, args ...string) (string, error) {
	d.calls = append(d.calls, append([]string(nil), args...))
	if err := ctx.Err(); err != nil {
		return "", err
	}
	joined := strings.Join(args, " ")
	switch {
	case args[0] == "create" && d.createError:
		return "", errors.New("create failed")
	case args[0] == "image":
		return "sha256:fixture", nil
	case args[0] == "rm" && d.removeError:
		return "", errors.New("remove failed")
	case args[0] == "ps" && d.containerExists:
		return "container-id", nil
	case strings.Contains(joined, "pg_restore --list"):
		if in != nil {
			_, _ = io.Copy(io.Discard, in)
		}
		return "; Dumped from database version: 17.7\n; Dumped by pg_dump version: 17.7\n", nil
	case strings.Contains(joined, "pg_restore --username"):
		if in != nil {
			_, _ = io.Copy(io.Discard, in)
		}
		if d.cancel != nil {
			d.cancel()
			return "", context.Canceled
		}
		return "", d.restoreError
	case strings.Contains(joined, "SELECT 1"):
		return "1\n", nil
	case strings.Contains(joined, "BEGIN READ ONLY"):
		return d.queryOutput, d.queryError
	}
	return "", nil
}

func fakeBackup(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.dump")
	if err := os.WriteFile(path, []byte("PGDMPfake archive body"), 0600); err != nil {
		t.Fatal(err)
	}
	return path, filepath.Join(dir, "work")
}

func TestFailuresNeverBecomeSuccessfulAndAlwaysCleanUp(t *testing.T) {
	cases := []struct {
		name    string
		docker  fakeDocker
		want    string
		cleanup string
	}{
		{"false check", fakeDocker{queryOutput: "f\n"}, "failed", "passed"},
		{"null check", fakeDocker{queryOutput: "\n"}, "error", "passed"},
		{"non boolean", fakeDocker{queryOutput: "1\n"}, "error", "passed"},
		{"query error", fakeDocker{queryError: errors.New("password=secret row data")}, "error", "passed"},
		{"restore failure", fakeDocker{restoreError: errors.New("private data")}, "not_run", "passed"},
		{"failed create", fakeDocker{createError: true}, "not_run", "passed"},
		{"cleanup failure", fakeDocker{queryOutput: "t\n", removeError: true, containerExists: true}, "passed", "failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, work := fakeBackup(t)
			cfg := DefaultConfig()
			cfg.Checks = []Check{{Name: "business check", SQL: "SELECT true"}}
			report := (Runner{Docker: &tc.docker, WorkDir: work}).Run(context.Background(), path, cfg)
			if report.Status != "failed" || report.Checks[0].Status != tc.want || report.CleanupStatus != tc.cleanup {
				t.Fatalf("unexpected report: %+v", report)
			}
			if strings.Contains(report.Error, "secret") || strings.Contains(report.Error, "private data") {
				t.Fatal("report leaked command diagnostics")
			}
			removed := false
			for _, call := range tc.docker.calls {
				if call[0] == "rm" {
					removed = true
				}
			}
			if !removed {
				t.Fatal("container cleanup was not attempted")
			}
			entries, err := os.ReadDir(work)
			if err != nil || len(entries) != 0 {
				t.Fatal("private snapshot was not removed")
			}
		})
	}
}

func TestCancellationStillCleansUpWithIndependentContext(t *testing.T) {
	path, work := fakeBackup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	docker := &fakeDocker{cancel: cancel}
	r := (Runner{Docker: docker, WorkDir: work}).Run(ctx, path, DefaultConfig())
	if r.Status != "failed" || r.Error != "run was cancelled" || r.CleanupStatus != "passed" {
		t.Fatalf("unexpected cancellation report: %+v", r)
	}
	removed := false
	for _, call := range docker.calls {
		if call[0] == "rm" {
			removed = true
		}
	}
	if !removed {
		t.Fatal("cancelled context prevented cleanup")
	}
}

func TestSuccessfulRunUsesSnapshotAndRestrictedContainer(t *testing.T) {
	path, work := fakeBackup(t)
	docker := &fakeDocker{queryOutput: "t\n"}
	cfg := DefaultConfig()
	cfg.Checks = []Check{{Name: "rows", SQL: "SELECT true"}}
	r := (Runner{Docker: docker, WorkDir: work}).Run(context.Background(), path, cfg)
	if r.Status != "passed" || r.RestoreStatus != "passed" || r.CleanupStatus != "passed" {
		t.Fatalf("unexpected report: %+v", r)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(content)
	if r.Backup.SHA256 != hex.EncodeToString(h[:]) {
		t.Fatal("report did not fingerprint tested data")
	}
	for _, call := range docker.calls {
		if call[0] != "create" {
			continue
		}
		joined := strings.Join(call, " ")
		for _, required := range []string{"--network none", "--read-only", "--cap-drop ALL", "--security-opt no-new-privileges=true", "--memory 1536m", "--cpus 1", "--pids-limit 128", "--user 999:999"} {
			if !strings.Contains(joined, required) {
				t.Errorf("missing isolation: %s", required)
			}
		}
		if strings.Contains(joined, "--publish") || strings.Contains(joined, "--volume") {
			t.Fatal("container must not expose ports or host files")
		}
	}
}

func TestInvalidInputsDoNotStartDocker(t *testing.T) {
	for _, data := range []string{"", "SELECT 1;", "PGDMPoversized"} {
		t.Run(data, func(t *testing.T) {
			path, work := fakeBackup(t)
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			cfg := DefaultConfig()
			cfg.MaxBackupBytes = 10
			docker := &fakeDocker{}
			r := (Runner{Docker: docker, WorkDir: work}).Run(context.Background(), path, cfg)
			if r.Status != "failed" || len(docker.calls) != 0 {
				t.Fatal("invalid input reached Docker")
			}
		})
	}
}

func TestConfigRejectsUnsafeOrAmbiguousChecks(t *testing.T) {
	for _, q := range []string{"DELETE FROM customers", "SELECT true; DROP TABLE customers", "SELECT true -- comment", "SELECT true /* comment */", "SELECT \\gexec"} {
		cfg := DefaultConfig()
		cfg.Checks = []Check{{Name: "check", SQL: q}}
		if cfg.Validate() == nil {
			t.Errorf("accepted unsupported SQL: %s", q)
		}
	}
	for _, raw := range []string{`{"typo":1}`, `{} {}`, `{"memory_mb":128}`, `{"timeout_seconds":0}`} {
		if _, err := ReadConfig(strings.NewReader(raw)); err == nil {
			t.Errorf("accepted invalid config: %s", raw)
		}
	}
}

type labelDocker struct {
	calls [][]string
	label string
}

func (d *labelDocker) Run(ctx context.Context, in io.Reader, args ...string) (string, error) {
	d.calls = append(d.calls, args)
	return d.label, nil
}

func TestCleanupRefusesUnownedContainer(t *testing.T) {
	id := strings.Repeat("a", 32)
	d := &labelDocker{label: "different"}
	if err := (Runner{Docker: d}).Cleanup(context.Background(), id); err == nil {
		t.Fatal("cleanup accepted mismatched label")
	}
	if len(d.calls) != 1 || d.calls[0][0] != "inspect" {
		t.Fatal("cleanup mutated unowned container")
	}
	before := append([][]string(nil), d.calls...)
	if err := (Runner{Docker: d}).Cleanup(context.Background(), "--all"); err == nil {
		t.Fatal("accepted invalid ID")
	}
	if !reflect.DeepEqual(d.calls, before) {
		t.Fatal("invalid ID reached Docker")
	}
}
