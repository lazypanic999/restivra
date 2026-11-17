package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lazypanic999/restivra/internal/app"
	"github.com/lazypanic999/restivra/internal/webui"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func service(ctx context.Context, mode string, args []string) int {
	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	listen := flags.String("listen", env("RESTIVRA_LISTEN", "127.0.0.1:7474"), "HTTP listen address")
	assetsDirectory := flags.String("assets-dir", "", "optional compiled frontend directory for local development")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	url := os.Getenv("RESTIVRA_DATABASE_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "RESTIVRA_DATABASE_URL is required")
		return 2
	}
	startCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	store, err := app.Open(startCtx, url)
	if err == nil {
		err = store.Migrate(startCtx)
	}
	cancel()
	if err != nil {
		if store != nil {
			store.DB.Close()
		}
		log.Print("database startup failed; check connection settings")
		return 1
	}
	defer store.DB.Close()
	instance := env("RESTIVRA_INSTANCE", "restivra-main")
	if mode == "worker" {
		webhook := os.Getenv("RESTIVRA_WEBHOOK_URL")
		if err := app.ValidateWebhook(webhook); err != nil {
			log.Print(err)
			return 2
		}
		days, err := strconv.Atoi(env("RESTIVRA_RETENTION_DAYS", "30"))
		if err != nil || days < 1 || days > 3650 {
			log.Print("RESTIVRA_RETENTION_DAYS must be between 1 and 3650")
			return 2
		}
		worker := app.Worker{Store: store, BackupRoot: env("RESTIVRA_BACKUP_ROOT", "backups"), WorkDir: env("RESTIVRA_WORK_DIR", "work/jobs"), Instance: instance, WebhookURL: webhook, RetentionDays: days}
		if err := worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Print("worker stopped: ", err)
			return 1
		}
		return 0
	}
	hosts := strings.Split(env("RESTIVRA_ALLOWED_HOSTS", "localhost:7474,127.0.0.1:7474,[::1]:7474"), ",")
	for i := range hosts {
		hosts[i] = strings.TrimSpace(hosts[i])
	}
	api := app.API{Store: store, AllowedHosts: hosts, Instance: instance}
	if *assetsDirectory != "" {
		handler, closeAssets, err := webui.DirectoryHandler(*assetsDirectory)
		if err != nil {
			log.Print("could not open frontend asset directory")
			return 2
		}
		defer closeAssets()
		api.Assets = handler
	}
	server := &http.Server{Addr: *listen, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	go func() {
		<-ctx.Done()
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("Restivra listening on http://%s", *listen)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Print("HTTP server failed: ", err)
		return 1
	}
	return 0
}
