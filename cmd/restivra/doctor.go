package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lazypanic999/restivra/internal/app"
	"github.com/lazypanic999/restivra/internal/verify"
)

type diagnosis struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// doctor inspects prerequisites without migrating, restoring or deleting data.
func doctor(ctx context.Context, args []string) int {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	role := flags.String("role", "standalone", "standalone or worker; worker does not hold the UI access token")
	if flags.Parse(args) != nil || flags.NArg() != 0 || (*role != "standalone" && *role != "worker") {
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	checks := []diagnosis{}
	failed := false
	add := func(name, status, detail string) {
		checks = append(checks, diagnosis{name, status, detail})
		if status == "failed" {
			failed = true
		}
	}
	docker := verify.Docker{}
	engine, err := docker.Run(ctx, nil, "info", "--format", "{{.OSType}}")
	if err != nil || strings.TrimSpace(engine) != "linux" {
		add("docker", "failed", "A reachable Linux Docker engine is required.")
	} else {
		add("docker", "passed", "Linux engine reachable.")
	}
	if _, err = docker.Run(ctx, nil, "image", "inspect", verify.Image, "--format", "{{.Id}}"); err != nil {
		add("restore_image", "failed", "Run docker pull "+verify.Image)
	} else {
		add("restore_image", "passed", verify.Image+" is available.")
	}
	root, err := os.OpenRoot(env("RESTIVRA_BACKUP_ROOT", "backups"))
	if err != nil {
		add("backup_root", "failed", "Backup directory is not accessible.")
	} else {
		root.Close()
		add("backup_root", "passed", "Backup directory accessible; individual archive permissions are checked per run.")
	}
	work := env("RESTIVRA_WORK_DIR", "work/jobs")
	if err = os.MkdirAll(work, 0700); err == nil {
		var f *os.File
		f, err = os.CreateTemp(work, "doctor-*")
		if err == nil {
			name := f.Name()
			err = f.Close()
			removeErr := os.Remove(name)
			if err == nil {
				err = removeErr
			}
		}
	}
	if err != nil {
		add("work_directory", "failed", "Cannot create and remove a temporary file.")
	} else {
		add("work_directory", "passed", "Private snapshot directory writable.")
	}
	if url := os.Getenv("RESTIVRA_DATABASE_URL"); url != "" {
		store, openErr := app.Open(ctx, url)
		if openErr != nil {
			add("database", "failed", "Cannot connect to metadata database.")
		} else {
			_, idErr := store.InstallationID(ctx)
			store.DB.Close()
			if idErr != nil {
				add("database", "failed", "Database reachable but installation schema is missing; start the API first.")
			} else {
				add("database", "passed", "Metadata database and installation identity ready.")
			}
		}
	} else {
		add("database", "warning", "RESTIVRA_DATABASE_URL is unset; standalone CLI needs no metadata database.")
	}
	auth, err := app.NewAuth(os.Getenv("RESTIVRA_AUTH_TOKEN"))
	if *role == "worker" {
		add("authentication", "skipped", "Worker holds no UI token. Confirm authentication_enabled through the signed-in API health endpoint.")
	} else {
		if err != nil {
			add("authentication", "failed", "RESTIVRA_AUTH_TOKEN must contain 32–256 bytes without surrounding whitespace.")
		} else if auth == nil {
			add("authentication", "warning", "UI authentication is disabled. Keep API loopback-only; configure a random token before sharing access.")
		} else {
			add("authentication", "passed", "Access token configured; its value is never displayed.")
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"checks": checks, "passed": !failed})
	if failed {
		return 1
	}
	return 0
}

func healthcheck(ctx context.Context, args []string) int {
	flags := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	url := flags.String("url", "http://127.0.0.1:7474/healthz", "API readiness endpoint")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	req, err := http.NewRequestWithContext(ctx, "GET", *url, nil)
	if err != nil {
		return 2
	}
	client := http.Client{Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return 1
	}
	fmt.Fprintln(os.Stdout, "ready")
	return 0
}
