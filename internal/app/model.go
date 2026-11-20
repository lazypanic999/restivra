package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/lazypanic999/restivra/internal/verify"
)

var ErrConflict = errors.New("project has an active job")
var ErrNotFound = errors.New("not found")

type Project struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	BackupPattern string        `json:"backup_pattern"`
	Config        verify.Config `json:"config"`
	ScheduleTime  string        `json:"schedule_time"`
	Timezone      string        `json:"timezone"`
	NextRunAt     *time.Time    `json:"next_run_at"`
	CreatedAt     time.Time     `json:"created_at"`
	LastJob       *Job          `json:"last_job,omitempty"`
	Archived      bool          `json:"archived"`
}

type Job struct {
	ID                   string         `json:"id"`
	ProjectID            string         `json:"project_id"`
	ProjectName          string         `json:"project_name"`
	BackupPattern        string         `json:"backup_pattern"`
	Config               verify.Config  `json:"config"`
	Status               string         `json:"status"`
	Trigger              string         `json:"trigger"`
	QueuedAt             time.Time      `json:"queued_at"`
	StartedAt            *time.Time     `json:"started_at"`
	FinishedAt           *time.Time     `json:"finished_at"`
	HeartbeatAt          *time.Time     `json:"heartbeat_at"`
	CancelRequested      bool           `json:"cancel_requested"`
	Report               *verify.Report `json:"report,omitempty"`
	Error                string         `json:"error,omitempty"`
	NotificationStatus   string         `json:"notification_status"`
	NotificationAttempts int            `json:"notification_attempts"`
	AttemptCount         int            `json:"attempt_count"`
	RetryAt              *time.Time     `json:"retry_at,omitempty"`
	Attempts             []Attempt      `json:"attempts,omitempty"`
}

type Attempt struct {
	Number     int            `json:"number"`
	Status     string         `json:"status"`
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	FinishedAt time.Time      `json:"finished_at"`
	Report     *verify.Report `json:"report,omitempty"`
	Error      string         `json:"error,omitempty"`
}

func NewID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic("secure random unavailable")
	}
	return hex.EncodeToString(id[:])
}

func ValidID(id string) bool {
	b, err := hex.DecodeString(id)
	return err == nil && len(b) == 16 && strings.ToLower(id) == id
}

func (p *Project) Validate(now time.Time) error {
	p.Name = strings.TrimSpace(p.Name)
	if len(p.Name) < 1 || len(p.Name) > 120 {
		return errors.New("proje adı 1–120 bayt olmalı")
	}
	p.BackupPattern = strings.TrimSpace(p.BackupPattern)
	if !filepath.IsLocal(p.BackupPattern) || len(p.BackupPattern) > 512 || strings.Contains(p.BackupPattern, "\\") || strings.Contains(p.BackupPattern, ":") {
		return errors.New("yedek yolu, yedek köküne göre bağıl bir yol olmalı; örnek: musteri/*.dump")
	}
	for _, part := range strings.Split(p.BackupPattern, "/") {
		if part == ".." {
			return errors.New("üst klasörlere erişim yasak")
		}
	}
	if _, err := filepath.Match(p.BackupPattern, ""); err != nil {
		return errors.New("geçersiz dosya eşleşmesi")
	}
	if err := p.Config.Validate(); err != nil {
		return err
	}
	if p.Timezone == "" {
		p.Timezone = "Europe/Istanbul"
	}
	if _, err := time.LoadLocation(p.Timezone); err != nil {
		return errors.New("geçersiz saat dilimi")
	}
	next, err := NextRun(p.ScheduleTime, p.Timezone, now)
	if err != nil {
		return err
	}
	p.NextRunAt = next
	return nil
}

func NextRun(clock, zone string, now time.Time) (*time.Time, error) {
	if clock == "" {
		return nil, nil
	}
	parsed, err := time.Parse("15:04", clock)
	if err != nil || parsed.Format("15:04") != clock {
		return nil, errors.New("zamanlama HH:MM biçiminde olmalı")
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, err
	}
	local := now.In(loc)
	for offset := 0; offset < 4; offset++ {
		day := time.Date(local.Year(), local.Month(), local.Day()+offset, 12, 0, 0, 0, loc)
		candidate := time.Date(day.Year(), day.Month(), day.Day(), parsed.Hour(), parsed.Minute(), 0, 0, loc)
		if candidate.Format("15:04") == clock && candidate.After(now) {
			utc := candidate.UTC()
			return &utc, nil
		}
	}
	return nil, errors.New("sonraki çalışma saati hesaplanamadı")
}
