package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/lazypanic999/restivra/internal/verify"
)

const MaxAttempts = 3

func transientFailure(status string, report *verify.Report) bool {
	if status == "interrupted" {
		return true
	}
	if report == nil {
		return false
	}
	switch report.ErrorCode {
	case "engine_unavailable", "database_start_failed", "cleanup_failed", "source_unavailable":
		return true
	case "run_timeout":
		return report.Stage == "start" || report.Stage == "engine" || report.Stage == "image" || report.Stage == "source"
	default:
		return false
	}
}

func (s *Store) Attempts(ctx context.Context, id string) ([]Attempt, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT number,status,started_at,finished_at,report,error FROM job_attempts WHERE job_id=$1 ORDER BY number", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Attempt{}
	for rows.Next() {
		var a Attempt
		var raw []byte
		if err = rows.Scan(&a.Number, &a.Status, &a.StartedAt, &a.FinishedAt, &raw, &a.Error); err != nil {
			return nil, err
		}
		if len(raw) > 0 {
			if err = json.Unmarshal(raw, &a.Report); err != nil {
				return nil, err
			}
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// Attempt completion and retry scheduling commit together. Previous failures
// remain immutable evidence; only infrastructure failures are retried.
func (s *Store) FinishAttempt(ctx context.Context, id string, expectedAttempt int, status, message string, report *verify.Report, webhook bool) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, err := scanJob(tx.QueryRowContext(ctx, "SELECT "+jobCols+" FROM jobs WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return err
	}
	// A worker that lost its advisory-lock connection can still have a live
	// pooled SQL connection. Its delayed completion must not finish a newer
	// attempt of the same job after startup recovery and persisted backoff.
	if current.Status != "running" || current.AttemptCount != expectedAttempt {
		return ErrConflict
	}
	if current.AttemptCount < 1 || current.AttemptCount > MaxAttempts {
		return errors.New("invalid verification attempt count")
	}
	cleanupFailed := report != nil && report.CleanupStatus == "failed"
	if current.CancelRequested && !cleanupFailed {
		status = "cancelled"
		message = "Cancelled by user."
	}
	if report != nil {
		report.Attempt = current.AttemptCount
		report.Status = status
		report.Error = message
		if status == "cancelled" {
			report.ErrorCode = "cancelled"
		}
		if status == "interrupted" {
			report.ErrorCode = "infrastructure_interrupted"
		}
	}
	var raw any
	if report != nil {
		data, e := json.Marshal(report)
		if e != nil {
			return e
		}
		raw = data
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO job_attempts(job_id,number,status,started_at,report,error) VALUES($1,$2,$3,$4,$5,$6)`, id, current.AttemptCount, status, current.StartedAt, raw, message); err != nil {
		return err
	}
	retry := !current.CancelRequested && current.AttemptCount < MaxAttempts && transientFailure(status, report)
	if retry {
		delay := 30 * time.Second * time.Duration(1<<(current.AttemptCount-1))
		_, err = tx.ExecContext(ctx, `UPDATE jobs SET status='queued',started_at=NULL,finished_at=NULL,heartbeat_at=NULL,report=NULL,error='Temporary infrastructure failure; retry scheduled.',retry_at=clock_timestamp()+($2::integer*interval '1 second'),notification_status='skipped',notification_attempts=0 WHERE id=$1`, id, int(delay.Seconds()))
	} else {
		notification := "skipped"
		if webhook && (status == "failed" || status == "interrupted") {
			notification = "pending"
		}
		_, err = tx.ExecContext(ctx, `UPDATE jobs SET status=$2,finished_at=clock_timestamp(),report=$3,error=$4,retry_at=NULL,notification_status=$5,notification_next_at=clock_timestamp() WHERE id=$1`, id, status, raw, message, notification)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Called only after startup has positively reconciled owned containers and
// private snapshots while holding the database's worker advisory lock.
func (s *Store) RecoverAttempts(ctx context.Context, webhook bool) error {
	rows, err := s.DB.QueryContext(ctx, "SELECT id,attempt_count FROM jobs WHERE status='running'")
	if err != nil {
		return err
	}
	type claim struct {
		id     string
		number int
	}
	claims := []claim{}
	for rows.Next() {
		var item claim
		if err = rows.Scan(&item.id, &item.number); err != nil {
			rows.Close()
			return err
		}
		claims = append(claims, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, item := range claims {
		if err = s.FinishAttempt(ctx, item.id, item.number, "interrupted", "Previous worker stopped before finishing. Temporary resources were reconciled.", nil, webhook); err != nil && !errors.Is(err, ErrConflict) {
			return err
		}
	}
	return nil
}
