package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lazypanic999/restivra/internal/verify"
)

func TestComparisonSeparatesChangedRulesFromRegressions(t *testing.T) {
	old := Job{Config: verify.DefaultConfig(), Report: &verify.Report{RestoreStatus: "passed", CleanupStatus: "passed", Backup: verify.Backup{SHA256: "abc"}}}
	old.Config.Checks = []verify.Check{{Name: "rows", SQL: "SELECT secret_count > 0"}, {Name: "threshold", SQL: "SELECT private_count > 10"}, {Name: "removed", SQL: "SELECT true"}, {Name: "unexecuted", SQL: "SELECT true"}}
	old.Report.Checks = []verify.CheckResult{{Name: "rows", Status: "passed"}, {Name: "threshold", Status: "passed"}, {Name: "removed", Status: "passed"}, {Name: "unexecuted", Status: "passed"}}
	current := old
	current.Config.Checks = []verify.Check{{Name: "rows", SQL: "SELECT secret_count > 0"}, {Name: "threshold", SQL: "SELECT private_count > 100"}, {Name: "added", SQL: "SELECT true"}, {Name: "unexecuted", SQL: "SELECT true"}}
	current.Report = &verify.Report{RestoreStatus: "passed", CleanupStatus: "failed", Backup: verify.Backup{SHA256: "abc"}, Checks: []verify.CheckResult{{Name: "rows", Status: "failed"}, {Name: "threshold", Status: "failed"}, {Name: "added", Status: "passed"}}}
	got := compareRuns(current, &old)
	if got.Content != "same" || got.Cleanup.Change != "regressed" || got.Restore.Change != "unchanged" || got.LimitsChanged || got.SourceChanged {
		t.Fatalf("wrong context: %+v", got)
	}
	want := map[string]string{"rows": "regressed", "threshold": "definition_changed", "added": "added", "removed": "removed", "unexecuted": "not_comparable"}
	for _, row := range got.Checks {
		if row.Change != want[row.Name] {
			t.Fatalf("misleading comparison: %+v", row)
		}
		delete(want, row.Name)
	}
	if len(want) != 0 {
		t.Fatal("checks omitted", want)
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), "secret_count") || strings.Contains(string(encoded), "private_count") || strings.Contains(string(encoded), "SELECT") {
		t.Fatal("comparison leaked SQL")
	}
}

func testComparisonEndpoint(t *testing.T, ctx context.Context, store *Store, template Project) {
	t.Helper()
	p := template
	p.ID = ""
	p.Name = "Comparison history fixture"
	p.ScheduleTime = ""
	p.NextRunAt = nil
	p.Config.Checks = []verify.Check{{Name: "rows", SQL: "SELECT true"}}
	p, err := store.SaveProject(ctx, p, true)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	insert := func(project Project, offset int, status string, report *verify.Report) string {
		id := NewID()
		cfg, _ := json.Marshal(project.Config)
		var raw any
		if report != nil {
			data, e := json.Marshal(report)
			if e != nil {
				t.Fatal(e)
			}
			raw = data
		}
		_, e := store.DB.ExecContext(ctx, `INSERT INTO jobs(id,project_id,project_name,backup_pattern,config,status,trigger,report,queued_at) VALUES($1,$2,$3,'fixture.dump',$4,$5,'manual',$6,$7)`, id, project.ID, project.Name, cfg, status, raw, base.Add(time.Duration(offset)*time.Second))
		if e != nil {
			t.Fatal(e)
		}
		return id
	}
	passed := &verify.Report{Status: "passed", RestoreStatus: "passed", CleanupStatus: "passed", Backup: verify.Backup{SHA256: "same"}, Checks: []verify.CheckResult{{Name: "rows", Status: "passed"}}}
	failed := *passed
	failed.Status = "failed"
	failed.Checks = []verify.CheckResult{{Name: "rows", Status: "failed"}}
	first := insert(p, 0, "passed", passed)
	foreign := insert(template, 1, "passed", passed)
	noReport := insert(p, 2, "cancelled", nil)
	running := insert(p, 3, "running", passed)
	current := insert(p, 4, "failed", &failed)
	future := insert(p, 5, "passed", passed)
	a := API{Store: store, AllowedHosts: []string{"localhost:7474"}, CSRFToken: NewID()}
	h := a.Handler()
	get := func(id, query string) (int, runComparison) {
		r := requestJSON(t, h, "GET", "/api/jobs/"+id+"/comparison"+query, nil, "")
		if strings.Contains(r.Body.String(), "SELECT true") || strings.Contains(r.Body.String(), `"config"`) {
			t.Fatal("comparison exported SQL settings")
		}
		var result runComparison
		if r.Code == 200 && json.Unmarshal(r.Body.Bytes(), &result) != nil {
			t.Fatal("invalid comparison response")
		}
		return r.Code, result
	}
	code, result := get(current, "")
	if code != 200 || result.Baseline == nil || result.Baseline.ID != first || result.Checks[0].Change != "regressed" {
		t.Fatalf("wrong previous completed report: %d %+v", code, result)
	}
	if code, result = get(first, ""); code != 200 || result.Baseline != nil {
		t.Fatal("first run invented predecessor")
	}
	for _, tc := range []struct {
		id, query string
		code      int
	}{{current, "?baseline=" + first, 200}, {current, "?baseline=" + foreign, 400}, {current, "?baseline=" + current, 400}, {current, "?baseline=" + future, 400}, {current, "?baseline=bad", 400}, {current, "?baseline=" + NewID(), 404}, {current, "?baseline=" + noReport, 409}, {current, "?baseline=" + running, 409}, {noReport, "", 409}, {running, "", 409}} {
		if code, _ := get(tc.id, tc.query); code != tc.code {
			t.Fatalf("comparison selection %q: got %d want %d", tc.query, code, tc.code)
		}
	}
	// This is a persisted-state fixture, with no worker to acknowledge cancel.
	if _, err = store.DB.ExecContext(ctx, "UPDATE jobs SET status='cancelled' WHERE id=$1", running); err != nil {
		t.Fatal(err)
	}
	if err = store.Archive(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if code, _ := get(current, ""); code != 200 {
		t.Fatal("archiving hid retained comparison evidence")
	}
}

func TestComparisonRequiresEvidenceAndFlagsChangedContext(t *testing.T) {
	old := Job{Config: verify.DefaultConfig(), Report: &verify.Report{Image: "pg17", ImageID: "old", Backup: verify.Backup{SHA256: "a"}}}
	old.Config.Checks = []verify.Check{{Name: "rows", SQL: "SELECT true"}}
	old.Report.Checks = []verify.CheckResult{{Name: "rows", Status: "error"}}
	current := old
	current.Config.MemoryMB++
	current.BackupPattern = "another.dump"
	current.Report = &verify.Report{Image: "pg17", ImageID: "new", Backup: verify.Backup{SHA256: "b"}, Checks: []verify.CheckResult{{Name: "rows", Status: "passed"}}}
	got := compareRuns(current, &old)
	if !got.LimitsChanged || !got.SourceChanged || !got.ImageChanged || got.Content != "different" || got.Checks[0].Change != "improved" {
		t.Fatalf("context changes lost: %+v", got)
	}
	current.Report.Backup.SHA256 = ""
	if compareRuns(current, &old).Content != "unknown" {
		t.Fatal("missing fingerprint interpreted as a different archive")
	}
	if compareRuns(current, nil).Baseline != nil {
		t.Fatal("invented a baseline")
	}
	old.Config.Checks = nil
	if compareRuns(current, &old).Checks[0].Change != "not_comparable" {
		t.Fatal("legacy missing query certified as comparable")
	}
}
