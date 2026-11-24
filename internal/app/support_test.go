package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSupportIsAuthenticatedAndDoesNotNeedAWorkingDatabaseToExplainFailure(t *testing.T) {
	auth, err := NewAuth(strings.Repeat("private-token-", 4))
	if err != nil {
		t.Fatal(err)
	}
	api := API{AllowedHosts: []string{"localhost:7474"}, Auth: auth}
	for _, path := range []string{"/api/support", "/api/support/download"} {
		response := requestJSON(t, api.Handler(), "GET", path, nil, "")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("support endpoint exposed without sign-in: %d", response.Code)
		}
	}
	// A legacy local operator can still obtain a useful, sanitized report when
	// the database has never initialized or is unavailable.
	api.Auth = nil
	response := requestJSON(t, api.Handler(), "GET", "/api/support/download", nil, "")
	var snapshot SupportSnapshot
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &snapshot) != nil {
		t.Fatal("cannot obtain degraded support report")
	}
	if snapshot.Database != "unavailable" || snapshot.Worker != "unknown" || snapshot.Counts != nil || snapshot.PostgresMajor != nil {
		t.Fatal("database failure was misrepresented as empty/healthy")
	}
	if response.Header().Get("Content-Disposition") != `attachment; filename="restivra-support.json"` {
		t.Fatal("support download must use a fixed safe filename")
	}
	if !strings.Contains(response.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("support response must not be cached")
	}
	if strings.Contains(response.Body.String(), "private-token") || strings.Contains(response.Body.String(), "csrf_token") {
		t.Fatal("support report leaked a credential")
	}
}

func testSupportEndpoint(t *testing.T, ctx context.Context, store *Store, template Project) {
	t.Helper()
	canary := "support-private-" + NewID()
	api := API{Store: store, Instance: canary, AllowedHosts: []string{"localhost:7474"}}
	baseline := api.supportSnapshot(ctx)
	if baseline.Database != "online" || baseline.Counts == nil || baseline.Worker != "not_seen" || baseline.PostgresMajor == nil || *baseline.PostgresMajor != 17 {
		t.Fatal("support snapshot failed on the real metadata database")
	}
	before := *baseline.Counts
	var projects []Project
	for i := 0; i < 4; i++ {
		p := template
		p.ID, p.Name, p.BackupPattern, p.ScheduleTime = "", canary, canary+"/*.dump", ""
		if i == 0 {
			p.ScheduleTime = "03:00"
		}
		p, err := store.SaveProject(ctx, p, true)
		if err != nil {
			t.Fatal(err)
		}
		projects = append(projects, p)
	}
	if err := store.Archive(ctx, projects[3].ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		job, err := store.Enqueue(ctx, projects[i].ID, "manual")
		if err != nil {
			t.Fatal(err)
		}
		switch i {
		case 0:
			_, err = store.DB.ExecContext(ctx, "UPDATE projects SET next_run_at=now()-interval '2 minutes' WHERE id=$1", projects[i].ID)
			if err == nil {
				_, err = store.DB.ExecContext(ctx, "UPDATE jobs SET notification_status='failed' WHERE id=$1", job.ID)
			}
		case 1:
			_, err = store.DB.ExecContext(ctx, "UPDATE jobs SET retry_at=now()+interval '1 hour' WHERE id=$1", job.ID)
		case 2:
			_, err = store.DB.ExecContext(ctx, "UPDATE jobs SET status='running',started_at=now() WHERE id=$1", job.ID)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	// A different worker must not make this instance appear online.
	if _, err := store.DB.ExecContext(ctx, "INSERT INTO workers(instance,heartbeat_at) VALUES($1,now())", canary+"-other"); err != nil {
		t.Fatal(err)
	}
	snapshot := api.supportSnapshot(ctx)
	if snapshot.Worker != "not_seen" {
		t.Fatal("another instance's heartbeat was used")
	}
	c := snapshot.Counts
	if c == nil || c.ActiveProjects-before.ActiveProjects != 3 || c.ArchivedProjects-before.ArchivedProjects != 1 || c.Scheduled-before.Scheduled != 1 || c.Overdue-before.Overdue != 1 || c.Queued-before.Queued != 1 || c.Deferred-before.Deferred != 1 || c.Running-before.Running != 1 || c.NotificationFail-before.NotificationFail != 1 {
		t.Fatal("aggregate counters lost scheduled/deferred/running work")
	}
	for _, tc := range []struct{ interval, want string }{{"0 seconds", "online"}, {"-1 minute", "offline"}, {"1 minute", "clock_skew"}} {
		if _, err := store.DB.ExecContext(ctx, "INSERT INTO workers(instance,heartbeat_at) VALUES($1,now()+$2::interval) ON CONFLICT(instance) DO UPDATE SET heartbeat_at=excluded.heartbeat_at", canary, tc.interval); err != nil {
			t.Fatal(err)
		}
		if got := api.supportSnapshot(ctx).Worker; got != tc.want {
			t.Fatalf("heartbeat %s: got %s want %s", tc.interval, got, tc.want)
		}
	}
	response := requestJSON(t, api.Handler(), "GET", "/api/support/download", nil, "")
	if response.Code != 200 || strings.Contains(response.Body.String(), canary) || strings.Contains(response.Body.String(), template.BackupPattern) {
		t.Fatal("support download exposed private project/worker/archive information")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	for key := range fields {
		switch key {
		case "format", "generated_at", "build", "authentication_enabled", "database", "worker", "worker_age_seconds", "postgres_major", "counts", "notices":
		default:
			t.Fatalf("unexpected support field %q", key)
		}
	}
	// Cancel only synthetic work created by this helper.
	for _, p := range projects {
		if _, err := store.DB.ExecContext(ctx, "UPDATE jobs SET status='cancelled',finished_at=now() WHERE project_id=$1 AND status IN ('queued','running')", p.ID); err != nil {
			t.Fatal(err)
		}
	}
}
