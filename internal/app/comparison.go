package app

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"
)

type comparisonRun struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	QueuedAt   time.Time `json:"queued_at"`
	SHA256     string    `json:"sha256"`
	Bytes      int64     `json:"bytes"`
	DurationMS int64     `json:"duration_ms"`
}
type comparisonCheck struct {
	Name   string `json:"name"`
	Before string `json:"before"`
	After  string `json:"after"`
	Change string `json:"change"`
}
type runComparison struct {
	Current         comparisonRun     `json:"current"`
	Baseline        *comparisonRun    `json:"baseline"`
	Content         string            `json:"content"`
	SourceChanged   bool              `json:"source_changed"`
	LimitsChanged   bool              `json:"limits_changed"`
	ImageChanged    bool              `json:"image_changed"`
	VerifierChanged bool              `json:"verifier_changed"`
	Restore         comparisonCheck   `json:"restore"`
	Cleanup         comparisonCheck   `json:"cleanup"`
	Checks          []comparisonCheck `json:"checks"`
}

func outcomeChange(before, after string) string {
	if before == "not_run" || after == "not_run" || before == "" || after == "" {
		return "not_comparable"
	}
	if before == after {
		return "unchanged"
	}
	if before == "passed" && (after == "failed" || after == "error") {
		return "regressed"
	}
	if after == "passed" && (before == "failed" || before == "error") {
		return "improved"
	}
	return "changed"
}

// Compare retained outcomes using immutable job settings. SQL is used only for
// equality in memory; it never enters the response. Changed definitions must not
// masquerade as a data regression, nor may an unexecuted check count as a pass.
func compareRuns(current Job, baseline *Job) runComparison {
	meta := func(j Job) comparisonRun {
		return comparisonRun{j.ID, j.Status, j.QueuedAt, j.Report.Backup.SHA256, j.Report.Backup.Bytes, j.Report.DurationMS}
	}
	result := runComparison{Current: meta(current), Content: "unknown", Checks: []comparisonCheck{}}
	if baseline == nil {
		return result
	}
	b := meta(*baseline)
	result.Baseline = &b
	a, old := current.Report, baseline.Report
	if a.Backup.SHA256 != "" && old.Backup.SHA256 != "" {
		result.Content = "different"
		if a.Backup.SHA256 == old.Backup.SHA256 {
			result.Content = "same"
		}
	}
	result.SourceChanged = current.Source != baseline.Source || current.BackupPattern != baseline.BackupPattern
	x, y := current.Config, baseline.Config
	result.LimitsChanged = x.TimeoutSeconds != y.TimeoutSeconds || x.MaxBackupBytes != y.MaxBackupBytes || x.MemoryMB != y.MemoryMB || x.DataMB != y.DataMB
	result.ImageChanged = a.Image != old.Image || (a.ImageID != "" && old.ImageID != "" && a.ImageID != old.ImageID)
	if a.ApplicationBuild != nil && old.ApplicationBuild != nil {
		result.VerifierChanged = *a.ApplicationBuild != *old.ApplicationBuild
	}
	result.Restore = comparisonCheck{"restore", old.RestoreStatus, a.RestoreStatus, outcomeChange(old.RestoreStatus, a.RestoreStatus)}
	result.Cleanup = comparisonCheck{"cleanup", old.CleanupStatus, a.CleanupStatus, outcomeChange(old.CleanupStatus, a.CleanupStatus)}
	type definition struct {
		query, status string
		configured    bool
	}
	index := func(j Job) (map[string]definition, []string) {
		items := map[string]definition{}
		names := []string{}
		for _, check := range j.Config.Checks {
			items[check.Name] = definition{strings.TrimSpace(check.SQL), "not_run", true}
			names = append(names, check.Name)
		}
		for _, check := range j.Report.Checks {
			d, exists := items[check.Name]
			if !exists {
				names = append(names, check.Name)
			}
			d.status = check.Status
			items[check.Name] = d
		}
		return items, names
	}
	now, names := index(current)
	previous, previousNames := index(*baseline)
	seen := map[string]bool{}
	for _, name := range append(names, previousNames...) {
		if seen[name] {
			continue
		}
		seen[name] = true
		n, nExists := now[name]
		p, pExists := previous[name]
		row := comparisonCheck{Name: name, Before: p.status, After: n.status}
		switch {
		case !pExists:
			row.Before = "not_configured"
			row.Change = "added"
		case !nExists:
			row.After = "not_configured"
			row.Change = "removed"
		case !n.configured || !p.configured || n.query == "" || p.query == "":
			row.Change = "not_comparable"
		case n.query != p.query:
			row.Change = "definition_changed"
		default:
			row.Change = outcomeChange(p.status, n.status)
		}
		result.Checks = append(result.Checks, row)
	}
	return result
}

func (a *API) comparison(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	baselineID := r.URL.Query().Get("baseline")
	if !ValidID(id) || (baselineID != "" && (!ValidID(baselineID) || baselineID == id)) {
		jsonError(w, 400, "invalid comparison selection")
		return
	}
	current, err := a.Store.Job(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	if current.Report == nil || current.Status == "running" || current.Status == "queued" {
		jsonError(w, 409, "completed report required for comparison")
		return
	}
	var previous Job
	if baselineID != "" {
		previous, err = a.Store.Job(r.Context(), baselineID)
	} else {
		previous, err = scanJob(a.Store.DB.QueryRowContext(r.Context(), "SELECT "+jobCols+" FROM jobs WHERE project_id=$1 AND (queued_at,id)<($2,$3) AND report IS NOT NULL AND status NOT IN ('running','queued') ORDER BY queued_at DESC,id DESC LIMIT 1", current.ProjectID, current.QueuedAt, current.ID))
		if errors.Is(err, sql.ErrNoRows) {
			jsonResponse(w, 200, compareRuns(current, nil))
			return
		}
	}
	if err != nil {
		apiError(w, err)
		return
	}
	if previous.ProjectID != current.ProjectID || !previous.QueuedAt.Before(current.QueuedAt) && !(previous.QueuedAt.Equal(current.QueuedAt) && previous.ID < current.ID) {
		jsonError(w, 400, "baseline must be an earlier run of the same project")
		return
	}
	if previous.Report == nil || previous.Status == "running" || previous.Status == "queued" {
		jsonError(w, 409, "completed baseline report required")
		return
	}
	jsonResponse(w, 200, compareRuns(current, &previous))
}
