package app

import (
	"database/sql"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRenderMetrics(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	beat := now.Add(-4 * time.Second)
	finished := now.Add(-time.Hour)
	success := now.Add(-26 * time.Hour)
	out := renderMetrics(metricsSnapshot{
		Now: now, WorkerHeartbeat: &beat, NotifyFailed: 2,
		JobsByStatus: map[string]int64{"passed": 40, "failed": 3},
		Projects: []projectMetric{
			{ID: "b", Name: `Clinic "east"`, LastStatus: "failed", LastFinished: &finished, LastSuccess: &success,
				LastDurationMS: sql.NullInt64{Int64: 11800, Valid: true}, LastBackupBytes: sql.NullInt64{Int64: 38240512, Valid: true}},
			{ID: "a", Name: "New project"},
		},
	})
	for _, want := range []string{
		"restivra_up 1\n",
		"restivra_worker_online 1\n",
		"restivra_worker_heartbeat_age_seconds 4.000\n",
		`restivra_jobs{status="passed"} 40` + "\n",
		`restivra_jobs{status="running"} 0` + "\n",
		"restivra_notifications_failed 2\n",
		`restivra_project_last_run_passed{project_id="b",project="Clinic \"east\""} 0` + "\n",
		`restivra_project_last_success_timestamp_seconds{project_id="b",project="Clinic \"east\""} 1.7955144e+09` + "\n",
		`restivra_project_last_restore_duration_seconds{project_id="b",project="Clinic \"east\""} 11.8` + "\n",
		`restivra_project_last_backup_bytes{project_id="b",project="Clinic \"east\""} 3.8240512e+07` + "\n",
		"# TYPE restivra_project_last_run_passed gauge\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, `project_id="a"`) {
		t.Errorf("project without finished runs produced series:\n%s", out)
	}
	stale := now.Add(-time.Minute)
	if !strings.Contains(renderMetrics(metricsSnapshot{Now: now, WorkerHeartbeat: &stale}), "restivra_worker_online 0\n") {
		t.Error("stale heartbeat reported online")
	}
}

func TestPromLabelEscaping(t *testing.T) {
	if got := promLabel("a\\b\"c\nd"); got != `a\\b\"c\nd` {
		t.Fatalf("got %q", got)
	}
}

func TestMetricsBearerToken(t *testing.T) {
	token := strings.Repeat("k", 40)
	auth, err := NewAuth(token)
	if err != nil {
		t.Fatal(err)
	}
	api := API{Auth: auth, AllowedHosts: []string{"localhost:7474"}, CSRFToken: "x"}
	handler := api.Handler()
	for header, want := range map[string]int{"": 401, "Bearer wrong": 401, "Basic " + token: 401, "Bearer " + token: 503} {
		req := httptest.NewRequest("GET", "http://localhost:7474/metrics", nil)
		req.Host = "localhost:7474"
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		out := httptest.NewRecorder()
		// Without a store, an authenticated request reaches the handler and gets 503.
		handler.ServeHTTP(out, req)
		if out.Code != want {
			t.Errorf("Authorization %q: got %d want %d", header, out.Code, want)
		}
	}
	var nilAuth *Auth
	if !nilAuth.bearer(httptest.NewRequest("GET", "/metrics", nil)) {
		t.Error("disabled authentication must allow metrics")
	}
}
