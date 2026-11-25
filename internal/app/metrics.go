package app

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// bearer accepts "Authorization: Bearer <access token>" so a scraper can read
// /metrics without a browser session. The token itself is never stored.
func (a *Auth) bearer(r *http.Request) bool {
	if a == nil {
		return true
	}
	value, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || value == "" || len(value) > 256 {
		return false
	}
	received := sha256.Sum256([]byte(value))
	return subtle.ConstantTimeCompare(received[:], a.tokenHash[:]) == 1
}

type projectMetric struct {
	ID, Name        string
	LastStatus      string
	LastFinished    *time.Time
	LastSuccess     *time.Time
	LastDurationMS  sql.NullInt64
	LastBackupBytes sql.NullInt64
}

type metricsSnapshot struct {
	Now             time.Time
	WorkerHeartbeat *time.Time
	JobsByStatus    map[string]int64
	NotifyFailed    int64
	Projects        []projectMetric
}

var jobStatuses = []string{"queued", "running", "passed", "failed", "cancelled", "interrupted"}

func promLabel(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(value)
}

func renderMetrics(s metricsSnapshot) string {
	var b strings.Builder
	head := func(name, kind, help string) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, kind)
	}
	head("restivra_up", "gauge", "Restivra API and metadata database are reachable.")
	b.WriteString("restivra_up 1\n")

	online := 0
	if s.WorkerHeartbeat != nil && workerHeartbeatAt(*s.WorkerHeartbeat, s.Now) {
		online = 1
	}
	head("restivra_worker_online", "gauge", "Restore worker heartbeat is recent.")
	fmt.Fprintf(&b, "restivra_worker_online %d\n", online)
	if s.WorkerHeartbeat != nil {
		head("restivra_worker_heartbeat_age_seconds", "gauge", "Seconds since the restore worker heartbeat.")
		fmt.Fprintf(&b, "restivra_worker_heartbeat_age_seconds %.3f\n", s.Now.Sub(*s.WorkerHeartbeat).Seconds())
	}

	head("restivra_jobs", "gauge", "Retained runs by status.")
	for _, status := range jobStatuses {
		fmt.Fprintf(&b, "restivra_jobs{status=%q} %d\n", status, s.JobsByStatus[status])
	}
	head("restivra_notifications_failed", "gauge", "Retained runs whose failure notification was not delivered after all attempts.")
	fmt.Fprintf(&b, "restivra_notifications_failed %d\n", s.NotifyFailed)

	projects := append([]projectMetric(nil), s.Projects...)
	sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
	label := func(p projectMetric) string {
		return fmt.Sprintf(`project_id="%s",project="%s"`, promLabel(p.ID), promLabel(p.Name))
	}
	type series struct {
		name, help string
		value      func(projectMetric) (float64, bool)
	}
	all := []series{
		{"restivra_project_last_run_passed", "1 when the newest finished run of an active project passed, 0 when it did not.", func(p projectMetric) (float64, bool) {
			if p.LastStatus == "" {
				return 0, false
			}
			if p.LastStatus == "passed" {
				return 1, true
			}
			return 0, true
		}},
		{"restivra_project_last_run_timestamp_seconds", "Finish time of the newest finished run.", func(p projectMetric) (float64, bool) {
			if p.LastFinished == nil {
				return 0, false
			}
			return float64(p.LastFinished.Unix()), true
		}},
		{"restivra_project_last_success_timestamp_seconds", "Finish time of the newest passed run.", func(p projectMetric) (float64, bool) {
			if p.LastSuccess == nil {
				return 0, false
			}
			return float64(p.LastSuccess.Unix()), true
		}},
		{"restivra_project_last_restore_duration_seconds", "Duration of the newest finished run.", func(p projectMetric) (float64, bool) {
			return float64(p.LastDurationMS.Int64) / 1000, p.LastDurationMS.Valid
		}},
		{"restivra_project_last_backup_bytes", "Archive size tested by the newest finished run.", func(p projectMetric) (float64, bool) {
			return float64(p.LastBackupBytes.Int64), p.LastBackupBytes.Valid
		}},
	}
	for _, m := range all {
		head(m.name, "gauge", m.help)
		for _, p := range projects {
			if v, ok := m.value(p); ok {
				fmt.Fprintf(&b, "%s{%s} %g\n", m.name, label(p), v)
			}
		}
	}
	return b.String()
}

func workerHeartbeatAt(beat, now time.Time) bool {
	age := now.Sub(beat)
	return age >= -5*time.Second && age < 30*time.Second
}

func (a *API) collectMetrics(ctx context.Context) (metricsSnapshot, error) {
	s := metricsSnapshot{Now: time.Now(), JobsByStatus: map[string]int64{}}
	err := a.Store.DB.QueryRowContext(ctx, "SELECT heartbeat_at FROM workers WHERE instance=$1", a.Instance).Scan(&s.WorkerHeartbeat)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return s, err
	}
	rows, err := a.Store.DB.QueryContext(ctx, "SELECT status,count(*) FROM jobs GROUP BY status")
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var status string
		var n int64
		if err = rows.Scan(&status, &n); err != nil {
			rows.Close()
			return s, err
		}
		s.JobsByStatus[status] = n
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return s, err
	}
	rows.Close()
	if err = a.Store.DB.QueryRowContext(ctx, "SELECT count(*) FROM jobs WHERE notification_status='failed'").Scan(&s.NotifyFailed); err != nil {
		return s, err
	}
	rows, err = a.Store.DB.QueryContext(ctx, `SELECT p.id,p.name,COALESCE(l.status,''),l.finished_at,
 CASE WHEN l.report->>'duration_ms' ~ '^[0-9]{1,15}$' THEN (l.report->>'duration_ms')::bigint END,
 CASE WHEN l.report->'backup'->>'bytes' ~ '^[0-9]{1,18}$' THEN (l.report->'backup'->>'bytes')::bigint END,
 (SELECT max(finished_at) FROM jobs WHERE project_id=p.id AND status='passed')
FROM projects p
LEFT JOIN LATERAL (SELECT status,finished_at,report FROM jobs WHERE project_id=p.id AND status IN ('passed','failed','interrupted')
 ORDER BY queued_at DESC,id DESC LIMIT 1) l ON true
WHERE NOT p.archived ORDER BY p.created_at DESC,p.id DESC LIMIT 1000`)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var p projectMetric
		if err = rows.Scan(&p.ID, &p.Name, &p.LastStatus, &p.LastFinished, &p.LastDurationMS, &p.LastBackupBytes, &p.LastSuccess); err != nil {
			return s, err
		}
		s.Projects = append(s.Projects, p)
	}
	return s, rows.Err()
}

func (a *API) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !a.Auth.bearer(r) && !a.Auth.authenticated(r) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="restivra"`)
		http.Error(w, "authentication required\n", http.StatusUnauthorized)
		return
	}
	if a.Store == nil {
		http.Error(w, "metrics unavailable\n", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	snapshot, err := a.collectMetrics(ctx)
	if err != nil {
		http.Error(w, "metrics unavailable\n", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(renderMetrics(snapshot)))
}
