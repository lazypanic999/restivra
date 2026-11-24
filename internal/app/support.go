package app

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/lazypanic999/restivra/internal/buildinfo"
)

// SupportSnapshot is an explicit allowlist: no project names, identifiers, SQL,
// archive paths, object keys, hostnames, source profiles, logs or credentials.
type SupportSnapshot struct {
	Format                string         `json:"format"`
	GeneratedAt           time.Time      `json:"generated_at"`
	Build                 buildinfo.Info `json:"build"`
	AuthenticationEnabled bool           `json:"authentication_enabled"`
	Database              string         `json:"database"`
	Worker                string         `json:"worker"`
	WorkerAgeSeconds      *int64         `json:"worker_age_seconds"`
	PostgresMajor         *int           `json:"postgres_major"`
	Counts                *SupportCounts `json:"counts"`
	Notices               []string       `json:"notices"`
}

type SupportCounts struct {
	ActiveProjects   int64 `json:"active_projects"`
	ArchivedProjects int64 `json:"archived_projects"`
	Scheduled        int64 `json:"scheduled_projects"`
	Overdue          int64 `json:"overdue_projects"`
	Queued           int64 `json:"queued"`
	Deferred         int64 `json:"deferred"`
	Running          int64 `json:"running"`
	NotificationFail int64 `json:"failed_notifications"`
}

func (a *API) support(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	snapshot := a.supportSnapshot(ctx)
	if r.URL.Path == "/api/support/download" {
		w.Header().Set("Content-Disposition", `attachment; filename="restivra-support.json"`)
	}
	jsonResponse(w, http.StatusOK, snapshot)
}

func (a *API) supportSnapshot(ctx context.Context) SupportSnapshot {
	s := SupportSnapshot{Format: "restivra.support.v1", GeneratedAt: time.Now().UTC(), Build: buildinfo.Current(), AuthenticationEnabled: a.Auth != nil, Database: "unavailable", Worker: "unknown", Notices: []string{}}
	if !s.AuthenticationEnabled {
		s.Notices = append(s.Notices, "authentication_disabled")
	}
	if a.Store == nil || a.Store.DB == nil {
		s.Notices = append(s.Notices, "database_unavailable")
		return s
	}
	// Keep related counters and the heartbeat on one consistent, read-only view.
	tx, err := a.Store.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		s.Notices = append(s.Notices, "database_unavailable")
		return s
	}
	defer tx.Rollback()
	c := SupportCounts{}
	var major int
	var age sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT current_setting('server_version_num')::integer / 10000,
 (SELECT floor(extract(epoch FROM (now()-heartbeat_at)))::bigint FROM workers WHERE instance=$1),
 (SELECT count(*) FROM projects WHERE NOT archived),
 (SELECT count(*) FROM projects WHERE archived),
 (SELECT count(*) FROM projects WHERE NOT archived AND schedule_time<>''),
 (SELECT count(*) FROM projects WHERE NOT archived AND schedule_time<>'' AND next_run_at<now()-interval '1 minute'),
 (SELECT count(*) FROM jobs WHERE status='queued' AND (retry_at IS NULL OR retry_at<=now())),
 (SELECT count(*) FROM jobs WHERE status='queued' AND retry_at>now()),
 (SELECT count(*) FROM jobs WHERE status='running'),
 (SELECT count(*) FROM jobs WHERE notification_status='failed')`, a.Instance).Scan(&major, &age, &c.ActiveProjects, &c.ArchivedProjects, &c.Scheduled, &c.Overdue, &c.Queued, &c.Deferred, &c.Running, &c.NotificationFail)
	if err != nil || tx.Commit() != nil {
		s.Notices = append(s.Notices, "database_unavailable")
		return s
	}
	s.Database, s.PostgresMajor, s.Counts = "online", &major, &c
	if age.Valid {
		s.WorkerAgeSeconds = &age.Int64
		s.Worker = "offline"
		if age.Int64 >= -5 && age.Int64 < 30 {
			s.Worker = "online"
		} else if age.Int64 < -5 {
			s.Worker = "clock_skew"
		}
	} else {
		s.Worker = "not_seen"
	}
	if s.Worker != "online" {
		s.Notices = append(s.Notices, "worker_unavailable")
	}
	if c.Overdue > 0 {
		s.Notices = append(s.Notices, "schedule_overdue")
	}
	if c.NotificationFail > 0 {
		s.Notices = append(s.Notices, "notification_failed")
	}
	return s
}
