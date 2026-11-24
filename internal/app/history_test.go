package app

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestHistoryRejectsInvalidFilters(t *testing.T) {
	for _, query := range []string{"status=unknown", "notification=unknown", "notification=Failed", "notification=failed&notification=delivered", "notification_status=failed", "q=first&q=second", "project_id=invalid", "limit=0", "limit=101", "limit=all", "since=yesterday", "since=2026-01-01", "since=2026-01-02T00:00:00Z&until=2026-01-01T00:00:00Z", "cursor=invalid", "q=" + strings.Repeat("a", 481)} {
		q, _ := url.ParseQuery(query)
		if _, err := parseHistoryFilter(q); err == nil {
			t.Fatalf("accepted %q", query)
		}
	}
	f, err := parseHistoryFilter(url.Values{"q": {" %_ "}})
	if err != nil || f.Search != "%_" || f.Limit != 50 {
		t.Fatal("literal search/default limit changed")
	}
	for _, value := range []string{"=1+1", " +SUM(A1)", "\t@SUM(1)", "\r=1", "\n=1", "-1+2"} {
		if !strings.HasPrefix(csvText(value), "'") {
			t.Fatalf("CSV formula not neutralized: %q", value)
		}
	}
	if csvText("Ödeme, \"archive\"") != "Ödeme, \"archive\"" {
		t.Fatal("safe CSV text changed")
	}
}

// Reuses the real isolated PostgreSQL fixture, after its restore worker stops.
func testHistoryAndProjectRestore(t *testing.T, ctx context.Context, store *Store, template Project) {
	t.Helper()
	marker := "history-" + NewID()
	name := marker + " Ödeme %_ \"quoted\""
	template.ID, template.Name = "", name
	template.ScheduleTime = "03:00"
	next := time.Now().Add(time.Hour)
	template.NextRunAt = &next
	p, err := store.SaveProject(ctx, template, true)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 3, 29, 0, 30, 0, 0, time.UTC)
	_, err = store.DB.ExecContext(ctx, `INSERT INTO jobs(id,project_id,project_name,backup_pattern,config,status,trigger,queued_at,started_at,finished_at,attempt_count,report,notification_status)
SELECT md5($1||n::text),$1,$2,'fixture.dump','{}',CASE WHEN n%2=0 THEN 'passed' ELSE 'failed' END,'manual',$3::timestamptz,$3::timestamptz,$3::timestamptz+interval '1 second',1,
jsonb_build_object('duration_ms',1000,'cleanup_status','passed','backup',jsonb_build_object('name','=1+1.dump','bytes',42,'sha256',repeat('a',64))),
CASE WHEN n%2=0 THEN 'skipped' ELSE CASE n%8 WHEN 1 THEN 'pending' WHEN 3 THEN 'delivered' WHEN 5 THEN 'failed' ELSE 'skipped' END END FROM generate_series(1,207) n`, p.ID, p.Name, base)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Archive(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	template.Name = "=1+" + marker
	other, err := store.SaveProject(ctx, template, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.ExecContext(ctx, `INSERT INTO jobs(id,project_id,project_name,backup_pattern,config,status,trigger,queued_at) SELECT md5($1||n::text),$1,$2,'fixture.dump','{}','cancelled','manual',$3::timestamptz FROM generate_series(1,3) n`, other.ID, other.Name, base.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	api := API{Store: store, AllowedHosts: []string{"localhost:7474"}, CSRFToken: NewID()}
	handler := api.Handler()
	params := url.Values{"q": {marker}, "limit": {"73"}}
	response := requestJSON(t, handler, "GET", "/api/history?"+params.Encode(), nil, "")
	var page HistoryPage
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil || page.Summary.Total != 210 || len(page.Items) != 73 || page.NextCursor == "" {
		t.Fatalf("history first page failed: %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), `"config"`) || strings.Contains(response.Body.String(), `"sql"`) {
		t.Fatal("history lists exposed check configuration")
	}
	firstCursor := page.NextCursor
	for _, state := range []struct {
		name  string
		count int64
	}{{"pending", 26}, {"delivered", 26}, {"failed", 26}, {"skipped", 129}} {
		filtered := url.Values{"project_id": {p.ID}, "notification": {state.name}, "limit": {"11"}}
		r := requestJSON(t, handler, "GET", "/api/history?"+filtered.Encode(), nil, "")
		var deliveryPage HistoryPage
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &deliveryPage) != nil || deliveryPage.Summary.Total != state.count || len(deliveryPage.Items) != 11 || deliveryPage.NextCursor == "" {
			t.Fatalf("delivery filter failed for %s", state.name)
		}
		for _, job := range deliveryPage.Items {
			if job.NotificationStatus != state.name {
				t.Fatal("delivery page contains a different notification state")
			}
		}
		filtered.Set("notification", "failed")
		if state.name == "failed" {
			filtered.Set("notification", "pending")
		}
		filtered.Set("cursor", deliveryPage.NextCursor)
		if requestJSON(t, handler, "GET", "/api/history?"+filtered.Encode(), nil, "").Code != 400 {
			t.Fatal("delivery cursor reused with a different notification filter")
		}
	}
	failedDelivery := url.Values{"project_id": {p.ID}, "notification": {"failed"}, "status": {"failed"}}
	r := requestJSON(t, handler, "GET", "/api/history/export.csv?"+failedDelivery.Encode(), nil, "")
	deliveryRows, err := csv.NewReader(strings.NewReader(r.Body.String())).ReadAll()
	if r.Code != 200 || err != nil || len(deliveryRows) != 27 {
		t.Fatal("failed-delivery CSV did not export every filtered archived-project row")
	}
	for _, row := range deliveryRows[1:] {
		if row[3] != "failed" || row[15] != "failed" {
			t.Fatal("delivery-filtered CSV mixed verification or notification outcomes")
		}
	}
	seen := map[string]bool{}
	for _, job := range page.Items {
		seen[job.ID] = true
	}
	// A newer job inserted between page requests must not move the boundary or
	// duplicate/skip old jobs, even when 207 timestamps collide exactly.
	newID := NewID()
	_, err = store.DB.ExecContext(ctx, `INSERT INTO jobs(id,project_id,project_name,backup_pattern,config,status,trigger,queued_at) VALUES($1,$2,$3,'fixture.dump','{}','cancelled','manual',$4)`, newID, other.ID, other.Name, base.Add(48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for page.NextCursor != "" {
		params.Set("cursor", page.NextCursor)
		response = requestJSON(t, handler, "GET", "/api/history?"+params.Encode(), nil, "")
		page = HistoryPage{}
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil {
			t.Fatal("older page unavailable")
		}
		for _, job := range page.Items {
			if seen[job.ID] || job.ID == newID {
				t.Fatal("pagination duplicated a row or included the newer insertion")
			}
			seen[job.ID] = true
		}
	}
	if len(seen) != 210 {
		t.Fatalf("pagination lost jobs beyond 100: %d", len(seen))
	}
	params.Del("cursor")
	response = requestJSON(t, handler, "GET", "/api/history?"+params.Encode(), nil, "")
	if json.Unmarshal(response.Body.Bytes(), &page) != nil || page.Items[0].ID != newID || page.Summary.Total != 211 {
		t.Fatal("latest page missed new activity")
	}
	// Filters apply to all retained rows, including archived sources. SQL LIKE
	// metacharacters must remain literal search text.
	query := url.Values{"q": {"%_"}, "project_id": {p.ID}, "status": {"failed"}}
	response = requestJSON(t, handler, "GET", "/api/history?"+query.Encode(), nil, "")
	if json.Unmarshal(response.Body.Bytes(), &page) != nil || page.Summary.Total != 104 || page.Summary.Failed != 104 {
		t.Fatal("server-side/archive/literal filters failed")
	}
	query = url.Values{"q": {"ÖDEME"}, "project_id": {p.ID}}
	response = requestJSON(t, handler, "GET", "/api/history?"+query.Encode(), nil, "")
	if json.Unmarshal(response.Body.Bytes(), &page) != nil || page.Summary.Total != 207 {
		t.Fatal("Unicode case-insensitive history search failed")
	}
	query.Set("q", "%' OR true --")
	response = requestJSON(t, handler, "GET", "/api/history?"+query.Encode(), nil, "")
	if json.Unmarshal(response.Body.Bytes(), &page) != nil || page.Summary.Total != 0 {
		t.Fatal("search text was interpreted as SQL")
	}
	query = url.Values{"project_id": {p.ID}, "since": {base.Format(time.RFC3339)}, "until": {base.Add(time.Second).Format(time.RFC3339)}}
	response = requestJSON(t, handler, "GET", "/api/history?"+query.Encode(), nil, "")
	if json.Unmarshal(response.Body.Bytes(), &page) != nil || page.Summary.Total != 207 {
		t.Fatal("inclusive lower date boundary failed")
	}
	query.Set("since", base.Add(-time.Second).Format(time.RFC3339))
	query.Set("until", base.Format(time.RFC3339))
	response = requestJSON(t, handler, "GET", "/api/history?"+query.Encode(), nil, "")
	if json.Unmarshal(response.Body.Bytes(), &page) != nil || page.Summary.Total != 0 {
		t.Fatal("exclusive upper date boundary failed")
	}
	for _, bad := range []string{"q=other&cursor=" + firstCursor, "cursor=" + base64.RawURLEncoding.EncodeToString([]byte(`{"v":9}`)), "limit=101"} {
		if r := requestJSON(t, handler, "GET", "/api/history?"+bad, nil, ""); r.Code != 400 {
			t.Fatal("bad/mismatched cursor accepted")
		}
	}
	params.Del("limit")
	response = requestJSON(t, handler, "GET", "/api/history/export.csv?"+params.Encode(), nil, "")
	rows, err := csv.NewReader(strings.NewReader(response.Body.String())).ReadAll()
	if response.Code != 200 || err != nil || len(rows) != 212 || response.Header().Get("Content-Type") != "text/csv; charset=utf-8" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("full filtered CSV export failed")
	}
	foundUnicode, foundFormula := false, false
	for _, row := range rows[1:] {
		if row[2] == name {
			foundUnicode = true
			if row[10] != "'=1+1.dump" {
				t.Fatal("archive-name formula not neutralized")
			}
		}
		if strings.HasPrefix(row[2], "'=1+") {
			foundFormula = true
		}
	}
	if !foundUnicode || !foundFormula {
		t.Fatal("CSV lost quoted/Unicode fields or formula protection")
	}
	params.Set("cursor", firstCursor)
	if r := requestJSON(t, handler, "GET", "/api/history/export.csv?"+params.Encode(), nil, ""); r.Code != 400 {
		t.Fatal("CSV accepted a page cursor")
	}
	// Exports must fail explicitly rather than silently truncate at the cap.
	capProject := template
	capProject.Name = "export-cap-" + NewID()
	capProject, err = store.SaveProject(ctx, capProject, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.ExecContext(ctx, `INSERT INTO jobs(id,project_id,project_name,backup_pattern,config,status,trigger) SELECT md5($1||n::text),$1,$2,'fixture.dump','{}','passed','manual' FROM generate_series(1,$3::int) n`, capProject.ID, capProject.Name, HistoryExportLimit+1)
	if err != nil {
		t.Fatal(err)
	}
	if r := requestJSON(t, handler, "GET", "/api/history/export.csv?project_id="+capProject.ID, nil, ""); r.Code != 422 {
		t.Fatal("oversize CSV was truncated instead of rejected")
	}
	if _, err = store.DB.ExecContext(ctx, "DELETE FROM jobs WHERE id IN (SELECT id FROM jobs WHERE project_id=$1 LIMIT 1)", capProject.ID); err != nil {
		t.Fatal(err)
	}
	response = requestJSON(t, handler, "GET", "/api/history/export.csv?project_id="+capProject.ID, nil, "")
	rows, err = csv.NewReader(strings.NewReader(response.Body.String())).ReadAll()
	if response.Code != 200 || err != nil || len(rows) != HistoryExportLimit+1 {
		t.Fatal("CSV export failed at its exact supported limit")
	}
	response = requestJSON(t, handler, "GET", "/api/projects?archived=true", nil, "")
	var archived []Project
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &archived) != nil {
		t.Fatal("archived projects unavailable")
	}
	found := false
	for _, item := range archived {
		if item.ID == p.ID {
			found = item.Archived && len(item.Config.Checks) == len(template.Config.Checks)
		}
	}
	if !found {
		t.Fatal("archived project lost settings")
	}
	response = requestJSON(t, handler, "POST", "/api/projects/"+p.ID+"/restore", map[string]any{}, api.CSRFToken)
	var restored Project
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &restored) != nil || restored.Archived || restored.ScheduleTime != "" || restored.NextRunAt != nil || restored.Name != p.Name || restored.BackupPattern != p.BackupPattern || !reflect.DeepEqual(restored.Config, p.Config) {
		t.Fatal("project restore changed settings or resumed scheduling")
	}
	if r := requestJSON(t, handler, "POST", "/api/projects/"+p.ID+"/restore", map[string]any{}, api.CSRFToken); r.Code != 404 {
		t.Fatal("restore re-applied to active project")
	}
	var count int
	if err = store.DB.QueryRowContext(ctx, "SELECT count(*) FROM jobs WHERE project_id=$1", p.ID).Scan(&count); err != nil || count != 207 {
		t.Fatal("project restore lost historical jobs")
	}
	if r := requestJSON(t, handler, "GET", "/api/projects?archived=invalid", nil, ""); r.Code != 400 {
		t.Fatal("invalid archive view accepted")
	}
}
