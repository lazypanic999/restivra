package verify

import (
	"bytes"
	"context"
	"io"
	"os/exec"
)

// Command never invokes a shell. stderr is deliberately not included in reports:
// PostgreSQL errors can contain restored data, SQL or secrets.
type Commander interface {
	Run(context.Context, io.Reader, ...string) (string, error)
}

type Docker struct{}

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
	cmd.Stdin = stdin
	var out limitedBuffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	err := cmd.Run()
	return out.String(), err
}
