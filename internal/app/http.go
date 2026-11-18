package app

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lazypanic999/restivra/internal/verify"
	"github.com/lazypanic999/restivra/internal/webui"
)

type API struct {
	Store        *Store
	AllowedHosts []string
	Instance     string
	CSRFToken    string
	Assets       http.Handler
	Auth         *Auth
}

func (a *API) Handler() http.Handler {
	if a.CSRFToken == "" {
		a.CSRFToken = NewID() + NewID()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if a.Store == nil || a.Store.DB.PingContext(ctx) != nil {
			jsonError(w, 503, "unavailable")
			return
		}
		jsonResponse(w, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /api/auth", a.authState)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		clearSession(w, r)
		jsonResponse(w, 200, map[string]bool{"authenticated": false})
	})
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{"csrf_token": a.CSRFToken, "default_config": verify.DefaultConfig(), "version": "0.1.0"})
	})
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/projects", func(w http.ResponseWriter, r *http.Request) {
		p, err := a.Store.Projects(r.Context())
		if err != nil {
			apiError(w, err)
			return
		}
		jsonResponse(w, 200, p)
	})
	mux.HandleFunc("POST /api/projects", a.saveProject)
	mux.HandleFunc("PUT /api/projects/{id}", a.saveProject)
	mux.HandleFunc("DELETE /api/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !ValidID(r.PathValue("id")) {
			jsonError(w, 400, "geçersiz proje kimliği")
			return
		}
		if err := a.Store.Archive(r.Context(), r.PathValue("id")); err != nil {
			apiError(w, err)
			return
		}
		jsonResponse(w, 200, map[string]bool{"archived": true})
	})
	mux.HandleFunc("GET /api/jobs", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("project_id")
		if id != "" && !ValidID(id) {
			jsonError(w, 400, "geçersiz proje kimliği")
			return
		}
		jobs, err := a.Store.Jobs(r.Context(), id)
		if err != nil {
			apiError(w, err)
			return
		}
		jsonResponse(w, 200, jobs)
	})
	mux.HandleFunc("POST /api/jobs", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ProjectID string `json:"project_id"`
		}
		if decodeJSON(w, r, &body) != nil {
			return
		}
		if !ValidID(body.ProjectID) {
			jsonError(w, 400, "geçersiz proje kimliği")
			return
		}
		job, err := a.Store.Enqueue(r.Context(), body.ProjectID, "manual")
		if err != nil {
			apiError(w, err)
			return
		}
		jsonResponse(w, 202, job)
	})
	mux.HandleFunc("GET /api/jobs/{id}", a.job)
	mux.HandleFunc("GET /api/jobs/{id}/report", a.job)
	mux.HandleFunc("POST /api/jobs/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		if !ValidID(r.PathValue("id")) {
			jsonError(w, 400, "geçersiz çalışma kimliği")
			return
		}
		if err := a.Store.Cancel(r.Context(), r.PathValue("id")); err != nil {
			apiError(w, err)
			return
		}
		jsonResponse(w, 202, map[string]bool{"cancel_requested": true})
	})
	mux.Handle("/api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { jsonError(w, 404, "uç nokta bulunamadı") }))
	assets := a.Assets
	if assets == nil {
		assets = webui.Handler()
	}
	mux.Handle("/", assets)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		allowed := false
		for _, host := range a.AllowedHosts {
			if r.Host == host {
				allowed = true
				break
			}
		}
		if !allowed {
			jsonError(w, 403, "host izin listesinde değil")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
			origin := r.Header.Get("Origin")
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" || (origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host) {
				jsonError(w, 403, "farklı kaynaktan erişim reddedildi")
				return
			}
			if r.Method != "GET" && r.Method != "HEAD" {
				if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(a.CSRFToken)) != 1 {
					jsonError(w, 403, "oturum belirteci geçersiz; sayfayı yenileyin")
					return
				}
				if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
					jsonError(w, 415, "application/json gerekli")
					return
				}
			}
			publicAuth := r.URL.Path == "/api/auth" || r.URL.Path == "/api/auth/login"
			if !publicAuth && !a.Auth.authenticated(r) {
				jsonError(w, 401, "authentication required")
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (a *API) saveProject(w http.ResponseWriter, r *http.Request) {
	p := Project{Config: verify.DefaultConfig(), Timezone: "Europe/Istanbul"}
	if decodeJSON(w, r, &p) != nil {
		return
	}
	create := r.Method == "POST"
	p.ID = r.PathValue("id")
	if !create && !ValidID(p.ID) {
		jsonError(w, 400, "geçersiz proje kimliği")
		return
	}
	if err := p.Validate(time.Now()); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	saved, err := a.Store.SaveProject(r.Context(), p, create)
	if err != nil {
		apiError(w, err)
		return
	}
	code := 200
	if create {
		code = 201
	}
	jsonResponse(w, code, saved)
}
func (a *API) job(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !ValidID(id) {
		jsonError(w, 400, "geçersiz çalışma kimliği")
		return
	}
	j, err := a.Store.Job(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/report") {
		report := j.Report
		suffix := ""
		if value := r.URL.Query().Get("attempt"); value != "" {
			n, e := strconv.Atoi(value)
			if e != nil || n < 1 || n > MaxAttempts {
				jsonError(w, 400, "invalid attempt number")
				return
			}
			report = nil
			for _, attempt := range j.Attempts {
				if attempt.Number == n {
					report = attempt.Report
					break
				}
			}
			suffix = "-attempt-" + strconv.Itoa(n)
		}
		if report == nil {
			jsonError(w, 409, "bu çalışma için henüz geri yükleme raporu yok")
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="restivra-`+id+suffix+`.json"`)
		jsonResponse(w, 200, report)
		return
	}
	jsonResponse(w, 200, j)
}
func (a *API) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := a.Store.DB.PingContext(ctx); err != nil {
		jsonError(w, 503, "veritabanı erişilemiyor")
		return
	}
	var beat *time.Time
	err := a.Store.DB.QueryRowContext(ctx, "SELECT heartbeat_at FROM workers WHERE instance=$1", a.Instance).Scan(&beat)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		jsonError(w, 503, "worker state unavailable")
		return
	}
	online := beat != nil && time.Since(*beat) < 30*time.Second
	jsonResponse(w, 200, map[string]any{"database": "online", "worker_online": online, "worker_heartbeat": beat, "authentication_enabled": a.Auth != nil})
}
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	err := d.Decode(dst)
	if err == nil {
		if extra := d.Decode(new(any)); extra != io.EOF {
			err = errors.New("tek JSON nesnesi gerekli")
		}
	}
	if err != nil {
		jsonError(w, 400, "geçersiz JSON veya desteklenmeyen alan")
		return err
	}
	return nil
}
func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func jsonError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}
func apiError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, 404, "kayıt bulunamadı")
	case errors.Is(err, ErrConflict):
		jsonError(w, 409, "bu projenin bekleyen veya çalışan bir testi var")
	default:
		jsonError(w, 500, "işlem tamamlanamadı")
	}
}
