package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestProjectPageRejectsAmbiguousAndMismatchedFilters(t *testing.T) {
	queries := []string{"limit=0", "limit=101", "limit=all", "archived=1", "q=a&q=b", "unknown=1", "q=" + strings.Repeat("a", 481), "q=%00", "cursor=invalid"}
	for _, raw := range queries {
		query, _ := url.ParseQuery(raw)
		if _, err := parseProjectFilter(query); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	raw, _ := json.Marshal(projectCursor{At: time.Now().UTC(), ID: strings.Repeat("a", 32), Archived: true, Search: "original"})
	cursor := base64.RawURLEncoding.EncodeToString(raw)
	for _, query := range []url.Values{{"cursor": {cursor}}, {"cursor": {cursor}, "archived": {"true"}, "q": {"changed"}}} {
		if _, err := parseProjectFilter(query); err == nil {
			t.Fatal("cursor crossed project filters")
		}
	}
	f, err := parseProjectFilter(url.Values{"cursor": {cursor}, "archived": {"true"}, "q": {" original "}})
	if err != nil || f.Before == nil || f.Limit != 50 {
		t.Fatal("valid scoped cursor rejected")
	}
	api := API{AllowedHosts: []string{"localhost:7474"}}
	response := requestJSON(t, api.Handler(), "GET", "/api/projects/page?limit=100000", nil, "")
	if response.Code != 400 {
		t.Fatal("invalid page reached the database")
	}
}

func testProjectPagination(t *testing.T, ctx context.Context, store *Store, template Project) {
	t.Helper()
	marker := "pagination-" + NewID()
	cfg, _ := json.Marshal(template.Config)
	_, err := store.DB.ExecContext(ctx, `INSERT INTO projects(id,name,backup_pattern,config,timezone,created_at,archived)
 SELECT md5($1||n::text),$1||CASE WHEN n%31=0 THEN ' %_ ' ELSE ' normal ' END||n::text,'fixture.dump',$2,'UTC',$3,n>610 FROM generate_series(1,620) n`, marker, cfg, time.Date(2026, 1, 1, 0, 0, 0, 123456000, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	api := API{Store: store, AllowedHosts: []string{"localhost:7474"}}
	read := func(query url.Values) ProjectPage {
		t.Helper()
		response := requestJSON(t, api.Handler(), "GET", "/api/projects/page?"+query.Encode(), nil, "")
		var page ProjectPage
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil {
			t.Fatalf("project page failed: %d %s", response.Code, response.Body.String())
		}
		return page
	}
	query := url.Values{"q": {marker}, "limit": {"73"}}
	first := read(query)
	if first.Total != 610 || len(first.Items) != 73 || first.NextCursor == "" {
		t.Fatal("first page was truncated or totals are wrong")
	}
	// Newer rows between page requests must not push old rows into duplicate pages.
	p := template
	p.ID = ""
	p.Name = marker + " newest"
	p.ScheduleTime = ""
	newest, err := store.SaveProject(ctx, p, true)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	ids := []string{}
	page := first
	for attempts := 0; ; attempts++ {
		if attempts > 12 {
			t.Fatal("pagination did not terminate")
		}
		for _, p := range page.Items {
			if seen[p.ID] || p.ID == newest.ID || p.Archived {
				t.Fatal("pagination repeated/crossed a project after a concurrent insert")
			}
			seen[p.ID] = true
			ids = append(ids, p.ID)
		}
		if page.NextCursor == "" {
			break
		}
		query.Set("cursor", page.NextCursor)
		page = read(query)
	}
	if len(seen) != 610 || !sort.SliceIsSorted(ids, func(i, j int) bool { return ids[i] > ids[j] }) {
		t.Fatal("same-timestamp projects were omitted or reordered beyond the old 500 row cap")
	}
	query.Del("cursor")
	latest := read(query)
	if latest.Total != 611 || latest.Items[0].ID != newest.ID {
		t.Fatal("newest page did not pick up the newly created project")
	}
	archived := read(url.Values{"q": {marker}, "archived": {"true"}, "limit": {"100"}})
	if archived.Total != 10 || len(archived.Items) != 10 || archived.NextCursor != "" {
		t.Fatal("archived pagination crossed active data")
	}
	for _, p := range archived.Items {
		if !p.Archived {
			t.Fatal("active project in archive view")
		}
	}
	literal := read(url.Values{"q": {marker + " %_"}, "limit": {"100"}})
	if literal.Total != 19 || len(literal.Items) != 19 {
		t.Fatal("search interpreted literal percent/underscore as SQL wildcards")
	}
	injection := read(url.Values{"q": {marker + "' OR true --"}})
	if injection.Total != 0 || len(injection.Items) != 0 {
		t.Fatal("project search escaped its literal parameter")
	}
	if latest.ActiveTotal < 611 || latest.ArchivedTotal < 10 {
		t.Fatal("overview totals were capped to page size")
	}
}
