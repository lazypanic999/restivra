package verify

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestDiagnosticsPersistOnlyFixedCategories(t *testing.T) {
	for _, tc := range []struct{ input, category string }{
		{`ERROR: could not extend file: No space left on device; row contains secret@example.invalid`, "resource_limit"},
		{`ERROR: extension "private_customer_extension" is not available`, "extension_unavailable"},
		{`ERROR: canceling statement due to statement timeout; SELECT private`, "query_timeout"},
		{`ERROR: cannot execute lo_create in a read-only transaction`, "read_only_violation"},
		{`ERROR: relation "private_table" does not exist`, "object_missing"},
		{`pg_restore: could not uncompress data: incorrect data check`, "archive_corrupt"},
		{`ERROR: password=never-persist-this`, "command_failed"},
	} {
		if got := classifyDiagnostic(tc.input); got != tc.category {
			t.Errorf("category %s: got %s", tc.category, got)
		}
		public := (&CommandError{Category: classifyDiagnostic(tc.input), ExitCode: 1}).Error()
		if strings.Contains(public, "secret") || strings.Contains(public, "private") || strings.Contains(public, "password") {
			t.Fatal("diagnostic leaked source data")
		}
	}
}

func TestRestoreReportCarriesSafeFailureCode(t *testing.T) {
	source, work := fakeBackup(t)
	docker := &fakeDocker{restoreError: &CommandError{Category: "resource_limit", ExitCode: 1}}
	report := (Runner{Docker: docker, WorkDir: work}).Run(context.Background(), source, DefaultConfig())
	if report.ErrorCode != "resource_limit" || report.Status != "failed" || report.CleanupStatus != "passed" {
		t.Fatalf("unexpected report: %+v", report)
	}
	docker = &fakeDocker{restoreError: errors.New("private failure")}
	report = (Runner{Docker: docker, WorkDir: work}).Run(context.Background(), source, DefaultConfig())
	if report.ErrorCode != "restore_failed" {
		t.Fatal("generic failures need a stable code")
	}
}

type removalDeadline struct{ *fakeDocker }

func (d removalDeadline) Run(ctx context.Context, input io.Reader, args ...string) (string, error) {
	if len(args) > 0 && args[0] == "rm" {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return d.fakeDocker.Run(ctx, input, args...)
}
func TestTimedOutRemovalUsesFreshConfirmation(t *testing.T) {
	for _, exists := range []bool{false, true} {
		docker := removalDeadline{&fakeDocker{containerExists: exists}}
		err := cleanupCreatedContainer(docker, "restivra-"+strings.Repeat("a", 32), 5*time.Millisecond)
		if (err != nil) != exists {
			t.Fatalf("confirmation of remaining=%v returned %v", exists, err)
		}
	}
}
