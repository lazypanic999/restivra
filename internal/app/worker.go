package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lazypanic999/restivra/internal/verify"
)

type Worker struct {
	Store         *Store
	BackupRoot    string
	WorkDir       string
	Instance      string
	Installation  string
	WebhookURL    string
	RetentionDays int
	Docker        verify.Commander
}

func (w *Worker) Run(ctx context.Context) error {
	if w.Docker == nil {
		w.Docker = verify.Docker{}
	}
	if w.RetentionDays == 0 {
		w.RetentionDays = 30
	}
	if w.Instance == "" || strings.ContainsAny(w.Instance, "\r\n\x00") {
		return errors.New("RESTIVRA_INSTANCE is required")
	}
	root, err := os.OpenRoot(w.BackupRoot)
	if err != nil {
		return fmt.Errorf("open backup root: %w", err)
	}
	defer root.Close()
	if err = os.MkdirAll(w.WorkDir, 0700); err != nil {
		return err
	}
	conn, err := w.Store.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var locked bool
	if err = conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(78312402)").Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return errors.New("another worker owns this database; only one restore worker may run")
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(unlockCtx, "SELECT pg_advisory_unlock(78312402)"); err != nil {
			// Never put a possibly still-locked session back in the shared pool.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	w.Installation, err = w.Store.InstallationID(ctx)
	if err != nil {
		return err
	}
	if err = w.recover(ctx); err != nil {
		return err
	}
	log.Print("worker ready; one restore at a time")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastPrune := time.Time{}
	for {
		if ctx.Err() != nil {
			return nil
		}
		tickCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err = conn.PingContext(tickCtx)
		if err == nil {
			_, err = w.Store.DB.ExecContext(tickCtx, `INSERT INTO workers(instance,heartbeat_at) VALUES($1,clock_timestamp()) ON CONFLICT(instance) DO UPDATE SET heartbeat_at=EXCLUDED.heartbeat_at`, w.Instance)
		}
		if err == nil {
			err = w.Store.Schedule(tickCtx, time.Now().UTC())
		}
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return errors.New("worker lost database connectivity; stopping")
		}
		claimCtx, stopClaim := context.WithTimeout(ctx, 10*time.Second)
		job, err := w.Store.Claim(claimCtx)
		stopClaim()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if job != nil {
			if job.AttemptCount > 1 {
				// Reconcile an ambiguous late Docker create before reusing this
				// job's container name. Never overlap attempts.
				cleanCtx, stopClean := context.WithTimeout(ctx, 30*time.Second)
				found, e := w.Docker.Run(cleanCtx, nil, "ps", "--all", "--quiet", "--filter", "name=^/restivra-"+job.ID+"$", "--filter", "label=app=restivra")
				if e == nil && strings.TrimSpace(found) != "" {
					e = (verify.Runner{Docker: w.Docker}).Cleanup(cleanCtx, job.ID)
				}
				if e == nil {
					e = cleanSnapshotDir(w.WorkDir, job.ID)
				}
				stopClean()
				if e != nil {
					return ErrCleanupPending
				}
			}
			if err = w.execute(ctx, conn, root, *job); err != nil {
				return err
			}
		}
		notifyCtx, stopNotify := context.WithTimeout(ctx, 15*time.Second)
		err = w.notifyPending(notifyCtx)
		stopNotify()
		if err != nil {
			log.Print("notification state update failed")
		}
		if time.Since(lastPrune) > time.Hour {
			pruneCtx, stopPrune := context.WithTimeout(ctx, 10*time.Second)
			_, err = w.Store.DB.ExecContext(pruneCtx, `DELETE FROM jobs WHERE finished_at < clock_timestamp() - make_interval(days => $1) AND status NOT IN ('queued','running') AND notification_status <> 'pending'`, w.RetentionDays)
			stopPrune()
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
			lastPrune = time.Now()
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (w *Worker) recover(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	ids, err := w.Docker.Run(ctx, nil, "ps", "--all", "--filter", "label=app=restivra", "--filter", "label=restivra.installation="+w.Installation, "--format", `{{.Label "restivra.run"}}`)
	if err != nil {
		return errors.New("Docker unavailable; cannot reconcile old restore containers")
	}
	owned := map[string]bool{}
	for _, id := range strings.Fields(ids) {
		owned[id] = true
	}
	// Legacy containers have no installation label. Only exact running job IDs
	// in this database are authoritative; instance names are not ownership proof.
	rows, err := w.Store.DB.QueryContext(ctx, "SELECT id FROM jobs WHERE status='running'")
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		owned[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for id := range owned {
		if !ValidID(id) {
			return errors.New("invalid ownership label on restore container")
		}
		found, err := w.Docker.Run(ctx, nil, "ps", "--all", "--quiet", "--filter", "name=^/restivra-"+id+"$", "--filter", "label=app=restivra")
		if err != nil {
			return err
		}
		if strings.TrimSpace(found) == "" {
			continue
		}
		if err := (verify.Runner{Docker: w.Docker}).Cleanup(ctx, id); err != nil {
			return err
		}
	}
	// The DB lock is held and no job has started; only known private snapshot
	// filenames beneath 32-hex run directories are removed, without recursion.
	entries, err := os.ReadDir(w.WorkDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !ValidID(entry.Name()) {
			continue
		}
		if err := cleanSnapshotDir(w.WorkDir, entry.Name()); err != nil {
			return err
		}
	}
	return w.Store.RecoverAttempts(ctx, w.WebhookURL != "")
}

func cleanSnapshotDir(work, id string) error {
	if !ValidID(id) {
		return errors.New("invalid private work directory ID")
	}
	root, err := os.OpenRoot(work)
	if err != nil {
		return err
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), id)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "snapshot-") || !strings.HasSuffix(entry.Name(), ".dump") {
			return errors.New("unexpected file in private run directory; refusing cleanup")
		}
		if err = root.Remove(filepath.Join(id, entry.Name())); err != nil {
			return err
		}
	}
	return root.Remove(id)
}

func latestBackup(root *os.Root, pattern string) (string, error) {
	matches, err := fs.Glob(root.FS(), pattern)
	if err != nil {
		return "", err
	}
	var selected string
	var modified time.Time
	for _, path := range matches {
		stat, err := root.Stat(path)
		if err != nil || !stat.Mode().IsRegular() {
			continue
		}
		if selected == "" || stat.ModTime().After(modified) || (stat.ModTime().Equal(modified) && path > selected) {
			selected = path
			modified = stat.ModTime()
		}
	}
	if selected == "" {
		return "", errors.New("No completed backup matched the configured path inside the backup root.")
	}
	return selected, nil
}

func (w *Worker) execute(parent context.Context, conn *sql.Conn, root *os.Root, job Job) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	heartbeatDone := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				heartbeatDone <- nil
				return
			case <-ticker.C:
				beatCtx, stop := context.WithTimeout(ctx, 15*time.Second)
				err := conn.PingContext(beatCtx)
				var requested bool
				if err == nil {
					// One transaction/fsync for both liveness records. Heartbeat
					// writes tolerate short host I/O stalls, while loss of the
					// advisory-lock connection still stops the active restore.
					err = w.Store.DB.QueryRowContext(beatCtx, `WITH job_beat AS (
					 UPDATE jobs SET heartbeat_at=clock_timestamp() WHERE id=$1 AND status='running' RETURNING cancel_requested
					), worker_beat AS (UPDATE workers SET heartbeat_at=clock_timestamp() WHERE instance=$2)
					SELECT cancel_requested FROM job_beat`, job.ID, w.Instance).Scan(&requested)
				}
				stop()
				if err != nil {
					if ctx.Err() != nil {
						heartbeatDone <- nil
						return
					}
					cancel()
					message := "worker heartbeat lost; active restore was stopped"
					if errors.Is(err, context.DeadlineExceeded) {
						message = "worker heartbeat timed out; active restore was stopped"
					}
					heartbeatDone <- errors.New(message)
					return
				}
				if requested {
					cancel()
					heartbeatDone <- nil
					return
				}
			}
		}
	}()
	work := filepath.Join(w.WorkDir, job.ID)
	path, pathErr := latestBackup(root, job.BackupPattern)
	var report *verify.Report
	status := "failed"
	errorText := ""
	cleanupFailed := false
	if pathErr != nil {
		errorText = pathErr.Error()
	} else {
		result := (verify.Runner{Docker: w.Docker, WorkDir: work, RunID: job.ID, Instance: w.Instance, Installation: w.Installation, OpenBackup: root.Open}).Run(ctx, path, job.Config)
		report = &result
		cleanupFailed = result.CleanupStatus == "failed"
		status = result.Status
		errorText = result.Error
		if result.Backup.SHA256 != "" {
			var previous sql.NullString
			lookupCtx, stopLookup := context.WithTimeout(parent, 15*time.Second)
			err := w.Store.DB.QueryRowContext(lookupCtx, `SELECT report->'backup'->>'sha256' FROM jobs WHERE project_id=$1 AND id<>$2 AND report IS NOT NULL ORDER BY queued_at DESC LIMIT 1`, job.ProjectID, job.ID).Scan(&previous)
			stopLookup()
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				cancel()
				<-heartbeatDone
				return err
			}
			if previous.Valid && previous.String == result.Backup.SHA256 {
				report.Warnings = append(report.Warnings, "The tested backup is identical to the previous run; backup freshness was not established.")
			}
			if time.Since(result.Backup.ModifiedAt) > 24*time.Hour {
				report.Warnings = append(report.Warnings, "The backup file modification time is over 24 hours old; this is not a verified backup creation timestamp.")
			}
		}
		if err := cleanSnapshotDir(w.WorkDir, job.ID); err != nil {
			cleanupFailed = true
			status = "failed"
			errorText = "Private work directory cleanup failed."
			report.CleanupStatus = "failed"
			report.Status = "failed"
		}
	}
	cancel()
	heartbeatErr := <-heartbeatDone
	// Cancellation of the worker itself is interrupted work, never a success.
	finalCtx, stop := context.WithTimeout(context.Background(), 15*time.Second)
	defer stop()
	var requested bool
	if err := w.Store.DB.QueryRowContext(finalCtx, "SELECT cancel_requested FROM jobs WHERE id=$1", job.ID).Scan(&requested); err != nil {
		return err
	}
	if requested {
		status = "cancelled"
		errorText = "Cancelled by user."
	} else if parent.Err() != nil {
		status = "interrupted"
		errorText = "Worker stopped before completing this run."
	}
	if heartbeatErr != nil {
		status = "interrupted"
		errorText = heartbeatErr.Error()
	}
	if cleanupFailed {
		status = "failed"
		errorText = "Temporary resource cleanup failed. Worker stopped before accepting more jobs."
		if report != nil {
			report.Status = "failed"
			report.Error = errorText
			report.ErrorCode = "cleanup_failed"
		}
	}
	if err := w.Store.FinishAttempt(finalCtx, job.ID, status, errorText, report, w.WebhookURL != ""); err != nil {
		return err
	}
	if cleanupFailed {
		return ErrCleanupPending
	}
	if heartbeatErr != nil {
		return heartbeatErr
	}
	return nil
}

var ErrCleanupPending = errors.New("temporary restore resources remain; refusing to start another job")
