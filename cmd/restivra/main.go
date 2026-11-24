package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/lazypanic999/restivra/internal/buildinfo"
	"github.com/lazypanic999/restivra/internal/verify"
)

func main() { os.Exit(run()) }

func run() int {
	if len(os.Args) < 2 {
		usage()
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch os.Args[1] {
	case "version":
		if len(os.Args) != 2 {
			return 2
		}
		if json.NewEncoder(os.Stdout).Encode(buildinfo.Current()) != nil {
			return 1
		}
		return 0
	case "doctor":
		return doctor(ctx, os.Args[2:])
	case "healthcheck":
		return healthcheck(ctx, os.Args[2:])
	case "serve", "worker":
		return service(ctx, os.Args[1], os.Args[2:])
	case "verify":
		flags := flag.NewFlagSet("verify", flag.ContinueOnError)
		backup := flags.String("backup", "", "path to completed PostgreSQL 17 custom dump")
		configPath := flags.String("config", "", "optional JSON configuration")
		reportPath := flags.String("report", "", "optional JSON report file (must not exist)")
		work := flags.String("work-dir", "work", "private temporary snapshot directory")
		if flags.Parse(os.Args[2:]) != nil {
			return 2
		}
		if *backup == "" || flags.NArg() != 0 {
			fmt.Fprintln(os.Stderr, "--backup is required; unexpected positional arguments are not allowed")
			return 2
		}
		cfg := verify.DefaultConfig()
		if *configPath != "" {
			f, err := os.Open(*configPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "cannot open config")
				return 2
			}
			cfg, err = verify.ReadConfig(f)
			f.Close()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 2
			}
		}
		// Reserve report output before starting a potentially expensive restore.
		var reportFile *os.File
		if *reportPath != "" {
			if err := os.MkdirAll(filepath.Dir(*reportPath), 0700); err != nil {
				fmt.Fprintln(os.Stderr, "cannot create report directory")
				return 2
			}
			var err error
			reportFile, err = os.OpenFile(*reportPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				fmt.Fprintln(os.Stderr, "report path must be writable and must not already exist")
				return 2
			}
			defer reportFile.Close()
		}
		report := (verify.Runner{WorkDir: *work}).Run(ctx, *backup, cfg)
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "cannot encode report")
			return 1
		}
		data = append(data, '\n')
		if reportFile != nil {
			if _, err = reportFile.Write(data); err == nil {
				err = reportFile.Sync()
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "cannot save report")
				return 1
			}
		}
		if _, err := os.Stdout.Write(data); err != nil {
			return 1
		}
		if report.Status != "passed" {
			return 1
		}
		return 0
	case "cleanup":
		flags := flag.NewFlagSet("cleanup", flag.ContinueOnError)
		id := flags.String("run-id", "", "exact run ID from a report or container label")
		if flags.Parse(os.Args[2:]) != nil || flags.NArg() != 0 {
			return 2
		}
		cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := (verify.Runner{}).Cleanup(cleanupCtx, *id); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Fprintln(os.Stdout, "Temporary container removed.")
		return 0
	case "help", "--help", "-h":
		usage()
		return 0
	default:
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Restivra — PostgreSQL 17 restore verification\n\n  restivra serve [--listen 127.0.0.1:7474]\n  restivra worker\n  restivra version\n  restivra doctor [--role worker]\n  restivra healthcheck [--url http://127.0.0.1:7474/healthz]\n  restivra verify --backup file.dump [--config config.json] [--report work/report.json]\n  restivra cleanup --run-id <id>\n\nServices require RESTIVRA_DATABASE_URL. See README for installation.\nPrerequisite: Docker Linux engine; docker pull "+verify.Image+"\nExit codes: 0 passed, 1 verification/runtime failure, 2 invalid command/config.")
}
