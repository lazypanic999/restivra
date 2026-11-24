package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrProjectFilter = errors.New("invalid project filters or page cursor")

type projectCursor struct {
	At       time.Time `json:"at"`
	ID       string    `json:"id"`
	Archived bool      `json:"archived"`
	Search   string    `json:"q"`
}

type ProjectFilter struct {
	Search   string
	Archived bool
	Limit    int
	Before   *projectCursor
}

type ProjectPage struct {
	Items         []Project `json:"items"`
	Total         int64     `json:"total"`
	ActiveTotal   int64     `json:"active_total"`
	ArchivedTotal int64     `json:"archived_total"`
	NextCursor    string    `json:"next_cursor,omitempty"`
}

func parseProjectFilter(values url.Values) (ProjectFilter, error) {
	f := ProjectFilter{Search: strings.TrimSpace(values.Get("q")), Limit: 50}
	for key, entries := range values {
		if len(entries) != 1 {
			return f, ErrProjectFilter
		}
		switch key {
		case "q", "archived", "limit", "cursor":
		default:
			return f, ErrProjectFilter
		}
	}
	if len(f.Search) > 480 || strings.ContainsRune(f.Search, 0) {
		return f, ErrProjectFilter
	}
	switch values.Get("archived") {
	case "", "false":
	case "true":
		f.Archived = true
	default:
		return f, ErrProjectFilter
	}
	if raw := values.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			return f, ErrProjectFilter
		}
		f.Limit = limit
	}
	if raw := values.Get("cursor"); raw != "" {
		if len(raw) > 2048 {
			return f, ErrProjectFilter
		}
		data, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			return f, ErrProjectFilter
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		var cursor projectCursor
		if decoder.Decode(&cursor) != nil || decoder.Decode(new(any)) != io.EOF || cursor.At.IsZero() || !ValidID(cursor.ID) || cursor.Archived != f.Archived || cursor.Search != f.Search {
			return f, ErrProjectFilter
		}
		f.Before = &cursor
	}
	return f, nil
}

func (a *API) projectPage(w http.ResponseWriter, r *http.Request) {
	f, err := parseProjectFilter(r.URL.Query())
	if err != nil {
		jsonError(w, 400, ErrProjectFilter.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := a.Store.ProjectsPage(ctx, f)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, 200, result)
}

func (s *Store) ProjectsPage(ctx context.Context, f ProjectFilter) (ProjectPage, error) {
	result := ProjectPage{Items: []Project{}}
	if f.Limit < 1 || f.Limit > 100 {
		return result, ErrProjectFilter
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	// Total belongs to the entire matching view, independent of cursor position.
	err = tx.QueryRowContext(ctx, `SELECT count(*) FILTER(WHERE archived=$1 AND ($2='' OR strpos(lower(name),lower($2))>0)),count(*) FILTER(WHERE NOT archived),count(*) FILTER(WHERE archived) FROM projects`, f.Archived, f.Search).Scan(&result.Total, &result.ActiveTotal, &result.ArchivedTotal)
	if err != nil {
		return result, err
	}
	var beforeTime *time.Time
	beforeID := ""
	if f.Before != nil {
		beforeTime = &f.Before.At
		beforeID = f.Before.ID
	}
	rows, err := tx.QueryContext(ctx, `SELECT to_jsonb(p)||jsonb_build_object('last_job',latest.data)
FROM (SELECT `+projectCols+` FROM projects WHERE archived=$1 AND ($2='' OR strpos(lower(name),lower($2))>0)
 AND ($3::timestamptz IS NULL OR (created_at,id)<($3::timestamptz,$4::text)) ORDER BY created_at DESC,id DESC LIMIT $5) p
LEFT JOIN LATERAL (SELECT to_jsonb(j) AS data FROM jobs j WHERE j.project_id=p.id ORDER BY j.queued_at DESC,j.id DESC LIMIT 1) latest ON true
ORDER BY p.created_at DESC,p.id DESC`, f.Archived, f.Search, beforeTime, beforeID, f.Limit+1)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var raw []byte
		var p Project
		if err = rows.Scan(&raw); err == nil {
			err = json.Unmarshal(raw, &p)
		}
		if err != nil {
			rows.Close()
			return result, err
		}
		result.Items = append(result.Items, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if len(result.Items) > f.Limit {
		result.Items = result.Items[:f.Limit]
		last := result.Items[len(result.Items)-1]
		raw, err := json.Marshal(projectCursor{At: last.CreatedAt, ID: last.ID, Archived: f.Archived, Search: f.Search})
		if err != nil {
			return result, err
		}
		result.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}
