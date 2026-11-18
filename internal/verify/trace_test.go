package verify

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

type privateDiagnostic struct{}

func (privateDiagnostic) Run(context.Context, io.Reader, ...string) (string, error) {
	return "private row", &CommandError{Category: "password=private-diagnostic"}
}
func TestCommandTimingsAreBoundedAndPrivate(t *testing.T) {
	events := []CommandTiming{}
	d := tracedDocker{privateDiagnostic{}, &events}
	for i := 0; i < 40; i++ {
		_, _ = d.Run(context.Background(), nil, "exec", "private-container", "psql", "SELECT private-password")
	}
	_, _ = d.Run(context.Background(), nil, "rm", "private-container")
	if len(events) != 32 || events[31].Operation != "rm" {
		t.Fatal("trace lost bounded cleanup evidence")
	}
	raw, _ := json.Marshal(events)
	if strings.Contains(string(raw), "private") || strings.Contains(string(raw), "password") || strings.Contains(string(raw), "SELECT") {
		t.Fatal("trace persisted command arguments or stderr")
	}
}
