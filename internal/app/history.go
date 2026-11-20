package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrHistoryFilter = errors.New("invalid history filters")

const HistoryExportLimit = 10000

type HistoryFilter struct {
	Search, Status, ProjectID string
	Since, Until              *time.Time
	Limit                     int
	Before                    *historyCursor
}

type historyCursor struct {
	Version int       `json:"v"`
	At      time.Time `json:"at"`
	ID      string    `json:"id"`
	Filter  string    `json:"filter"`
}

type HistoryItem struct {
	ID                 string     `json:"id"`
	ProjectID          string     `json:"project_id"`
	ProjectName        string     `json:"project_name"`
	Status             string     `json:"status"`
	Trigger            string     `json:"trigger"`
	QueuedAt           time.Time  `json:"queued_at"`
	StartedAt          *time.Time `json:"started_at"`
	FinishedAt         *time.Time `json:"finished_at"`
	AttemptCount       int        `json:"attempt_count"`
	RetryAt            *time.Time `json:"retry_at,omitempty"`
	DurationMS         *int64     `json:"duration_ms"`
	BackupBytes        *int64     `json:"backup_bytes"`
	BackupName         string     `json:"backup_name"`
	BackupSHA256       string     `json:"backup_sha256"`
	CleanupStatus      string     `json:"cleanup_status"`
	ErrorCode          string     `json:"error_code"`
	NotificationStatus string     `json:"notification_status"`
}

type HistorySummary struct {
	Total       int64 `json:"total"`
	Passed      int64 `json:"passed"`
	Failed      int64 `json:"failed"`
	Cancelled   int64 `json:"cancelled"`
	Interrupted int64 `json:"interrupted"`
	Queued      int64 `json:"queued"`
	Running     int64 `json:"running"`
}

type HistoryPage struct {
	Items      []HistoryItem  `json:"items"`
	Summary    HistorySummary `json:"summary"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

func parseHistoryFilter(values url.Values) (HistoryFilter, error) {
	f := HistoryFilter{Search: strings.TrimSpace(values.Get("q")), Status: values.Get("status"), ProjectID: values.Get("project_id"), Limit: 50}
	if len(f.Search) > 480 || strings.ContainsRune(f.Search, 0) || (f.ProjectID != "" && !ValidID(f.ProjectID)) {
		return f, ErrHistoryFilter
	}
	switch f.Status {
	case "", "passed", "failed", "cancelled", "interrupted", "queued", "running":
	default:
		return f, ErrHistoryFilter
	}
	for _, field := range []struct {
		key string
		out **time.Time
	}{{"since", &f.Since}, {"until", &f.Until}} {
		if value := values.Get(field.key); value != "" {
			at, err := time.Parse(time.RFC3339Nano, value)
			if err != nil || at.Year() < 1900 || at.Year() > 9999 {
				return f, ErrHistoryFilter
			}
			at = at.UTC()
			*field.out = &at
		}
	}
	if f.Since != nil && f.Until != nil && !f.Since.Before(*f.Until) {
		return f, ErrHistoryFilter
	}
	if value := values.Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			return f, ErrHistoryFilter
		}
		f.Limit = limit
	}
	if encoded := values.Get("cursor"); encoded != "" {
		if len(encoded) > 1024 {
			return f, ErrHistoryFilter
		}
		raw, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return f, ErrHistoryFilter
		}
		var cursor historyCursor
		if json.Unmarshal(raw, &cursor) != nil || cursor.Version != 1 || !ValidID(cursor.ID) || cursor.At.Year() < 1900 || cursor.At.Year() > 9999 || cursor.Filter != f.fingerprint() {
			return f, ErrHistoryFilter
		}
		f.Before = &cursor
	}
	return f, nil
}

func (f HistoryFilter) fingerprint() string {
	data, _ := json.Marshal([]any{f.Search, f.Status, f.ProjectID, f.Since, f.Until})
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// Search is literal text, including '%' and '_', not a SQL LIKE pattern.
func (f HistoryFilter) where(cursor bool) (string, []any) {
	parts := []string{"true"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		parts = append(parts, fmt.Sprintf(clause, len(args)))
	}
	if f.Search != "" {
		add("strpos(lower(project_name),lower($%d))>0", f.Search)
	}
	if f.ProjectID != "" {
		add("project_id=$%d", f.ProjectID)
	}
	if f.Status != "" {
		add("status=$%d", f.Status)
	}
	if f.Since != nil {
		add("queued_at >= $%d", *f.Since)
	}
	if f.Until != nil {
		add("queued_at < $%d", *f.Until)
	}
	if cursor && f.Before != nil {
		args = append(args, f.Before.At, f.Before.ID)
		parts = append(parts, fmt.Sprintf("(queued_at,id)<($%d,$%d)", len(args)-1, len(args)))
	}
	return strings.Join(parts, " AND "), args
}

// Lists deliberately omit SQL settings, checks and full report bodies. Detail
// and report download continue to use the existing authenticated job endpoint.
const historyCols = `id,project_id,project_name,status,trigger,queued_at,started_at,finished_at,attempt_count,retry_at,
COALESCE((report->>'duration_ms')::bigint,CASE WHEN started_at IS NOT NULL AND finished_at IS NOT NULL THEN (extract(epoch FROM (finished_at-started_at))*1000)::bigint END),
(report->'backup'->>'bytes')::bigint,COALESCE(report->'backup'->>'name',''),COALESCE(report->'backup'->>'sha256',''),COALESCE(report->>'cleanup_status',''),COALESCE(report->>'error_code',''),notification_status`

func readHistoryItems(ctx context.Context, tx *sql.Tx, f HistoryFilter, limit int, cursor bool) ([]HistoryItem, error) {
	where, args := f.where(cursor)
	args = append(args, limit)
	rows, err := tx.QueryContext(ctx, "SELECT "+historyCols+" FROM jobs WHERE "+where+fmt.Sprintf(" ORDER BY queued_at DESC,id DESC LIMIT $%d", len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []HistoryItem{}
	for rows.Next() {
		var j HistoryItem
		if err = rows.Scan(&j.ID, &j.ProjectID, &j.ProjectName, &j.Status, &j.Trigger, &j.QueuedAt, &j.StartedAt, &j.FinishedAt, &j.AttemptCount, &j.RetryAt, &j.DurationMS, &j.BackupBytes, &j.BackupName, &j.BackupSHA256, &j.CleanupStatus, &j.ErrorCode, &j.NotificationStatus); err != nil {
			return nil, err
		}
		items = append(items, j)
	}
	return items, rows.Err()
}

func (s *Store) History(ctx context.Context, f HistoryFilter, export bool) (HistoryPage, error) {
	result := HistoryPage{Items: []HistoryItem{}}
	if !export && (f.Limit < 1 || f.Limit > 100) {
		return result, ErrHistoryFilter
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	where, args := f.where(false)
	err = tx.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE status='passed'),count(*) FILTER(WHERE status='failed'),count(*) FILTER(WHERE status='cancelled'),count(*) FILTER(WHERE status='interrupted'),count(*) FILTER(WHERE status='queued'),count(*) FILTER(WHERE status='running') FROM jobs WHERE `+where, args...).Scan(&result.Summary.Total, &result.Summary.Passed, &result.Summary.Failed, &result.Summary.Cancelled, &result.Summary.Interrupted, &result.Summary.Queued, &result.Summary.Running)
	if err != nil {
		return result, err
	}
	limit := f.Limit
	if export {
		if result.Summary.Total > HistoryExportLimit {
			return result, ErrExportTooLarge
		}
		limit = HistoryExportLimit
	}
	result.Items, err = readHistoryItems(ctx, tx, f, limit+1, !export)
	if err != nil {
		return result, err
	}
	if !export && len(result.Items) > limit {
		result.Items = result.Items[:limit]
		last := result.Items[limit-1]
		raw, e := json.Marshal(historyCursor{Version: 1, At: last.QueuedAt, ID: last.ID, Filter: f.fingerprint()})
		if e != nil {
			return result, e
		}
		result.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return result, tx.Commit()
}
