package app

import (
	"context"
	"encoding/json"
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
	workerCtx, stopWorker := context.WithCancel(ctx)
	done := make(chan error, 1)
	worker := Worker{Store: store, BackupRoot: rootDir, WorkDir: work, Instance: name, RetentionDays: 30}
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
	waitJob(old.ID, "interrupted")
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
	if err != nil || notified.Status != "interrupted" || notified.NotificationStatus != "failed" || notified.NotificationAttempts != 3 {
		t.Fatalf("notification retry state wrong: %+v %v", notified, err)
	}
}
func activeStatus(status string) bool { return status == "queued" || status == "running" }
