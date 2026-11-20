package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAuthProtectsDataAndRequiresSameOriginCSRF(t *testing.T) {
	secret := strings.Repeat("test-token-", 5)
	auth, err := NewAuth(secret)
	if err != nil {
		t.Fatal(err)
	}
	api := API{Auth: auth, AllowedHosts: []string{"localhost:7474"}, CSRFToken: NewID()}
	handler := api.Handler()
	for _, path := range []string{"/api/history", "/api/history/export.csv", "/api/projects?archived=true", "/api/sources", "/api/jobs/" + NewID() + "/evidence.html"} {
		if response := requestJSON(t, handler, "GET", path, nil, ""); response.Code != 401 {
			t.Fatal("unauthenticated history/archive access accepted")
		}
	}
	req := httptest.NewRequest("GET", "http://localhost:7474/api/session", nil)
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, req)
	if out.Code != 401 {
		t.Fatal("unauthenticated API data was exposed")
	}
	boot := requestJSON(t, handler, "GET", "/api/auth", nil, "")
	if boot.Code != 200 || strings.Contains(boot.Body.String(), secret) {
		t.Fatal("auth bootstrap exposed a secret")
	}
	var state struct {
		Enabled, Authenticated bool
		CSRFToken              string `json:"csrf_token"`
	}
	if err = json.Unmarshal(boot.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if !state.Enabled || state.Authenticated || state.CSRFToken == "" {
		t.Fatal("bad authentication bootstrap")
	}
	if response := requestJSON(t, handler, "POST", "/api/auth/login", map[string]string{"token": secret}, ""); response.Code != 403 {
		t.Fatal("login skipped CSRF")
	}
	cross := httptest.NewRequest("POST", "http://localhost:7474/api/auth/login", strings.NewReader(`{"token":"`+secret+`"}`))
	cross.Header.Set("Origin", "https://attacker.example")
	cross.Header.Set("Content-Type", "application/json")
	cross.Header.Set("X-CSRF-Token", api.CSRFToken)
	crossOut := httptest.NewRecorder()
	handler.ServeHTTP(crossOut, cross)
	if crossOut.Code != 403 {
		t.Fatal("cross-origin login accepted")
	}
	wrong := requestJSON(t, handler, "POST", "/api/auth/login", map[string]string{"token": "wrong"}, api.CSRFToken)
	if wrong.Code != 401 {
		t.Fatal("wrong access token accepted")
	}
	login := requestJSON(t, handler, "POST", "/api/auth/login", map[string]string{"token": secret}, api.CSRFToken)
	if login.Code != 200 {
		t.Fatalf("login failed: %s", login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("session cookie missing")
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" {
		t.Fatal("weak cookie attributes")
	}
	if strings.Contains(cookie.Value, secret) {
		t.Fatal("session cookie exposed access token")
	}
	authenticated := httptest.NewRequest("GET", "http://localhost:7474/api/session", nil)
	authenticated.AddCookie(cookie)
	data := httptest.NewRecorder()
	handler.ServeHTTP(data, authenticated)
	if data.Code != 200 {
		t.Fatal("authenticated request denied")
	}
	tampered := httptest.NewRequest("GET", "http://localhost:7474/api/session", nil)
	bad := *cookie
	bad.Value += "x"
	tampered.AddCookie(&bad)
	badOut := httptest.NewRecorder()
	handler.ServeHTTP(badOut, tampered)
	if badOut.Code != 401 {
		t.Fatal("tampered cookie accepted")
	}
	auth.now = func() time.Time { return time.Now().Add(9 * time.Hour) }
	expired := httptest.NewRecorder()
	handler.ServeHTTP(expired, authenticated)
	if expired.Code != 401 {
		t.Fatal("expired cookie accepted")
	}
	auth.now = time.Now
	rotated, _ := NewAuth(strings.Repeat("different-token-", 4))
	if rotated.authenticated(authenticated) {
		t.Fatal("token rotation failed to revoke sessions")
	}
	logout := httptest.NewRequest("POST", "http://localhost:7474/api/auth/logout", strings.NewReader("{}"))
	logout.Header.Set("X-CSRF-Token", api.CSRFToken)
	logout.Header.Set("Content-Type", "application/json")
	logout.AddCookie(cookie)
	loggedOut := httptest.NewRecorder()
	handler.ServeHTTP(loggedOut, logout)
	if loggedOut.Code != 200 || loggedOut.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not expire cookie")
	}
}

func TestLoginRateLimitAndDisabledLegacyMode(t *testing.T) {
	auth, _ := NewAuth(strings.Repeat("x", 64))
	api := API{Auth: auth, AllowedHosts: []string{"localhost:7474"}, CSRFToken: NewID()}
	handler := api.Handler()
	for i := 0; i < 10; i++ {
		if out := requestJSON(t, handler, "POST", "/api/auth/login", map[string]string{"token": "wrong"}, api.CSRFToken); out.Code != 401 {
			t.Fatalf("attempt %d: %d", i, out.Code)
		}
	}
	if out := requestJSON(t, handler, "POST", "/api/auth/login", map[string]string{"token": strings.Repeat("x", 64)}, api.CSRFToken); out.Code != 429 {
		t.Fatal("rate limit missing")
	}
	auth.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	if out := requestJSON(t, handler, "POST", "/api/auth/login", map[string]string{"token": strings.Repeat("x", 64)}, api.CSRFToken); out.Code != 200 {
		t.Fatal("rate limit did not expire")
	}
	if _, err := NewAuth("too-short"); err == nil {
		t.Fatal("weak auth configuration accepted")
	}
	legacy := API{AllowedHosts: []string{"localhost:7474"}}
	out := requestJSON(t, legacy.Handler(), "GET", "/api/session", nil, "")
	if out.Code != 200 {
		t.Fatal("legacy local-only mode changed")
	}
}

func TestConcurrentLoginLimitsAndSecureCookie(t *testing.T) {
	secret := strings.Repeat("parallel-test-", 4)
	auth, _ := NewAuth(secret)
	api := API{Auth: auth, AllowedHosts: []string{"localhost:7474"}, CSRFToken: NewID()}
	handler := api.Handler()
	results := make(chan int, 20)
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			r := httptest.NewRequest("POST", "https://localhost:7474/api/auth/login", strings.NewReader(`{"token":"`+secret+`"}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-CSRF-Token", api.CSRFToken)
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, r)
			if out.Code == 200 {
				cookies := out.Result().Cookies()
				if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly {
					t.Error("HTTPS login did not issue a secure HttpOnly cookie")
				}
			}
			results <- out.Code
		}()
	}
	group.Wait()
	close(results)
	passed, limited := 0, 0
	for code := range results {
		switch code {
		case 200:
			passed++
		case 429:
			limited++
		default:
			t.Errorf("unexpected login status %d", code)
		}
	}
	if passed != 10 || limited != 10 {
		t.Fatalf("concurrent peer limit: %d accepted, %d limited", passed, limited)
	}
	for _, path := range []string{"/api/projects", "/api/jobs", "/api/jobs/" + NewID() + "/report", "/api/health"} {
		out := requestJSON(t, handler, "GET", path, nil, "")
		if out.Code != 401 {
			t.Errorf("unauthenticated record access %s: %d", path, out.Code)
		}
		if out.Header().Get("Cache-Control") != "no-store" {
			t.Error("API response may be cached")
		}
	}
}
