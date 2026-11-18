package app

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lazypanic999/restivra/internal/verify"
)

func TestRetryClassification(t *testing.T) {
	for _, tc := range []struct {
		status, stage, code string
		retry               bool
	}{
		{"failed", "engine", "engine_unavailable", true}, {"failed", "start", "database_start_failed", true}, {"failed", "start", "run_timeout", true},
		{"failed", "restore", "run_timeout", false}, {"failed", "restore", "resource_limit", false}, {"failed", "checks", "checks_failed", false},
		{"failed", "archive", "archive_invalid", false}, {"interrupted", "checks", "infrastructure_interrupted", true},
	} {
		if got := transientFailure(tc.status, &verify.Report{Stage: tc.stage, ErrorCode: tc.code}); got != tc.retry {
			t.Errorf("%+v: %v", tc, got)
		}
	}
}

// Real SQL tests called from the isolated PostgreSQL/Docker integration fixture.
func testRetryPolicy(t *testing.T, ctx context.Context, store *Store, template Project) {
	t.Helper()
	template.ID = ""
	template.Name = "Retry policy"
	template.ScheduleTime = ""
	template.NextRunAt = nil
	p, err := store.SaveProject(ctx, template, true)
	if err != nil {
		t.Fatal(err)
	}
	enqueue := func() Job {
		t.Helper()
		j, e := store.Enqueue(ctx, p.ID, "manual")
		if e != nil {
			t.Fatal(e)
		}
		return j
	}
	claim := func(id string, number int) Job {
		t.Helper()
		j, e := store.Claim(ctx)
		if e != nil || j == nil || j.ID != id || j.AttemptCount != number {
			t.Fatalf("claim attempt %d: %+v %v", number, j, e)
		}
		return *j
	}
	finish := func(id, code string) {
		t.Helper()
		e := store.FinishAttempt(ctx, id, "failed", "fixed safe diagnostic", &verify.Report{Stage: "start", Status: "failed", ErrorCode: code, CleanupStatus: "passed"}, true)
		if e != nil {
			t.Fatal(e)
		}
	}
	j := enqueue()
	api := API{Store: store, AllowedHosts: []string{"localhost:7474"}, CSRFToken: NewID()}
	handler := api.Handler()
	for n := 1; n <= MaxAttempts; n++ {
		claim(j.ID, n)
		finish(j.ID, "database_start_failed")
		state, e := store.Job(ctx, j.ID)
		if e != nil {
			t.Fatal(e)
		}
		if len(state.Attempts) != n {
			t.Fatal("attempt evidence was lost")
		}
		// A previous failed report stays downloadable while a fresh retry has
		// no current result. Downloads must identify the selected attempt.
		path := "/api/jobs/" + j.ID + "/report"
		response := requestJSON(t, handler, "GET", path+"?attempt="+strconv.Itoa(n), nil, "")
		var exported verify.Report
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &exported) != nil || exported.Attempt != n || exported.Status != "failed" || !strings.Contains(response.Header().Get("Content-Disposition"), "-attempt-"+strconv.Itoa(n)+".json") {
			t.Fatal("previous attempt download lost identity or failure evidence")
		}
		if n < MaxAttempts {
			if response = requestJSON(t, handler, "GET", path, nil, ""); response.Code != 409 {
				t.Fatal("queued retry exposed a previous report as its current result")
			}
			if state.Status != "queued" || state.RetryAt == nil || !state.RetryAt.After(time.Now()) || state.NotificationStatus != "skipped" {
				t.Fatalf("retry not durably delayed: %+v", state)
			}
			if claimed, e := store.Claim(ctx); e != nil || claimed != nil {
				t.Fatal("delayed retry was claimed early")
			}
			if _, e = store.DB.ExecContext(ctx, "UPDATE jobs SET retry_at=clock_timestamp() WHERE id=$1", j.ID); e != nil {
				t.Fatal(e)
			}
		} else if state.Status != "failed" || state.NotificationStatus != "pending" || state.RetryAt != nil {
			t.Fatal("retry budget/notification boundary failed")
		}
	}
	for _, value := range []string{"0", "4", "-1", "invalid"} {
		response := requestJSON(t, handler, "GET", "/api/jobs/"+j.ID+"/report?attempt="+value, nil, "")
		if response.Code != 400 {
			t.Fatal("invalid attempt number accepted")
		}
	}
	for _, code := range []string{"checks_failed", "archive_invalid", "resource_limit"} {
		j = enqueue()
		claim(j.ID, 1)
		finish(j.ID, code)
		state, e := store.Job(ctx, j.ID)
		if e != nil || state.Status != "failed" || state.AttemptCount != 1 || state.RetryAt != nil {
			t.Fatalf("non-transient failure retried: %s %+v %v", code, state, e)
		}
	}
	j = enqueue()
	claim(j.ID, 1)
	finish(j.ID, "database_start_failed")
	if err = store.Cancel(ctx, j.ID); err != nil {
		t.Fatal(err)
	}
	cancelled, e := store.Job(ctx, j.ID)
	if e != nil || cancelled.Status != "cancelled" || cancelled.RetryAt != nil || cancelled.Error != "Cancelled by user." || len(cancelled.Attempts) != 1 {
		t.Fatal("cancelled retry retained a future retry or lost attempt evidence")
	}
	if next, e := store.Claim(ctx); e != nil || next != nil {
		t.Fatal("cancelled retry ran")
	}
	j = enqueue()
	claim(j.ID, 1)
	if err = store.Cancel(ctx, j.ID); err != nil {
		t.Fatal(err)
	}
	if err = store.FinishAttempt(ctx, j.ID, "passed", "", &verify.Report{Status: "passed", CleanupStatus: "passed"}, false); err != nil {
		t.Fatal(err)
	}
	state, err := store.Job(ctx, j.ID)
	if err != nil || state.Status != "cancelled" || state.Report.Status != "cancelled" {
		t.Fatal("late cancellation was turned into success")
	}
}
