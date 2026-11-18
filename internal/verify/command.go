package verify

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
	"time"
)

// Command never invokes a shell. stderr is deliberately not included in reports:
// PostgreSQL errors can contain restored data, SQL or secrets.
type Commander interface {
	Run(context.Context, io.Reader, ...string) (string, error)
}

type Docker struct{}

// CommandError intentionally contains no original command or stderr. Classification
// is derived in memory from a fixed allowlist and is safe to persist in a report.
type CommandError struct {
	Category string
	ExitCode int
}

func (e *CommandError) Error() string { return "Docker command failed (" + e.Category + ")" }

func classifyDiagnostic(stderr string) string {
	message := strings.ToLower(stderr)
	switch {
	case strings.Contains(message, "no space left on device"), strings.Contains(message, "out of memory"), strings.Contains(message, "cannot allocate memory"):
		return "resource_limit"
	case strings.Contains(message, "extension") && (strings.Contains(message, "is not available") || strings.Contains(message, "could not open extension control file")):
		return "extension_unavailable"
	case strings.Contains(message, "unsupported version") || strings.Contains(message, "unsupported version in file header"):
		return "unsupported_version"
	case strings.Contains(message, "unexpected end of file") || strings.Contains(message, "could not read from input file") || strings.Contains(message, "could not uncompress data") || strings.Contains(message, "invalid archive"):
		return "archive_corrupt"
	case strings.Contains(message, "statement timeout"):
		return "query_timeout"
	case strings.Contains(message, "read-only transaction"):
		return "read_only_violation"
	case strings.Contains(message, "does not exist"):
		return "object_missing"
	default:
		return "command_failed"
	}
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if remaining := 65536 - b.Len(); remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.Buffer.Write(p)
	}
	return n, nil
}

func (Docker) Run(ctx context.Context, stdin io.Reader, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.WaitDelay = 3 * time.Second
	cmd.Stdin = stdin
	var out limitedBuffer
	var diagnostic limitedBuffer
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	err := cmd.Run()
	if err != nil && ctx.Err() == nil {
		code := -1
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		}
		return out.String(), &CommandError{Category: classifyDiagnostic(diagnostic.String()), ExitCode: code}
	}
	return out.String(), err
}
