package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/lazypanic999/restivra/internal/source"
)

func (a *API) sources(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	profiles := []source.PublicProfile{}
	var raw []byte
	var beat time.Time
	err := a.Store.DB.QueryRowContext(ctx, "SELECT source_profiles,heartbeat_at FROM workers WHERE instance=$1", a.Instance).Scan(&raw, &beat)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		jsonError(w, 503, "worker source state unavailable")
		return
	}
	if err == nil && json.Unmarshal(raw, &profiles) != nil {
		jsonError(w, 503, "worker source state unavailable")
		return
	}
	jsonResponse(w, 200, map[string]any{"profiles": profiles, "worker_online": err == nil && time.Since(beat) < 30*time.Second})
}
