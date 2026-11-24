package verify

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lazypanic999/restivra/internal/buildinfo"
)

type Runner struct {
	Docker         Commander
	WorkDir        string
	RunID          string
	Instance       string
	Installation   string
	OpenBackup     func(string) (*os.File, error)
	SnapshotBackup func(context.Context, string, string, int64) (*os.File, Backup, error)
}

// Listing reads only the header/TOC. Drain the remainder before the exec exits:
// otherwise Docker can report a stdin forwarding failure for a valid archive.
// The fixed shell program keeps pg_restore's status; a successful drain must
// never turn a corrupt archive into a successful listing.
const archiveListScript = `pg_restore --list; result=$?; cat >/dev/null || exit 1; exit "$result"`

func (r Runner) Run(parent context.Context, source string, cfg Config) (report Report) {
	start := time.Now().UTC()
	id := make([]byte, 16)
	_, randomErr := rand.Read(id)
	if r.RunID != "" {
		decoded, err := hex.DecodeString(r.RunID)
		if err != nil || len(decoded) != 16 || strings.ToLower(r.RunID) != r.RunID {
			randomErr = errors.New("invalid run ID")
		} else {
			id = decoded
		}
	}
	build := buildinfo.Current()
	report = Report{ApplicationBuild: &build, SchemaVersion: 1, RunID: hex.EncodeToString(id), Status: "failed", Stage: "validate", StartedAt: start, Image: Image, RestoreStatus: "not_run", CleanupStatus: "not_needed", Checks: []CheckResult{}, Warnings: []string{
		"This verifies a logical restore and selected checks, not full application recovery, ownership, grants or point-in-time recovery.",
		"File modification time is not a verified backup creation time. Compare SHA-256 values across runs to detect unchanged backups.",
	}}
	for _, c := range cfg.Checks {
		report.Checks = append(report.Checks, CheckResult{Name: c.Name, Status: "not_run"})
	}
	defer func() {
		report.FinishedAt = time.Now().UTC()
		report.DurationMS = time.Since(start).Milliseconds()
	}()
	fail := func(message string) { report.Error = message; report.ErrorCode = stageErrorCode(report.Stage) }
	if randomErr != nil {
		fail("could not generate run ID")
		return
	}
	if err := cfg.Validate(); err != nil {
		fail(err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()
	defer func() {
		if ctx.Err() != nil && report.CleanupStatus != "failed" {
			report.Status = "failed"
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				report.Error = "run exceeded its time limit"
				report.ErrorCode = "run_timeout"
			} else {
				report.Error = "run was cancelled"
				report.ErrorCode = "cancelled"
			}
		}
	}()
	if r.Docker == nil {
		r.Docker = Docker{}
	}
	r.Docker = tracedDocker{r.Docker, &report.CommandTimings}
	if r.WorkDir == "" {
		r.WorkDir = "work"
	}
	if err := os.MkdirAll(r.WorkDir, 0700); err != nil {
		fail("could not create work directory")
		return
	}
	var snapshot *os.File
	var backup Backup
	var err error
	if r.SnapshotBackup != nil {
		report.Stage = "source"
		snapshot, backup, err = r.SnapshotBackup(ctx, source, r.WorkDir, cfg.MaxBackupBytes)
	} else {
		snapshot, backup, err = snapshotBackupWithOpener(ctx, source, r.WorkDir, cfg.MaxBackupBytes, r.OpenBackup)
	}
	report.Backup = backup
	if backup.Source != nil && backup.Source.Kind == "s3" {
		report.Warnings[1] = "S3 Last-Modified is object metadata, not a verified backup creation time. ETag is not assumed to be a content hash; SHA-256 is calculated from the downloaded bytes."
	}
	if err != nil {
		fail(err.Error())
		var coded interface{ ErrorCode() string }
		if errors.As(err, &coded) {
			report.ErrorCode = coded.ErrorCode()
		}
		return
	}
	defer func() {
		_ = snapshot.Close()
		if err := os.Remove(snapshot.Name()); err != nil {
			report.Status = "failed"
			report.CleanupStatus = "failed"
			report.ErrorCode = "cleanup_failed"
			report.Error = "could not remove private backup snapshot; remove it from work directory"
		}
	}()
	report.Stage = "engine"
	if _, err = r.Docker.Run(ctx, nil, "info", "--format", "{{.OSType}}"); err != nil {
		fail("Docker engine unavailable; start Docker with Linux containers")
		return
	}
	report.Stage = "image"
	imageID, err := r.Docker.Run(ctx, nil, "image", "inspect", "--format", "{{.Id}}", Image)
	if err != nil {
		fail("PostgreSQL image missing; run: docker pull " + Image)
		return
	}
	report.ImageID = strings.TrimSpace(imageID)
	name := "restivra-" + report.RunID
	// Cleanup is registered BEFORE create: cancellation can occur after Docker has
	// created a container but before the CLI receives its successful response.
	defer func() {
		if err := cleanupCreatedContainer(r.Docker, name, 20*time.Second); err != nil {
			report.Status = "failed"
			report.CleanupStatus = "failed"
			report.ErrorCode = "cleanup_failed"
			report.Error = "container cleanup failed; use cleanup --run-id " + report.RunID
			return
		}
		report.CleanupStatus = "passed"
	}()
	report.Stage = "start"
	_, err = r.Docker.Run(ctx, nil, "create", "--name", name,
		"--label", "app=restivra", "--label", "restivra.run="+report.RunID,
		"--label", "restivra.instance="+r.Instance,
		"--label", "restivra.installation="+r.Installation,
		"--network", "none", "--read-only", "--user", "999:999",
		"--cap-drop", "ALL", "--security-opt", "no-new-privileges=true",
		"--memory", strconv.Itoa(cfg.MemoryMB)+"m", "--memory-swap", strconv.Itoa(cfg.MemoryMB)+"m",
		"--cpus", "1", "--pids-limit", "128", "--log-driver", "none",
		"--tmpfs", "/var/lib/postgresql/data:rw,nosuid,nodev,size="+strconv.Itoa(cfg.DataMB)+"m,uid=999,gid=999,mode=0700",
		"--tmpfs", "/var/run/postgresql:rw,nosuid,nodev,size=16m,uid=999,gid=999",
		"--tmpfs", "/tmp:rw,nosuid,nodev,size=32m,mode=1777",
		"--env", "POSTGRES_HOST_AUTH_METHOD=trust", "--env", "POSTGRES_DB=verification",
		report.ImageID, "postgres", "-c", "listen_addresses=", "-c", "unix_socket_directories=/var/run/postgresql")
	if err != nil {
		fail("could not create isolated PostgreSQL container")
		return
	}
	if _, err = r.Docker.Run(ctx, nil, "start", name); err != nil {
		fail("could not start PostgreSQL container")
		return
	}
	for {
		// Initdb's temporary server can answer pg_isready; wait until the final
		// postgres process is PID 1 before accepting readiness.
		_, err = r.Docker.Run(ctx, nil, "exec", name, "sh", "-c", `test "$(cat /proc/1/comm)" = postgres && pg_isready -q -U postgres -d verification`)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			fail("PostgreSQL did not become ready")
			return
		case <-time.After(250 * time.Millisecond):
		}
	}
	report.Stage = "archive"
	listing, err := r.Docker.Run(ctx, snapshot, "exec", "--interactive", name, "sh", "-c", archiveListScript)
	if err != nil {
		fail("archive is unreadable by PostgreSQL 17 pg_restore")
		return
	}
	report.Backup.SourceVersion = archiveVersion(listing, "Dumped from database version:")
	report.Backup.DumpVersion = archiveVersion(listing, "Dumped by pg_dump version:")
	if major(report.Backup.SourceVersion) != "17" || major(report.Backup.DumpVersion) != "17" {
		fail("only PostgreSQL 17 backups created with pg_dump 17 are supported")
		report.ErrorCode = "unsupported_version"
		return
	}
	if _, err = snapshot.Seek(0, io.SeekStart); err != nil {
		fail("could not rewind backup snapshot")
		return
	}
	report.Stage = "restore"
	report.RestoreStatus = "failed"
	if _, err = r.Docker.Run(ctx, snapshot, "exec", "--interactive", name, "pg_restore", "--username=postgres", "--dbname=verification", "--no-owner", "--no-privileges", "--exit-on-error", "--no-password"); err != nil {
		fail("pg_restore failed; archive may be corrupt, require unavailable extensions, or exceed resource limits")
		var commandError *CommandError
		if errors.As(err, &commandError) && commandError.Category != "command_failed" {
			report.ErrorCode = commandError.Category
		}
		return
	}
	report.RestoreStatus = "passed"
	report.Stage = "connect"
	out, err := r.Docker.Run(ctx, nil, "exec", name, "psql", "-X", "-qAt", "-U", "postgres", "-d", "verification", "-v", "ON_ERROR_STOP=1", "-c", "SELECT 1")
	if err != nil || strings.TrimSpace(out) != "1" {
		fail("restored database connection check failed")
		return
	}
	if len(cfg.Checks) == 0 {
		report.Warnings = append(report.Warnings, "No custom checks configured; application data was not validated.")
	}
	if len(cfg.Checks) > 0 {
		report.Stage = "checks"
		_, err = r.Docker.Run(ctx, nil, "exec", name, "psql", "-X", "-qAt", "-U", "postgres", "-d", "verification", "-v", "ON_ERROR_STOP=1", "-c",
			"CREATE ROLE restivra_verifier LOGIN; GRANT CONNECT ON DATABASE verification TO restivra_verifier; GRANT pg_read_all_data TO restivra_verifier; ALTER ROLE restivra_verifier SET default_transaction_read_only = on;")
		if err != nil {
			fail("could not create restricted verification role")
			return
		}
	}
	checksPassed := true
	for i, check := range cfg.Checks {
		query := "BEGIN READ ONLY; SET LOCAL statement_timeout = '10s'; SET LOCAL lock_timeout = '2s'; SELECT (" + strings.TrimSpace(check.SQL) + ")::boolean; ROLLBACK;"
		out, err := r.Docker.Run(ctx, nil, "exec", name, "psql", "-X", "-qAt", "-U", "restivra_verifier", "-d", "verification", "-v", "ON_ERROR_STOP=1", "-c", query)
		switch {
		case err != nil:
			report.Checks[i].Status = "error"
			report.Checks[i].ErrorCode = "query_error"
			var commandError *CommandError
			if errors.As(err, &commandError) && commandError.Category != "command_failed" {
				report.Checks[i].ErrorCode = commandError.Category
			}
			report.Checks[i].Error = "query failed or exceeded its time limit"
			checksPassed = false
		case strings.TrimSpace(out) == "t":
			report.Checks[i].Status = "passed"
		case strings.TrimSpace(out) == "f":
			report.Checks[i].Status = "failed"
			report.Checks[i].ErrorCode = "check_false"
			checksPassed = false
		default:
			report.Checks[i].Status = "error"
			report.Checks[i].ErrorCode = "invalid_check_result"
			report.Checks[i].Error = "query must return exactly one non-null boolean"
			checksPassed = false
		}
		if ctx.Err() != nil {
			return
		}
	}
	if !checksPassed {
		fail("one or more verification checks did not pass")
		return
	}
	report.Status = "passed"
	report.Stage = "complete"
	return
}

func cleanupCreatedContainer(docker Commander, name string, removalBudget time.Duration) error {
	removeCtx, stopRemove := context.WithTimeout(context.Background(), removalBudget)
	_, err := docker.Run(removeCtx, nil, "rm", "--force", "--volumes", name)
	stopRemove()
	if err == nil {
		return nil
	}
	// The daemon may have completed removal after the CLI timed out. Confirm
	// absence with a fresh, independently bounded context, never the expired one.
	confirmCtx, stopConfirm := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopConfirm()
	ids, listErr := docker.Run(confirmCtx, nil, "ps", "--all", "--quiet", "--filter", "name=^/"+name+"$")
	if listErr != nil || strings.TrimSpace(ids) != "" {
		return errors.New("container removal could not be confirmed")
	}
	return nil
}

func stageErrorCode(stage string) string {
	switch stage {
	case "engine":
		return "engine_unavailable"
	case "validate":
		return "backup_invalid"
	case "image":
		return "image_unavailable"
	case "start":
		return "database_start_failed"
	case "archive":
		return "archive_invalid"
	case "restore":
		return "restore_failed"
	case "connect":
		return "database_connection_failed"
	case "checks":
		return "checks_failed"
	default:
		return "verification_failed"
	}
}

func archiveVersion(listing, label string) string {
	for _, line := range strings.Split(listing, "\n") {
		if _, after, ok := strings.Cut(line, label); ok {
			fields := strings.Fields(after)
			if len(fields) > 0 {
				return fields[0]
			}
		}
	}
	return ""
}

func major(v string) string { first, _, _ := strings.Cut(v, "."); return first }

func snapshotBackup(ctx context.Context, source, work string, max int64) (*os.File, Backup, error) {
	return snapshotBackupWithOpener(ctx, source, work, max, nil)
}

func snapshotBackupWithOpener(ctx context.Context, source, work string, max int64, opener func(string) (*os.File, error)) (*os.File, Backup, error) {
	b := Backup{Name: filepath.Base(source)}
	if opener == nil {
		opener = os.Open
	}
	in, err := opener(source)
	if err != nil {
		return nil, b, errors.New("could not open backup file")
	}
	defer in.Close()
	stat, err := in.Stat()
	if err != nil || !stat.Mode().IsRegular() {
		return nil, b, errors.New("backup must be a regular file")
	}
	b.Bytes = stat.Size()
	b.ModifiedAt = stat.ModTime().UTC()
	if b.Bytes > max {
		return nil, b, errors.New("backup exceeds max_backup_bytes")
	}
	snapshot, b, err := SnapshotStream(ctx, in, b, work, max)
	if err != nil {
		return nil, b, err
	}
	after, err := in.Stat()
	if err != nil || !after.ModTime().Equal(stat.ModTime()) {
		snapshot.Close()
		os.Remove(snapshot.Name())
		return nil, b, errors.New("backup changed while being copied; use an atomically completed backup")
	}
	return snapshot, b, nil
}

// SnapshotStream creates one bounded, hashed, seekable private copy. The caller
// owns and must close the input stream; the returned file belongs to the runner.
func SnapshotStream(ctx context.Context, in io.Reader, b Backup, work string, max int64) (*os.File, Backup, error) {
	if b.Bytes < 0 || b.Bytes > max {
		return nil, b, errors.New("backup exceeds max_backup_bytes")
	}
	snapshot, err := os.CreateTemp(work, "snapshot-*.dump")
	if err != nil {
		return nil, b, errors.New("could not create private snapshot")
	}
	ok := false
	defer func() {
		if !ok {
			snapshot.Close()
			os.Remove(snapshot.Name())
		}
	}()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(snapshot, h), io.LimitReader(contextReader{ctx, in}, max+1))
	if err != nil {
		return nil, b, errors.New("could not snapshot backup")
	}
	if n > max || n != b.Bytes {
		return nil, b, errors.New("backup grew or shrank while being copied")
	}
	if ctx.Err() != nil {
		return nil, b, errors.New("could not snapshot backup")
	}
	b.SHA256 = hex.EncodeToString(h.Sum(nil))
	if _, err = snapshot.Seek(0, io.SeekStart); err != nil {
		return nil, b, err
	}
	header := make([]byte, 5)
	if _, err = io.ReadFull(snapshot, header); err != nil || !bytes.Equal(header, []byte("PGDMP")) {
		return nil, b, errors.New("expected a pg_dump custom archive (PGDMP header)")
	}
	if _, err = snapshot.Seek(0, io.SeekStart); err != nil {
		return nil, b, err
	}
	ok = true
	return snapshot, b, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func (r Runner) Cleanup(ctx context.Context, id string) error {
	if !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(id) {
		return errors.New("run ID must be 32 lowercase hexadecimal characters")
	}
	if r.Docker == nil {
		r.Docker = Docker{}
	}
	name := "restivra-" + id
	label, err := r.Docker.Run(ctx, nil, "inspect", "--format", `{{index .Config.Labels "restivra.run"}}`, name)
	if err != nil {
		return errors.New("container not found or Docker unavailable")
	}
	if strings.TrimSpace(label) != id {
		return errors.New("container ownership label does not match; refusing cleanup")
	}
	if _, err = r.Docker.Run(ctx, nil, "rm", "--force", "--volumes", name); err != nil {
		return fmt.Errorf("cleanup failed for run %s", id)
	}
	return nil
}
