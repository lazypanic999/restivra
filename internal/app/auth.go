package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const sessionCookie = "restivra_session"
const sessionLifetime = 8 * time.Hour

type loginWindow struct {
	started  time.Time
	attempts int
}
type Auth struct {
	secret    []byte
	tokenHash [32]byte
	mutex     sync.Mutex
	attempts  map[string]loginWindow
	now       func() time.Time
}

// Empty configuration preserves local-only legacy installs. Fresh setups generate
// an independent random access token; it is never returned by any HTTP endpoint.
func NewAuth(token string) (*Auth, error) {
	if token == "" {
		return nil, nil
	}
	if len(token) < 32 || len(token) > 256 || strings.TrimSpace(token) != token {
		return nil, errors.New("RESTIVRA_AUTH_TOKEN must contain 32–256 bytes without surrounding whitespace")
	}
	return &Auth{secret: []byte(token), tokenHash: sha256.Sum256([]byte(token)), attempts: map[string]loginWindow{}, now: time.Now}, nil
}

func (a *Auth) authenticated(r *http.Request) bool {
	if a == nil {
		return true
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || len(cookie.Value) > 256 {
		return false
	}
	payload, signature, ok := strings.Cut(cookie.Value, ".")
	if !ok {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(payload))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return false
	}
	expiry, nonce, ok := strings.Cut(string(raw), ":")
	if !ok || !ValidID(nonce) {
		return false
	}
	deadline, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil {
		return false
	}
	return a.now().Before(time.Unix(deadline, 0))
}

func (a *Auth) issue(w http.ResponseWriter, r *http.Request) {
	expires := a.now().Add(sessionLifetime)
	payload := base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(expires.Unix(), 10) + ":" + NewID()))
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(payload))
	value := payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: value, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: int(sessionLifetime.Seconds()), Expires: expires})
}
func clearSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}

func (a *Auth) allowAttempt(peer string) bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	now := a.now()
	for key, entry := range a.attempts {
		if now.Sub(entry.started) > time.Minute {
			delete(a.attempts, key)
		}
	}
	entry := a.attempts[peer]
	if entry.started.IsZero() {
		entry.started = now
	}
	if entry.attempts >= 10 || len(a.attempts) >= 1024 {
		return false
	}
	entry.attempts++
	a.attempts[peer] = entry
	return true
}

func (a *API) authState(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, 200, map[string]any{"enabled": a.Auth != nil, "authenticated": a.Auth.authenticated(r), "csrf_token": a.CSRFToken})
}
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if a.Auth == nil {
		jsonError(w, 404, "authentication is not enabled")
		return
	}
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	if !a.Auth.allowAttempt(peer) {
		w.Header().Set("Retry-After", "60")
		jsonError(w, 429, "too many sign-in attempts; wait one minute")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var body struct {
		Token string `json:"token"`
	}
	if decodeJSON(w, r, &body) != nil {
		return
	}
	received := sha256.Sum256([]byte(body.Token))
	if subtle.ConstantTimeCompare(received[:], a.Auth.tokenHash[:]) != 1 {
		jsonError(w, 401, "invalid access token")
		return
	}
	a.Auth.issue(w, r)
	jsonResponse(w, 200, map[string]bool{"authenticated": true})
}
