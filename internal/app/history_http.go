package app

import (
	"bytes"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var ErrExportTooLarge = errors.New("history export exceeds 10000 runs; narrow the filters")

func (a *API) history(w http.ResponseWriter, r *http.Request) {
	f, err := parseHistoryFilter(r.URL.Query())
	if err != nil {
		jsonError(w, 400, ErrHistoryFilter.Error())
		return
	}
	export := strings.HasSuffix(r.URL.Path, "/export.csv")
	if export && f.Before != nil {
		jsonError(w, 400, "history exports do not accept a page cursor")
		return
	}
	page, err := a.Store.History(r.Context(), f, export)
	if errors.Is(err, ErrExportTooLarge) {
		jsonError(w, 422, ErrExportTooLarge.Error())
		return
	}
	if err != nil {
		apiError(w, err)
		return
	}
	if !export {
		jsonResponse(w, 200, page)
		return
	}
	// Buffer the bounded export before sending headers: DB/encoding errors must
	// never leave a truncated download that looks like a complete report.
	var body bytes.Buffer
	c := csv.NewWriter(&body)
	c.UseCRLF = true
	_ = c.Write([]string{"run_id", "project_id", "project_name", "status", "trigger", "queued_at_utc", "started_at_utc", "finished_at_utc", "attempts", "latest_attempt_duration_ms", "archive_name", "archive_bytes", "archive_sha256", "cleanup_status", "error_code", "notification_status"})
	for _, j := range page.Items {
		_ = c.Write([]string{j.ID, j.ProjectID, csvText(j.ProjectName), j.Status, j.Trigger, csvTime(&j.QueuedAt), csvTime(j.StartedAt), csvTime(j.FinishedAt), strconv.Itoa(j.AttemptCount), csvNumber(j.DurationMS), csvText(j.BackupName), csvNumber(j.BackupBytes), j.BackupSHA256, j.CleanupStatus, j.ErrorCode, j.NotificationStatus})
	}
	c.Flush()
	if c.Error() != nil {
		jsonError(w, 500, "history export failed")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="restivra-history.csv"`)
	w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
	w.WriteHeader(200)
	_, _ = w.Write(body.Bytes())
}

// CSV quoting alone does not prevent spreadsheet formula execution. Prefix
// potentially executable user-controlled text, including leading whitespace.
func csvText(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") || strings.HasPrefix(value, "\n") || (trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0]))) {
		return "'" + value
	}
	return value
}

func csvTime(at *time.Time) string {
	if at == nil {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}

func csvNumber(n *int64) string {
	if n == nil {
		return ""
	}
	return strconv.FormatInt(*n, 10)
}
