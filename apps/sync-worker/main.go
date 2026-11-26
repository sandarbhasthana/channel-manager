// Command sync-worker runs the channel manager's background jobs on a
// schedule: pulling bookings from the OTAs and resending bookings the PMS
// didn't take. The jobs themselves run inside the API (POST
// /internal/worker/jobs/{job}), where every service is wired and the OTA
// credentials are held; this process only decides when, so it needs no
// database or OTA access of its own.
//
// Environment:
//
//	SYNC_WORKER_TOKEN       shared with the API; required
//	SYNC_WORKER_API_URL     API base URL (default http://localhost:8080)
//	SYNC_PULL_INTERVAL      how often to pull OTA bookings (default 5m; 0 = off)
//	SYNC_REDELIVER_INTERVAL how often to resend undelivered bookings (default 1m; 0 = off)
//
// `sync-worker -once` runs each job once and exits (for cron).
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	once := flag.Bool("once", false, "run each job once and exit")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("app", "sync-worker")
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Error("invalid configuration", "err", err)
		os.Exit(2)
	}
	client := newJobClient(cfg.apiURL, cfg.token)
	jobs := cfg.schedule()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if *once {
		failed := false
		for _, j := range jobs {
			if err := runOnce(ctx, log, client, j.name); err != nil {
				failed = true
			}
		}
		if failed {
			os.Exit(1)
		}
		return
	}

	log.Info("sync worker starting", "api", cfg.apiURL, "jobs", describe(jobs))
	run(ctx, log, client, jobs)
	log.Info("sync worker stopped")
}

// run starts one loop per job and returns when ctx is cancelled and every
// loop has finished its current run.
func run(ctx context.Context, log *slog.Logger, client jobRunner, jobs []scheduledJob) {
	done := make(chan struct{}, len(jobs))
	for _, j := range jobs {
		go func(j scheduledJob) {
			defer func() { done <- struct{}{} }()
			loop(ctx, log, client, j)
		}(j)
	}
	for range jobs {
		<-done
	}
}

// loop runs a job now and then every interval. Runs never overlap: the next
// one is scheduled after the previous one returns.
func loop(ctx context.Context, log *slog.Logger, client jobRunner, j scheduledJob) {
	for {
		_ = runOnce(ctx, log, client, j.name)
		select {
		case <-ctx.Done():
			return
		case <-time.After(j.every):
		}
	}
}

func runOnce(ctx context.Context, log *slog.Logger, client jobRunner, job string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	res, err := client.Run(ctx, job)
	if err != nil {
		log.Error("job failed", "job", job, "err", err)
		return err
	}
	level := slog.LevelInfo
	if len(res.Errors) > 0 {
		level = slog.LevelWarn
	}
	log.Log(ctx, level, "job done",
		"job", job,
		"orgs", res.Orgs,
		"properties", res.Properties,
		"imported", res.Imported,
		"redelivered", res.Redelivered,
		"undelivered", res.Undelivered,
		"errors", res.Errors,
		"duration_ms", res.DurationMs,
	)
	return nil
}
