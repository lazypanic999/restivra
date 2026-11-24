package app

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lazypanic999/restivra/internal/buildinfo"
	"github.com/lazypanic999/restivra/internal/verify"
)

func evidenceFixture() (Job, *verify.Report) {
	now := time.Date(2026, 11, 20, 2, 0, 0, 0, time.UTC)
	report := &verify.Report{SchemaVersion: 1, RunID: strings.Repeat("a", 32), Status: "failed", Attempt: 1, StartedAt: now, FinishedAt: now.Add(time.Second), DurationMS: 1000, Image: verify.Image, ImageID: "sha256:fixture", RestoreStatus: "passed", CleanupStatus: "passed", ErrorCode: "checks_failed", Checks: []verify.CheckResult{{Name: `<img src=x onerror=alert(1)>`, Status: "failed", ErrorCode: "check_false"}}, Backup: verify.Backup{Name: "private-archive.dump", Bytes: 2048, ModifiedAt: now, SHA256: strings.Repeat("f", 64), Source: &verify.BackupSource{Kind: "s3", Profile: "private-profile", Bucket: "private-bucket", Key: "customer-specific/key.dump", VersionID: "private-version", ETag: `"private-tag"`}}}
	job := Job{ID: report.RunID, ProjectName: `<script>alert("project")</script>`, QueuedAt: now, Status: "passed", AttemptCount: 2, Config: verify.Config{Checks: []verify.Check{{Name: "not exported", SQL: "SELECT 'secret-sql-sentinel'"}}}, Attempts: []Attempt{{Number: 1, Status: "failed", FinishedAt: now.Add(time.Second), Report: report}, {Number: 2, Status: "passed", FinishedAt: now.Add(time.Minute)}}}
	return job, report
}

func TestEvidenceEscapesNamesExcludesSQLAndRedactsIdentifiers(t *testing.T) {
	j, report := evidenceFixture()
	for _, lang := range []string{"en", "tr"} {
		for _, redact := range []bool{false, true} {
			data := makeEvidence(j, report, lang, redact)
			var body bytes.Buffer
			if err := evidenceTemplate.Execute(&body, data); err != nil {
				t.Fatal(err)
			}
			html := body.String()
			for _, forbidden := range []string{`<script>`, `<img src=x`, `secret-sql-sentinel`, `secret_access_key`, `access_key_id`, `<iframe`, `<link `} {
				if strings.Contains(html, forbidden) {
					t.Fatalf("unsafe or private export field: %s", forbidden)
				}
			}
			if !strings.Contains(html, strings.Repeat("f", 64)) || !strings.Contains(html, "checks_failed") || !strings.Contains(html, "check_false") {
				t.Fatal("evidence omitted fingerprint or outcome codes")
			}
			if redact {
				for _, secret := range []string{"private-archive", "private-profile", "private-bucket", "customer-specific", "private-version", "private-tag", "onerror=alert"} {
					if strings.Contains(html, secret) {
						t.Fatal("redacted export retained a hidden identifier")
					}
				}
			} else if !strings.Contains(html, "&lt;script&gt;") || !strings.Contains(html, "private-archive.dump") {
				t.Fatal("original escaped metadata missing")
			}
			if lang == "tr" && (!strings.Contains(html, "Geri yükleme doğrulama kanıtı") || !strings.Contains(html, "Başarısız")) {
				t.Fatal("Turkish export not translated")
			}
			if data.Result == data.CurrentState || data.Attempt != 1 {
				t.Fatal("older attempt misrepresented by current job status")
			}
		}
	}
}

func TestEvidenceUsesSelectedReportsVerifierInsteadOfCurrentServer(t *testing.T) {
	j, report := evidenceFixture()
	for _, lang := range []string{"en", "tr"} {
		legacy := makeEvidence(j, report, lang, false)
		if legacy.VerifierVersion != legacy.Labels["buildNotRecorded"] || legacy.VerifierRevision != "" {
			t.Fatal("legacy report was assigned a verifier it did not record")
		}
	}
	report.ApplicationBuild = &buildinfo.Info{Version: "v0.0.1-retained", Revision: strings.Repeat("b", 40)}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var retained verify.Report
	if err = json.Unmarshal(raw, &retained); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"en", "tr"} {
		for _, redact := range []bool{false, true} {
			d := makeEvidence(j, &retained, lang, redact)
			if d.VerifierVersion != "v0.0.1-retained" || d.VerifierRevision != strings.Repeat("b", 40) {
				t.Fatal("selected attempt lost its original verifier after JSON reload or redaction")
			}
			var body bytes.Buffer
			if err = evidenceTemplate.Execute(&body, d); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(body.String(), d.VerifierVersion) || !strings.Contains(body.String(), d.VerifierRevision) {
				t.Fatal("print evidence omitted the retained verifier")
			}
		}
	}
}

func testEvidenceEndpoint(t *testing.T, ctx context.Context, store *Store, project Project) {
	t.Helper()
	j, report := evidenceFixture()
	j.ID = NewID()
	report.RunID = j.ID
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := json.Marshal(project.Config)
	if _, err = store.DB.ExecContext(ctx, `INSERT INTO jobs(id,project_id,project_name,backup_pattern,config,status,trigger,attempt_count,report) VALUES($1,$2,$3,'fixture.dump',$4,'passed','manual',2,$5)`, j.ID, project.ID, j.ProjectName, cfg, raw); err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB.ExecContext(ctx, `INSERT INTO job_attempts(job_id,number,status,report) VALUES($1,1,'failed',$2)`, j.ID, raw); err != nil {
		t.Fatal(err)
	}
	api := API{Store: store, AllowedHosts: []string{"localhost:7474"}, CSRFToken: NewID()}
	handler := api.Handler()
	base := "/api/jobs/" + j.ID + "/evidence.html"
	response := requestJSON(t, handler, "GET", base+"?lang=tr&redact=1&attempt=1", nil, "")
	if response.Code != 200 || response.Header().Get("Content-Type") != "text/html; charset=utf-8" || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Header().Get("Content-Security-Policy"), "script-src 'none'") {
		t.Fatal("printable export boundary failed")
	}
	if strings.Contains(response.Body.String(), "private-bucket") || !strings.Contains(response.Body.String(), "Başarısız") {
		t.Fatal("selected attempt/redaction missing")
	}
	for _, query := range []string{"lang=fr", "redact=true", "attempt=0", "attempt=invalid"} {
		if r := requestJSON(t, handler, "GET", base+"?"+query, nil, ""); r.Code != 400 {
			t.Fatal("invalid export option accepted")
		}
	}
	if r := requestJSON(t, handler, "GET", base+"?attempt=3", nil, ""); r.Code != 409 {
		t.Fatal("missing historical report looked available")
	}
}
