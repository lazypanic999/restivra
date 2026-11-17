package verify

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

const Image = "postgres:17-bookworm"

type Check struct {
	Name string `json:"name"`
	SQL  string `json:"sql"`
}

type Config struct {
	TimeoutSeconds int     `json:"timeout_seconds"`
	MaxBackupBytes int64   `json:"max_backup_bytes"`
	MemoryMB       int     `json:"memory_mb"`
	DataMB         int     `json:"data_mb"`
	Checks         []Check `json:"checks"`
}

func DefaultConfig() Config {
	return Config{TimeoutSeconds: 180, MaxBackupBytes: 256 << 20, MemoryMB: 1536, DataMB: 512}
}

func ReadConfig(r io.Reader) (Config, error) {
	c := DefaultConfig()
	d := json.NewDecoder(io.LimitReader(r, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return c, errors.New("config must contain one JSON object")
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	if c.TimeoutSeconds < 5 || c.TimeoutSeconds > 3600 {
		return errors.New("timeout_seconds must be between 5 and 3600")
	}
	if c.MaxBackupBytes < 5 || c.MaxBackupBytes > 10<<30 {
		return errors.New("max_backup_bytes must be between 5 and 10737418240")
	}
	if c.DataMB < 128 || c.DataMB > 8192 {
		return errors.New("data_mb must be between 128 and 8192")
	}
	if c.MemoryMB < c.DataMB+256 || c.MemoryMB > 16384 {
		return errors.New("memory_mb must be at least data_mb + 256, and at most 16384")
	}
	if len(c.Checks) > 32 {
		return errors.New("at most 32 checks are supported")
	}
	names := map[string]bool{}
	for _, check := range c.Checks {
		if strings.TrimSpace(check.Name) == "" || len(check.Name) > 120 || names[check.Name] {
			return errors.New("check names must be unique, nonempty and at most 120 bytes")
		}
		names[check.Name] = true
		q := strings.TrimSpace(check.SQL)
		// Deliberately narrow grammar. Queries are wrapped as a scalar subquery;
		// comments and statement separators are excluded, including inside literals.
		if len(q) > 8192 || !regexp.MustCompile(`(?i)^select\s`).MatchString(q) || strings.ContainsAny(q, ";\\\x00") || strings.Contains(q, "--") || strings.Contains(q, "/*") {
			return fmt.Errorf("check %q must be a single SELECT without semicolons, comments or backslashes", check.Name)
		}
	}
	return nil
}

type CheckResult struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

type Backup struct {
	Name          string    `json:"name"`
	Bytes         int64     `json:"bytes"`
	ModifiedAt    time.Time `json:"modified_at"`
	SHA256        string    `json:"sha256,omitempty"`
	SourceVersion string    `json:"source_version,omitempty"`
	DumpVersion   string    `json:"dump_version,omitempty"`
}

type Report struct {
	SchemaVersion int           `json:"schema_version"`
	RunID         string        `json:"run_id"`
	Status        string        `json:"status"`
	Stage         string        `json:"stage"`
	StartedAt     time.Time     `json:"started_at"`
	FinishedAt    time.Time     `json:"finished_at"`
	DurationMS    int64         `json:"duration_ms"`
	Image         string        `json:"image"`
	ImageID       string        `json:"image_id,omitempty"`
	Backup        Backup        `json:"backup"`
	RestoreStatus string        `json:"restore_status"`
	Checks        []CheckResult `json:"checks"`
	CleanupStatus string        `json:"cleanup_status"`
	Error         string        `json:"error,omitempty"`
	ErrorCode     string        `json:"error_code,omitempty"`
	Warnings      []string      `json:"warnings"`
}
