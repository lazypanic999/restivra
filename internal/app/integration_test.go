package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lazypanic999/restivra/internal/verify"
)

func TestIntegrationApp(t *testing.T) {
	if os.Getenv("RESTIVRA_INTEGRATION") != "1" {
		t.Skip("set RESTIVRA_INTEGRATION=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	d := verify.Docker{}
	name := "restivra-app-test-" + NewID()
	password := NewID()
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		if _, err := d.Run(cleanup, nil, "rm", "--force", "--volumes", name); err != nil {
			t.Error("test database cleanup failed")
		}
	}()
	if _, err := d.Run(ctx, nil, "run", "--detach", "--name", name, "--publish", "127.0.0.1::5432", "--memory", "512m", "--env", "POSTGRES_PASSWORD="+password, verify.Image); err != nil {
		t.Fatal("cannot start test database")
	}
	port, err := d.Run(ctx, nil, "port", name, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}
	_, number, err := net.SplitHostPort(strings.TrimSpace(port))
	if err != nil {
		t.Fatal(err)
	}
	var store *Store
	for attempts := 0; attempts < 120; attempts++ {
		store, err = Open(ctx, "postgres://postgres:"+password+"@127.0.0.1:"+number+"/postgres?sslmode=disable")
		if err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if err != nil {
		logs, _ := d.Run(ctx, nil, "logs", name)
		t.Fatalf("test database not ready: %v; logs: %s", err, logs)
	}
	defer store.DB.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = store.Migrate(ctx); err != nil {
		t.Fatal("migration is not idempotent:", err)
	}
	rootDir := t.TempDir()
	work := t.TempDir()
	if _, err = d.Run(ctx, nil, "exec", name, "psql", "-U", "postgres", "-c", "CREATE TABLE customers(id int PRIMARY KEY); INSERT INTO customers VALUES(1),(2);"); err != nil {
		t.Fatal("fixture schema failed")
	}
	dump, err := d.Run(ctx, nil, "exec", name, "pg_dump", "-U", "postgres", "-Fc", "postgres")
	if err != nil {
		t.Fatal("fixture dump failed")
	}
	if err = os.WriteFile(filepath.Join(rootDir, "customer.dump"), []byte(dump), 0600); err != nil {
		t.Fatal(err)
	}
	api := API{Store: store, Instance: name, AllowedHosts: []string{"localhost:7474"}, CSRFToken: NewID()}
	handler := api.Handler()
	cfg := verify.DefaultConfig()
	cfg.Checks = []verify.Check{{Name: "rows", SQL: "SELECT count(*)=2 FROM public.customers"}}
	p := Project{Name: "Integration customer", BackupPattern: "*.dump", Config: cfg, Timezone: "Europe/Istanbul", ScheduleTime: "03:00"}
	response := requestJSON(t, handler, "POST", "/api/projects", p, api.CSRFToken)
	if response.Code != 201 {
		t.Fatalf("create failed: %d %s", response.Code, response.Body.String())
	}
	if err = json.Unmarshal(response.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	j, err := store.Enqueue(ctx, p.ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Enqueue(ctx, p.ID, "manual"); err != ErrConflict {
		t.Fatalf("duplicate active job accepted: %v", err)
	}
	if err = store.Archive(ctx, p.ID); err != ErrConflict {
		t.Fatalf("archived active project: %v", err)
	}
	if err = store.Cancel(ctx, j.ID); err != nil {
		t.Fatal(err)
	}
	cancelled, err := store.Job(ctx, j.ID)
	if err != nil || cancelled.Status != "cancelled" {
		t.Fatal("queued cancellation failed")
	}
	// An old running job and a synthetic private snapshot model an abrupt exit.
	old, err := store.Enqueue(ctx, p.ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Claim(ctx); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(work, old.ID), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(work, old.ID, "snapshot-abandoned.dump"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Run(ctx, nil, "create", "--name", "restivra-"+old.ID, "--label", "app=restivra", "--label", "restivra.run="+old.ID, "--label", "restivra.instance="+name, "--network", "none", verify.Image); err != nil {
		t.Fatal("could not create abandoned restore fixture")
	}
	// Another database may use the same human-readable instance name. Its
	// container must survive this worker's startup recovery.
	foreignID := NewID()
	if _, err = d.Run(ctx, nil, "create", "--name", "restivra-"+foreignID, "--label", "app=restivra", "--label", "restivra.run="+foreignID, "--label", "restivra.instance="+name, "--label", "restivra.installation="+NewID(), "--network", "none", verify.Image); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = d.Run(context.Background(), nil, "rm", "--force", "--volumes", "restivra-"+foreignID) }()
	workerCtx, stopWorker := context.WithCancel(ctx)
	done := make(chan error, 1)
	worker := Worker{Store: store, BackupRoot: rootDir, WorkDir: work, Instance: name, RetentionDays: 30, reconcileInterval: time.Second}
	go func() { done <- worker.Run(workerCtx) }()
	workerStopped := false
	defer func() {
		if workerStopped {
			return
		}
		stopWorker()
		if err := <-done; err != nil {
			t.Errorf("worker: %v", err)
		}
	}()
	waitJob := func(id, status string) Job {
		t.Helper()
		for attempts := 0; attempts < 180; attempts++ {
			job, e := store.Job(ctx, id)
			if e != nil {
				t.Fatal(e)
			}
			if !activeStatus(job.Status) {
				if job.Status != status {
					t.Fatalf("expected %s, got %+v", status, job)
				}
				return job
			}
			time.Sleep(250 * time.Millisecond)
		}
		t.Fatal("job did not finish")
		return Job{}
	}
	// Startup recovery persists the interrupted attempt and schedules a retry.
	for attempts := 0; attempts < 120; attempts++ {
		recovered, e := store.Job(ctx, old.ID)
		if e != nil {
			t.Fatal(e)
		}
		if recovered.Status == "queued" && len(recovered.Attempts) == 1 {
			if recovered.RetryAt == nil || !recovered.RetryAt.After(time.Now()) || recovered.Attempts[0].Status != "interrupted" {
				t.Fatal("recovery retry lost its backoff/audit")
			}
			if _, e = store.DB.ExecContext(ctx, "UPDATE jobs SET retry_at=clock_timestamp() WHERE id=$1", old.ID); e != nil {
				t.Fatal(e)
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	waitJob(old.ID, "passed")
	if _, err = d.Run(ctx, nil, "inspect", "restivra-"+foreignID); err != nil {
		t.Fatal("recovery deleted another installation's container")
	}
	leftovers, err := d.Run(ctx, nil, "ps", "--all", "--quiet", "--filter", "label=restivra.run="+old.ID)
	if err != nil || strings.TrimSpace(leftovers) != "" {
		t.Fatal("recovery left abandoned restore container")
	}
	if _, err = os.Stat(filepath.Join(work, old.ID)); !os.IsNotExist(err) {
		t.Fatal("recovery left abandoned private snapshot")
	}
	// A second worker cannot claim jobs in the same database.
	otherCtx, stopOther := context.WithTimeout(ctx, 3*time.Second)
	err = (&Worker{Store: store, BackupRoot: rootDir, WorkDir: work, Instance: name + "-other"}).Run(otherCtx)
	stopOther()
	if err == nil || !strings.Contains(err.Error(), "another worker") {
		t.Fatalf("second worker was not excluded: %v", err)
	}
	good, err := store.Enqueue(ctx, p.ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	result := waitJob(good.ID, "passed")
	if result.Report == nil || result.Report.CleanupStatus != "passed" || result.Report.RunID != good.ID {
		t.Fatalf("invalid persisted report: %+v", result)
	}
	again, err := store.Enqueue(ctx, p.ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	repeated := waitJob(again.ID, "passed")
	found := false
	for _, warning := range repeated.Report.Warnings {
		if strings.Contains(warning, "identical") {
			found = true
		}
	}
	if !found {
		t.Fatal("unchanged backup warning missing")
	}
	response = requestJSON(t, handler, "GET", "/api/jobs/"+good.ID+"/report", nil, "")
	if response.Code != 200 || !strings.Contains(response.Header().Get("Content-Disposition"), good.ID) {
		t.Fatal("report download failed")
	}
	// A due schedule is enqueued once and next_run_at advances atomically.
	if _, err = store.DB.ExecContext(ctx, "UPDATE projects SET next_run_at=clock_timestamp()-interval '1 minute' WHERE id=$1", p.ID); err != nil {
		t.Fatal(err)
	}
	if err = store.Schedule(ctx, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err = store.Schedule(ctx, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = store.DB.QueryRowContext(ctx, "SELECT count(*) FROM jobs WHERE trigger='schedule'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("schedule duplicated: %d %v", count, err)
	}
	jobs, err := store.Jobs(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, scheduled := range jobs {
		if scheduled.Trigger == "schedule" {
			waitJob(scheduled.ID, "passed")
		}
	}
	// Running cancellation is observed by heartbeat and still cleans resources.
	p.Config.Checks = []verify.Check{{Name: "slow", SQL: "SELECT pg_sleep(25) IS NULL"}}
	if _, err = store.SaveProject(ctx, p, false); err != nil {
		t.Fatal(err)
	}
	slow, err := store.Enqueue(ctx, p.ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	for attempts := 0; attempts < 80; attempts++ {
		job, _ := store.Job(ctx, slow.ID)
		if job.Status == "running" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err = store.Cancel(ctx, slow.ID); err != nil {
		t.Fatal(err)
	}
	waitJob(slow.ID, "cancelled")
	entries, err := os.ReadDir(work)
	if err != nil || len(entries) != 0 {
		t.Fatal("worker leaked private files")
	}
	// Model the Docker daemon completing a cancelled create after the CLI's
	// initial cleanup/absence check. The idle worker must reconcile it without
	// a restart and without changing the cancelled verification outcome.
	installation, err := store.InstallationID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundLate, err := d.Run(ctx, nil, "ps", "--all", "--quiet", "--filter", "name=^/restivra-"+slow.ID+"$")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(foundLate) == "" {
		if _, err = d.Run(ctx, nil, "create", "--name", "restivra-"+slow.ID, "--label", "app=restivra", "--label", "restivra.run="+slow.ID, "--label", "restivra.installation="+installation, "--network", "none", verify.Image); err != nil {
			t.Fatal(err)
		}
	}
	stage, err := os.MkdirTemp(work, ".stage-")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(stage, "snapshot-late.dump"), []byte("synthetic late snapshot"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(stage, filepath.Join(work, slow.ID)); err != nil {
		t.Fatal(err)
	}
	cleanedLate := false
	for attempt := 0; attempt < 80; attempt++ {
		left, e := d.Run(ctx, nil, "ps", "--all", "--quiet", "--filter", "label=restivra.run="+slow.ID)
		_, statErr := os.Stat(filepath.Join(work, slow.ID))
		if e == nil && strings.TrimSpace(left) == "" && os.IsNotExist(statErr) {
			cleanedLate = true
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !cleanedLate {
		t.Fatal("idle reconciliation left a late cancelled container/snapshot")
	}
	if _, err = d.Run(ctx, nil, "inspect", "restivra-"+foreignID); err != nil {
		t.Fatal("idle reconciliation deleted another installation's container")
	}
	lateJob, err := store.Job(ctx, slow.ID)
	if err != nil || lateJob.Status != "cancelled" {
		t.Fatal("reconciliation rewrote the cancelled job")
	}
	stopWorker()
	err = <-done
	workerStopped = true
	if err != nil {
		t.Fatal(err)
	}
	// Delivery retries are durable and never rewrite the verification outcome.
	worker.WebhookURL = "https://127.0.0.1/blocked"
	for attempt := 1; attempt <= 3; attempt++ {
		if _, err = store.DB.ExecContext(ctx, "UPDATE jobs SET notification_status='pending',notification_next_at=clock_timestamp() WHERE id=$1", old.ID); err != nil {
			t.Fatal(err)
		}
		if err = worker.notifyPending(ctx); err != nil {
			t.Fatal(err)
		}
	}
	notified, err := store.Job(ctx, old.ID)
	if err != nil || notified.Status != "passed" || notified.NotificationStatus != "failed" || notified.NotificationAttempts != 3 {
		t.Fatalf("notification retry state wrong: %+v %v", notified, err)
	}
	// A failed removal is a stop condition, never permission to claim the next
	// queued job. Restart must reconcile the failed job's labelled container.
	p.ScheduleTime = ""
	p.Config.Checks = []verify.Check{{Name: "rows", SQL: "SELECT count(*)=2 FROM public.customers"}}
	if _, err = store.SaveProject(ctx, p, false); err != nil {
		t.Fatal(err)
	}
	blocked, err := store.Enqueue(ctx, p.ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	nextProject := p
	nextProject.ID = ""
	nextProject.Name = "After cleanup recovery"
	nextProject, err = store.SaveProject(ctx, nextProject, true)
	if err != nil {
		t.Fatal(err)
	}
	next, err := store.Enqueue(ctx, nextProject.ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	broken := Worker{Store: store, BackupRoot: rootDir, WorkDir: work, Instance: name, Docker: rejectCleanup{Commander: d, RunID: blocked.ID}}
	if err = broken.Run(ctx); !errors.Is(err, ErrCleanupPending) {
		t.Fatalf("worker did not stop on cleanup failure: %v", err)
	}
	failedJob, err := store.Job(ctx, blocked.ID)
	if err != nil || failedJob.Status != "queued" || len(failedJob.Attempts) != 1 || failedJob.Attempts[0].Report == nil || failedJob.Attempts[0].Report.ErrorCode != "cleanup_failed" {
		t.Fatalf("cleanup failure not durable: %+v %v", failedJob, err)
	}
	nextJob, err := store.Job(ctx, next.ID)
	if err != nil || nextJob.Status != "queued" {
		t.Fatal("worker claimed another job after failed cleanup")
	}
	restartCtx, stopRestart := context.WithCancel(ctx)
	restarted := Worker{Store: store, BackupRoot: rootDir, WorkDir: work, Instance: name}
	restartedDone := make(chan error, 1)
	restartStopped := false
	go func() { restartedDone <- restarted.Run(restartCtx) }()
	defer func() {
		if restartStopped {
			return
		}
		stopRestart()
		if err := <-restartedDone; err != nil {
			t.Error(err)
		}
	}()
	waitJob(next.ID, "passed")
	leftovers, err = d.Run(ctx, nil, "ps", "--all", "--quiet", "--filter", "label=restivra.run="+blocked.ID)
	if err != nil || strings.TrimSpace(leftovers) != "" {
		t.Fatal("restart did not reconcile failed cleanup")
	}
	listed, err := store.Projects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundFailed, foundPassed := false, false
	for _, project := range listed {
		if project.ID == p.ID {
			foundFailed = project.LastJob != nil && project.LastJob.ID == blocked.ID && project.LastJob.AttemptCount >= 1
		}
		if project.ID == nextProject.ID {
			foundPassed = project.LastJob != nil && project.LastJob.ID == next.ID && project.LastJob.Status == "passed" && project.LastJob.ProjectName == "After cleanup recovery"
		}
	}
	if !foundFailed || !foundPassed {
		t.Fatal("project listing lost latest immutable job/report details")
	}
	stopRestart()
	if err = <-restartedDone; err != nil {
		t.Fatal(err)
	}
	restartStopped = true
	if _, err = store.DB.ExecContext(ctx, "UPDATE jobs SET status='cancelled',finished_at=clock_timestamp() WHERE status='queued'"); err != nil {
		t.Fatal(err)
	}
	testRetryPolicy(t, ctx, store, p)
	testHistoryAndProjectRestore(t, ctx, store, p)
	testSourceSelections(t, ctx, store, p)
	testEvidenceEndpoint(t, ctx, store, p)
	testComparisonEndpoint(t, ctx, store, p)
}

type rejectCleanup struct {
	verify.Commander
	RunID string
}

func (d rejectCleanup) Run(ctx context.Context, input io.Reader, args ...string) (string, error) {
	if len(args) > 0 && args[0] == "rm" && args[len(args)-1] == "restivra-"+d.RunID {
		return "", errors.New("injected Docker removal failure")
	}
	return d.Commander.Run(ctx, input, args...)
}
func activeStatus(status string) bool { return status == "queued" || status == "running" }
