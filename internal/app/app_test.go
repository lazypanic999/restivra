package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lazypanic999/restivra/internal/verify"
)

func TestDailyScheduleRespectsTimezoneAndSkipsMissedDays(t *testing.T) {
	now := time.Date(2026, 11, 17, 23, 0, 0, 0, time.UTC)
	next, err := NextRun("03:00", "Europe/Istanbul", now)
	if err != nil || !next.Equal(time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("wrong next run: %v %v", next, err)
	}
	next, err = NextRun("03:00", "Europe/Istanbul", *next)
	if err != nil || !next.Equal(time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("schedule repeats same instant: %v %v", next, err)
	}
	spring := time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC)
	next, err = NextRun("02:30", "Europe/Berlin", spring)
	if err != nil || next.In(mustZone(t, "Europe/Berlin")).Day() != 30 {
		t.Fatalf("nonexistent DST time should skip day: %v %v", next, err)
	}
}
func mustZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestProjectRejectsTraversalAndInvalidSchedule(t *testing.T) {
	for _, pattern := range []string{"../secret.dump", "a/../../secret.dump", "/etc/passwd", "C:/secrets.dump", "a\\b.dump", "["} {
		p := Project{Name: "Example", BackupPattern: pattern, Config: verify.DefaultConfig(), Timezone: "Europe/Istanbul"}
		if p.Validate(time.Now()) == nil {
			t.Errorf("accepted invalid pattern %q", pattern)
		}
	}
	p := Project{Name: "Example", BackupPattern: "customer/*.dump", Config: verify.DefaultConfig(), Timezone: "Europe/Istanbul", ScheduleTime: "25:00"}
	if p.Validate(time.Now()) == nil {
		t.Fatal("accepted invalid clock")
	}
}

func TestLatestBackupAndContainedOpen(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "customer"), 0700); err != nil {
		t.Fatal(err)
	}
	older := filepath.Join(dir, "customer", "old.dump")
	newer := filepath.Join(dir, "customer", "new.dump")
	for _, path := range []string{older, newer} {
		if err := os.WriteFile(path, []byte("PGDMPtest"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	before := time.Now().Add(-time.Hour)
	if err := os.Chtimes(older, before, before); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	selected, err := latestBackup(root, "customer/*.dump")
	if err != nil || selected != "customer/new.dump" {
		t.Fatalf("wrong selection: %s %v", selected, err)
	}
	if _, err = root.Open("../secret.dump"); err == nil {
		t.Fatal("root allowed traversal")
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.dump"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "escape")); err == nil {
		if f, err := root.Open("escape/secret.dump"); err == nil {
			f.Close()
			t.Fatal("root allowed symlink escape")
		}
	}
}

func TestBrowserRequestBoundaries(t *testing.T) {
	api := API{AllowedHosts: []string{"localhost:7474"}, CSRFToken: strings.Repeat("a", 64)}
	handler := api.Handler()
	for _, tc := range []struct {
		name, method, host, origin, site, token string
		status                                  int
	}{
		{"session", "GET", "localhost:7474", "", "", "", 200},
		{"rebinding", "GET", "attacker.example", "", "", "", 403},
		{"cross origin", "GET", "localhost:7474", "https://evil.example", "", "", 403},
		{"fetch metadata", "GET", "localhost:7474", "", "cross-site", "", 403},
		{"missing csrf", "POST", "localhost:7474", "http://localhost:7474", "same-origin", "", 403},
		{"bad csrf", "POST", "localhost:7474", "", "", "wrong", 403},
		{"valid csrf unknown endpoint", "POST", "localhost:7474", "", "", strings.Repeat("a", 64), 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://localhost:7474/api/"+map[bool]string{true: "session", false: "unknown"}[tc.method == "GET"], strings.NewReader("{}"))
			req.Host = tc.host
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Sec-Fetch-Site", tc.site)
			req.Header.Set("X-CSRF-Token", tc.token)
			req.Header.Set("Content-Type", "application/json")
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			if out.Code != tc.status {
				t.Fatalf("got %d: %s", out.Code, out.Body.String())
			}
			if out.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("missing defensive headers")
			}
		})
	}
}

func TestWebhookRejectsPrivateAndSpecialNetworks(t *testing.T) {
	for _, raw := range []string{"http://example.com/hook", "https://127.0.0.1/hook", "https://10.0.0.1/hook", "https://[::1]/hook", "https://user:pass@example.com/hook", "https://example.com:8443/hook", "https://169.254.169.254/hook", "https://100.64.1.1/hook"} {
		if ValidateWebhook(raw) == nil {
			t.Errorf("accepted unsafe webhook %s", raw)
		}
	}
	for _, raw := range []string{"127.0.0.1", "10.1.1.1", "169.254.169.254", "100.64.0.1", "192.0.2.1", "::1", "::ffff:127.0.0.1", "2001:db8::1", "64:ff9b::7f00:1"} {
		if publicAddress(netip.MustParseAddr(raw)) {
			t.Errorf("accepted unsafe IP %s", raw)
		}
	}
	if ValidateWebhook("https://hooks.example.com/abc") != nil {
		t.Fatal("rejected valid https URL")
	}
}

func TestSnapshotCleanupRejectsUnexpectedFiles(t *testing.T) {
	dir := t.TempDir()
	id := NewID()
	run := filepath.Join(dir, id)
	if err := os.Mkdir(run, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run, "important.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if cleanSnapshotDir(dir, id) == nil {
		t.Fatal("removed directory containing unexpected files")
	}
	if _, err := os.Stat(filepath.Join(run, "important.txt")); err != nil {
		t.Fatal("unrelated file was removed")
	}
}

// Reuse the handler's real session token in a local client without exposing it.
func requestJSON(t *testing.T, handler http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(method, "http://localhost:7474"+path, strings.NewReader(string(raw)))
	req.Host = "localhost:7474"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", token)
	req = req.WithContext(context.Background())
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, req)
	return out
}
