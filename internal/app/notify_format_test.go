package app

import (
	"encoding/json"
	"strings"
	"testing"
)

const failedReport = `{"error_code":"checks_failed","checks":[{"name":"Patients restored","status":"passed"},{"name":"Appointments from the last 24 hours","status":"failed"},{"name":"Skipped","status":"not_run"}]}`

func TestNoticeListsOnlyChecksThatDidNotPass(t *testing.T) {
	n := noticeFromReport("abc", "Clinic", "failed", failedReport)
	if n.ErrorCode != "checks_failed" || len(n.FailedChecks) != 1 || n.FailedChecks[0] != "Appointments from the last 24 hours" {
		t.Fatalf("unexpected notice %+v", n)
	}
	want := "Restivra: Clinic failed (run abc). Did not pass: Appointments from the last 24 hours"
	if got := n.summary(); got != want {
		t.Fatalf("summary %q, want %q", got, want)
	}
	for _, bad := range []string{"", "{", "null"} {
		m := noticeFromReport("x", "P", "error", bad)
		if m.summary() != "Restivra: P error (run x)" || m.FailedChecks == nil {
			t.Fatalf("report %q gave %+v", bad, m)
		}
	}
	e := noticeFromReport("x", "P", "error", `{"error_code":"restore_failed"}`)
	if e.summary() != "Restivra: P error (run x), restore_failed" {
		t.Fatalf("error summary %q", e.summary())
	}
}

func TestNoticeBoundsLongInput(t *testing.T) {
	n := failureNotice{JobID: "j", Project: strings.Repeat("p", 300), Status: "failed"}
	for i := 0; i < 8; i++ {
		n.FailedChecks = append(n.FailedChecks, strings.Repeat("c", 200))
	}
	s := n.summary()
	if !strings.Contains(s, "+3 more") || strings.Count(s, "…") != 6 || len([]rune(s)) > 1000 {
		t.Fatalf("unbounded summary (%d runes): %s", len([]rune(s)), s)
	}
}

func TestWebhookClassification(t *testing.T) {
	cases := map[string]webhookKind{
		"https://hooks.slack.com/services/T/B/x":       webhookSlack,
		"https://discord.com/api/webhooks/1/abc":       webhookDiscord,
		"https://discordapp.com/api/webhooks/1/abc":    webhookDiscord,
		"https://discord.com/channels/1":               webhookGeneric,
		"https://hooks.slack.com.example.org/services": webhookGeneric,
		"https://example.com/hook":                     webhookGeneric,
	}
	for raw, want := range cases {
		if got := classifyWebhook(raw); got != want {
			t.Errorf("%s: got %d want %d", raw, got, want)
		}
	}
}

func TestChatPayloadsNeutraliseMentions(t *testing.T) {
	n := failureNotice{JobID: "j", Project: "<!channel> @everyone & co", Status: "failed", FailedChecks: []string{}}
	var slack map[string]any
	raw, _ := webhookPayload("https://hooks.slack.com/services/T/B/x", n)
	if json.Unmarshal(raw, &slack) != nil || len(slack) != 1 {
		t.Fatalf("slack payload %s", raw)
	}
	if text := slack["text"].(string); strings.Contains(text, "<!channel>") || !strings.Contains(text, "&lt;!channel&gt;") || !strings.Contains(text, "&amp;") {
		t.Fatalf("slack text not escaped: %s", text)
	}
	var discord struct {
		Content  string              `json:"content"`
		Mentions map[string][]string `json:"allowed_mentions"`
	}
	raw, _ = webhookPayload("https://discord.com/api/webhooks/1/abc", n)
	if json.Unmarshal(raw, &discord) != nil || discord.Content == "" || discord.Mentions["parse"] == nil || len(discord.Mentions["parse"]) != 0 {
		t.Fatalf("discord payload %s", raw)
	}
}

func TestGenericPayloadKeepsEventFields(t *testing.T) {
	raw, _ := webhookPayload("https://example.com/hook", noticeFromReport("abc", "Clinic", "failed", failedReport))
	var p map[string]any
	if json.Unmarshal(raw, &p) != nil {
		t.Fatal(string(raw))
	}
	for _, key := range []string{"event", "job_id", "project", "status", "text", "error_code", "failed_checks"} {
		if _, ok := p[key]; !ok {
			t.Errorf("missing %s in %s", key, raw)
		}
	}
	if p["event"] != "restivra.verification.failed" || p["job_id"] != "abc" {
		t.Fatalf("event fields changed: %s", raw)
	}
}
