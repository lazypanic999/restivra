package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct{ DB *sql.DB }

func Open(ctx context.Context, url string) (*Store, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{DB: db}, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS projects (
 id text PRIMARY KEY, name text NOT NULL, backup_pattern text NOT NULL,
 config jsonb NOT NULL, schedule_time text NOT NULL DEFAULT '', timezone text NOT NULL,
 next_run_at timestamptz, created_at timestamptz NOT NULL DEFAULT clock_timestamp(), archived boolean NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS jobs (
 id text PRIMARY KEY, project_id text NOT NULL REFERENCES projects(id), project_name text NOT NULL,
 backup_pattern text NOT NULL, config jsonb NOT NULL,
 status text NOT NULL CHECK(status IN ('queued','running','passed','failed','cancelled','interrupted')),
 trigger text NOT NULL, queued_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 started_at timestamptz, finished_at timestamptz, heartbeat_at timestamptz,
 cancel_requested boolean NOT NULL DEFAULT false, report jsonb, error text NOT NULL DEFAULT '',
 notification_status text NOT NULL DEFAULT 'skipped', notification_attempts integer NOT NULL DEFAULT 0,
 notification_next_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS one_active_job ON jobs(project_id) WHERE status IN ('queued','running');
CREATE INDEX IF NOT EXISTS job_history ON jobs(queued_at DESC);
CREATE INDEX IF NOT EXISTS job_history_cursor ON jobs(queued_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS job_status_history ON jobs(status,queued_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS project_job_history ON jobs(project_id,queued_at DESC,id DESC);
CREATE TABLE IF NOT EXISTS workers(instance text PRIMARY KEY, heartbeat_at timestamptz NOT NULL);
ALTER TABLE workers ADD COLUMN IF NOT EXISTS source_profiles jsonb NOT NULL DEFAULT '[]';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS source jsonb NOT NULL DEFAULT '{"kind":"local"}';
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS source jsonb NOT NULL DEFAULT '{"kind":"local"}';
CREATE TABLE IF NOT EXISTS installation(singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton), id text NOT NULL UNIQUE);
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS attempt_count integer NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS retry_at timestamptz;
UPDATE jobs SET attempt_count=1 WHERE attempt_count=0 AND started_at IS NOT NULL;
CREATE TABLE IF NOT EXISTS job_attempts (
 job_id text NOT NULL REFERENCES jobs(id) ON DELETE CASCADE, number integer NOT NULL,
 status text NOT NULL, started_at timestamptz, finished_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 report jsonb, error text NOT NULL DEFAULT '', PRIMARY KEY(job_id,number)
);
`

func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(78312401)"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, schema); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO installation(singleton,id) VALUES(true,$1) ON CONFLICT(singleton) DO NOTHING", NewID()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) InstallationID(ctx context.Context) (string, error) {
	var id string
	err := s.DB.QueryRowContext(ctx, "SELECT id FROM installation WHERE singleton=true").Scan(&id)
	if err == nil && !ValidID(id) {
		return "", errors.New("invalid installation identity")
	}
	return id, err
}

type scanner interface{ Scan(...any) error }

const projectCols = `id,name,backup_pattern,config,schedule_time,timezone,next_run_at,created_at,archived,source`

func scanProject(row scanner) (Project, error) {
	var p Project
	var raw, sourceRaw []byte
	err := row.Scan(&p.ID, &p.Name, &p.BackupPattern, &raw, &p.ScheduleTime, &p.Timezone, &p.NextRunAt, &p.CreatedAt, &p.Archived, &sourceRaw)
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(sourceRaw, &p.Source); err != nil {
		return p, err
	}
	err = json.Unmarshal(raw, &p.Config)
	return p, err
}

const jobCols = `id,project_id,project_name,backup_pattern,config,status,trigger,queued_at,started_at,finished_at,heartbeat_at,cancel_requested,report,error,notification_status,notification_attempts,attempt_count,retry_at,source`

func scanJob(row scanner) (Job, error) {
	var j Job
	var cfg, report, sourceRaw []byte
	err := row.Scan(&j.ID, &j.ProjectID, &j.ProjectName, &j.BackupPattern, &cfg, &j.Status, &j.Trigger, &j.QueuedAt, &j.StartedAt, &j.FinishedAt, &j.HeartbeatAt, &j.CancelRequested, &report, &j.Error, &j.NotificationStatus, &j.NotificationAttempts, &j.AttemptCount, &j.RetryAt, &sourceRaw)
	if err != nil {
		return j, err
	}
	if err = json.Unmarshal(cfg, &j.Config); err != nil {
		return j, err
	}
	if err = json.Unmarshal(sourceRaw, &j.Source); err != nil {
		return j, err
	}
	if len(report) > 0 {
		err = json.Unmarshal(report, &j.Report)
	}
	return j, err
}
func (s *Store) Projects(ctx context.Context) ([]Project, error) {
	return s.ProjectList(ctx, false)
}

func (s *Store) ProjectList(ctx context.Context, archived bool) ([]Project, error) {
	// One consistent statement replaces an extra round-trip per project. The
	// lateral lookup uses project_job_history and includes immutable job details.
	rows, err := s.DB.QueryContext(ctx, `SELECT to_jsonb(p) || jsonb_build_object('last_job',latest.data)
FROM (SELECT `+projectCols+` FROM projects WHERE archived=$1 ORDER BY created_at DESC,id DESC LIMIT 500) p
LEFT JOIN LATERAL (SELECT to_jsonb(j) AS data FROM jobs j WHERE j.project_id=p.id ORDER BY j.queued_at DESC,j.id DESC LIMIT 1) latest ON true
ORDER BY p.created_at DESC,p.id DESC`, archived)
	if err != nil {
		return nil, err
	}
	result := []Project{}
	for rows.Next() {
		var p Project
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return result, nil
}
func (s *Store) SaveProject(ctx context.Context, p Project, create bool) (Project, error) {
	p.Archived = false
	if p.Source.Kind == "" {
		p.Source.Kind = "local"
	}
	sourceRaw, err := json.Marshal(p.Source)
	if err != nil {
		return p, err
	}
	raw, err := json.Marshal(p.Config)
	if err != nil {
		return p, err
	}
	if create {
		p.ID = NewID()
		err = s.DB.QueryRowContext(ctx, `INSERT INTO projects(id,name,backup_pattern,config,schedule_time,timezone,next_run_at,source) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING created_at`, p.ID, p.Name, p.BackupPattern, raw, p.ScheduleTime, p.Timezone, p.NextRunAt, sourceRaw).Scan(&p.CreatedAt)
	} else {
		err = s.DB.QueryRowContext(ctx, `UPDATE projects SET name=$2,backup_pattern=$3,config=$4,schedule_time=$5,timezone=$6,next_run_at=$7,source=$8 WHERE id=$1 AND NOT archived RETURNING created_at`, p.ID, p.Name, p.BackupPattern, raw, p.ScheduleTime, p.Timezone, p.NextRunAt, sourceRaw).Scan(&p.CreatedAt)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}
func (s *Store) Archive(ctx context.Context, id string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var found string
	if err = tx.QueryRowContext(ctx, "SELECT id FROM projects WHERE id=$1 AND NOT archived FOR UPDATE", id).Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	var active bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM jobs WHERE project_id=$1 AND status IN ('queued','running'))", id).Scan(&active); err != nil {
		return err
	}
	if active {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, "UPDATE projects SET archived=true,next_run_at=NULL WHERE id=$1", id); err != nil {
		return err
	}
	return tx.Commit()
}

// An explicit restore preserves source/settings/history but never silently
// resumes the old schedule. Operators choose a new schedule after review.
func (s *Store) RestoreProject(ctx context.Context, id string) (Project, error) {
	p, err := scanProject(s.DB.QueryRowContext(ctx, `UPDATE projects SET archived=false,schedule_time='',next_run_at=NULL WHERE id=$1 AND archived RETURNING `+projectCols, id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

func (s *Store) Enqueue(ctx context.Context, id, trigger string) (Job, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback()
	p, err := scanProject(tx.QueryRowContext(ctx, "SELECT "+projectCols+" FROM projects WHERE id=$1 AND NOT archived FOR UPDATE", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, err
	}
	j, err := enqueue(ctx, tx, p, trigger)
	if err != nil {
		return j, err
	}
	return j, tx.Commit()
}
func enqueue(ctx context.Context, tx *sql.Tx, p Project, trigger string) (Job, error) {
	raw, err := json.Marshal(p.Config)
	if err != nil {
		return Job{}, err
	}
	sourceRaw, err := json.Marshal(p.Source)
	if err != nil {
		return Job{}, err
	}
	j, err := scanJob(tx.QueryRowContext(ctx, `INSERT INTO jobs(id,project_id,project_name,backup_pattern,config,status,trigger,source) VALUES($1,$2,$3,$4,$5,'queued',$6,$7) RETURNING `+jobCols, NewID(), p.ID, p.Name, p.BackupPattern, raw, trigger, sourceRaw))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return j, ErrConflict
	}
	return j, err
}
func (s *Store) Jobs(ctx context.Context, projectID string) ([]Job, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT "+jobCols+" FROM jobs WHERE ($1='' OR project_id=$1) ORDER BY queued_at DESC,id DESC LIMIT 100", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, j)
	}
	return result, rows.Err()
}
func (s *Store) Job(ctx context.Context, id string) (Job, error) {
	j, err := scanJob(s.DB.QueryRowContext(ctx, "SELECT "+jobCols+" FROM jobs WHERE id=$1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return j, ErrNotFound
	}
	if err == nil {
		j.Attempts, err = s.Attempts(ctx, id)
	}
	return j, err
}
func (s *Store) Cancel(ctx context.Context, id string) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE jobs SET cancel_requested=true,retry_at=NULL,error=CASE WHEN status='queued' THEN 'Cancelled by user.' ELSE error END,status=CASE WHEN status='queued' THEN 'cancelled' ELSE status END,finished_at=CASE WHEN status='queued' THEN clock_timestamp() ELSE finished_at END WHERE id=$1 AND status IN ('queued','running')`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) Claim(ctx context.Context) (*Job, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var id string
	if err = tx.QueryRowContext(ctx, "SELECT id FROM jobs WHERE status='queued' AND (retry_at IS NULL OR retry_at<=clock_timestamp()) ORDER BY queued_at FOR UPDATE SKIP LOCKED LIMIT 1").Scan(&id); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	j, err := scanJob(tx.QueryRowContext(ctx, "UPDATE jobs SET status='running',started_at=clock_timestamp(),heartbeat_at=clock_timestamp(),attempt_count=attempt_count+1,retry_at=NULL,report=NULL,error='' WHERE id=$1 RETURNING "+jobCols, id))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &j, nil
}
func (s *Store) Schedule(ctx context.Context, now time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT "+projectCols+" FROM projects WHERE NOT archived AND next_run_at <= $1 ORDER BY next_run_at FOR UPDATE SKIP LOCKED LIMIT 50", now)
	if err != nil {
		return err
	}
	due := []Project{}
	for rows.Next() {
		p, e := scanProject(rows)
		if e != nil {
			rows.Close()
			return e
		}
		due = append(due, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, p := range due {
		// The partial unique index enforces one active job per project, without
		// aborting this transaction when a manual run is already active.
		raw, e := json.Marshal(p.Config)
		if e != nil {
			return e
		}
		sourceRaw, e := json.Marshal(p.Source)
		if e != nil {
			return e
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO jobs(id,project_id,project_name,backup_pattern,config,status,trigger,source) VALUES($1,$2,$3,$4,$5,'queued','schedule',$6) ON CONFLICT DO NOTHING`, NewID(), p.ID, p.Name, p.BackupPattern, raw, sourceRaw)
		if err != nil {
			return err
		}
		next, e := NextRun(p.ScheduleTime, p.Timezone, now)
		if e != nil {
			return e
		}
		if _, err = tx.ExecContext(ctx, "UPDATE projects SET next_run_at=$2 WHERE id=$1", p.ID, next); err != nil {
			return err
		}
	}
	return tx.Commit()
}
