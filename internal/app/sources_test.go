package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lazypanic999/restivra/internal/source"
)

func testSourceSelections(t *testing.T, ctx context.Context, store *Store, template Project) {
	t.Helper()
	p := template
	p.ID = ""
	p.Name = "Immutable remote source"
	p.ScheduleTime = ""
	p.NextRunAt = nil
	p.Source = source.Selection{Kind: "s3", Profile: "original", Bucket: "original-backups"}
	p.BackupPattern = "daily/*.dump"
	if err := p.Validate(time.Now()); err != nil {
		t.Fatal(err)
	}
	saved, err := store.SaveProject(ctx, p, true)
	if err != nil {
		t.Fatal(err)
	}
	job, err := store.Enqueue(ctx, saved.ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	saved.Source = source.Selection{Kind: "s3", Profile: "replacement", Bucket: "replacement-backups"}
	saved.BackupPattern = "other/*.dump"
	if _, err = store.SaveProject(ctx, saved, false); err != nil {
		t.Fatal(err)
	}
	queued, err := store.Job(ctx, job.ID)
	if err != nil || queued.Source != p.Source || queued.BackupPattern != p.BackupPattern {
		t.Fatal("project edit rewrote queued source selection")
	}
	if err = store.Cancel(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	api := API{Store: store, Instance: "source-catalog-" + NewID(), AllowedHosts: []string{"localhost:7474"}, CSRFToken: NewID()}
	handler := api.Handler()
	// Only explicitly public fields survive even unexpected persisted fields.
	raw := `[{"id":"offsite","name":"Offsite storage","secret_access_key":"secret-sentinel","endpoint":"https://private-endpoint.invalid"}]`
	if _, err = store.DB.ExecContext(ctx, "INSERT INTO workers(instance,heartbeat_at,source_profiles) VALUES($1,clock_timestamp(),$2)", api.Instance, []byte(raw)); err != nil {
		t.Fatal(err)
	}
	response := requestJSON(t, handler, "GET", "/api/sources", nil, "")
	if response.Code != 200 || strings.Contains(response.Body.String(), "secret-sentinel") || strings.Contains(response.Body.String(), "private-endpoint") {
		t.Fatal("source catalog leaked private data")
	}
	var catalog struct {
		Profiles []source.PublicProfile `json:"profiles"`
		Online   bool                   `json:"worker_online"`
	}
	if json.Unmarshal(response.Body.Bytes(), &catalog) != nil || !catalog.Online || len(catalog.Profiles) != 1 || catalog.Profiles[0].ID != "offsite" {
		t.Fatal("source catalog missing public profile")
	}
	if _, err = store.DB.ExecContext(ctx, "UPDATE workers SET heartbeat_at=clock_timestamp()-interval '1 minute' WHERE instance=$1", api.Instance); err != nil {
		t.Fatal(err)
	}
	response = requestJSON(t, handler, "GET", "/api/sources", nil, "")
	if json.Unmarshal(response.Body.Bytes(), &catalog) != nil || catalog.Online {
		t.Fatal("stale worker advertised as online")
	}
	// Credentials are not accepted in either top-level project settings or the
	// source selection object. Never silently persist accidental secrets.
	encoded, _ := json.Marshal(p)
	injected := strings.Replace(string(encoded), `"profile":"original"`, `"profile":"original","access_key_id":"secret-sentinel"`, 1)
	response = requestJSON(t, handler, "POST", "/api/projects", json.RawMessage(injected), api.CSRFToken)
	if response.Code != 400 {
		t.Fatal("project source accepted credentials")
	}
	page, err := store.History(ctx, HistoryFilter{ProjectID: saved.ID, Limit: 50}, false)
	if err != nil || len(page.Items) != 1 || page.Items[0].SourceKind != "s3" || page.Items[0].SourceBucket != "original-backups" {
		t.Fatal("history lost immutable source metadata")
	}
}
