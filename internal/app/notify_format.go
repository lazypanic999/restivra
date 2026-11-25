package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

const maxListedChecks = 5

type failureNotice struct {
	JobID, Project, Status, ErrorCode string
	FailedChecks                      []string
}

// noticeFromReport extracts the error code and the names of checks that did
// not pass. A missing or malformed report still produces a notice.
func noticeFromReport(id, project, status, report string) failureNotice {
	n := failureNotice{JobID: id, Project: project, Status: status, FailedChecks: []string{}}
	var r struct {
		ErrorCode string `json:"error_code"`
		Checks    []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if report == "" || json.Unmarshal([]byte(report), &r) != nil {
		return n
	}
	n.ErrorCode = r.ErrorCode
	for _, c := range r.Checks {
		if c.Status != "passed" && c.Status != "not_run" && c.Status != "" {
			n.FailedChecks = append(n.FailedChecks, c.Name)
		}
	}
	return n
}

func clip(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}

func (n failureNotice) summary() string {
	line := fmt.Sprintf("Restivra: %s %s (run %s)", clip(n.Project, 120), n.Status, n.JobID)
	if len(n.FailedChecks) == 0 {
		if n.ErrorCode != "" {
			line += ", " + n.ErrorCode
		}
		return line
	}
	names := make([]string, 0, maxListedChecks)
	for i, name := range n.FailedChecks {
		if i == maxListedChecks {
			names = append(names, fmt.Sprintf("+%d more", len(n.FailedChecks)-maxListedChecks))
			break
		}
		names = append(names, clip(name, 80))
	}
	return line + ". Did not pass: " + strings.Join(names, "; ")
}

type webhookKind int

const (
	webhookGeneric webhookKind = iota
	webhookSlack
	webhookDiscord
)

func classifyWebhook(raw string) webhookKind {
	u, err := url.Parse(raw)
	if err != nil {
		return webhookGeneric
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "hooks.slack.com":
		return webhookSlack
	case (host == "discord.com" || host == "discordapp.com" || host == "ptb.discord.com" || host == "canary.discord.com") &&
		strings.HasPrefix(u.Path, "/api/webhooks/"):
		return webhookDiscord
	}
	return webhookGeneric
}

// Slack treats <, > and & as control characters (<!channel> pings everyone).
var slackEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// webhookPayload keeps the generic event fields for every receiver and adds
// the field each chat service renders. Project and check names are user
// input, so mentions are neutralised before they reach a chat channel.
func webhookPayload(raw string, n failureNotice) ([]byte, error) {
	text := n.summary()
	switch classifyWebhook(raw) {
	case webhookSlack:
		return json.Marshal(map[string]any{"text": slackEscaper.Replace(text)})
	case webhookDiscord:
		return json.Marshal(map[string]any{
			"content":          clip(text, 1900),
			"allowed_mentions": map[string]any{"parse": []string{}},
		})
	}
	return json.Marshal(map[string]any{
		"event": "restivra.verification.failed", "job_id": n.JobID, "project": n.Project, "status": n.Status,
		"error_code": n.ErrorCode, "failed_checks": n.FailedChecks, "text": text,
	})
}
