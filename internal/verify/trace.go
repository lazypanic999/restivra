package verify

import (
	"context"
	"errors"
	"io"
	"time"
)

type CommandTiming struct {
	Operation  string `json:"operation"`
	DurationMS int64  `json:"duration_ms"`
	Outcome    string `json:"outcome"`
}
type tracedDocker struct {
	Commander
	events *[]CommandTiming
}

func (d tracedDocker) Run(ctx context.Context, input io.Reader, args ...string) (string, error) {
	start := time.Now()
	out, err := d.Commander.Run(ctx, input, args...)
	elapsed := time.Since(start)
	// Keep slow/error calls only, bounded per report. Never store arguments,
	// SQL, stdout or stderr; operation/outcome are fixed vocabulary.
	if err != nil || elapsed >= time.Second {
		op := "other"
		if len(args) > 0 {
			switch args[0] {
			case "info", "image", "create", "start", "exec", "rm", "ps":
				op = args[0]
			}
		}
		outcome := "passed"
		if err != nil {
			outcome = "command_failed"
			var ce *CommandError
			if errors.As(err, &ce) {
				switch ce.Category {
				case "resource_limit", "extension_unavailable", "unsupported_version", "archive_corrupt", "query_timeout", "read_only_violation", "object_missing":
					outcome = ce.Category
				}
			}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				outcome = "deadline_exceeded"
			} else if errors.Is(ctx.Err(), context.Canceled) {
				outcome = "cancelled"
			}
		}
		event := CommandTiming{op, elapsed.Milliseconds(), outcome}
		if len(*d.events) < 32 {
			*d.events = append(*d.events, event)
		} else {
			// Preserve initial evidence and the latest failures/cleanup rather
			// than letting normal readiness retries hide the incident's end.
			copy((*d.events)[8:31], (*d.events)[9:32])
			(*d.events)[31] = event
		}
	}
	return out, err
}
