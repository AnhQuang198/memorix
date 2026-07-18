package main

import (
	"context"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/memorix/memorix/internal/identity/repo"
	"github.com/memorix/memorix/internal/platform/config"
	"github.com/memorix/memorix/internal/platform/jobs"
	"github.com/memorix/memorix/internal/platform/logger"
	schedrepo "github.com/memorix/memorix/internal/scheduling/repo"
	schedsvc "github.com/memorix/memorix/internal/scheduling/service"
	vocabjobs "github.com/memorix/memorix/internal/vocabulary/jobs"
	vocabrepo "github.com/memorix/memorix/internal/vocabulary/repo"
)

const purgeRetention = 30 * 24 * time.Hour

func main() {
	log := logger.New(os.Stdout, "info")
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("config load failed", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db pool failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// --- River queue (Sprint 2): đăng ký EnrollWorker + chạy client nền ---
	if err := jobs.Migrate(ctx, pool); err != nil {
		log.Error("river migrate failed", "err", err)
		os.Exit(1)
	}
	vRepo := vocabrepo.New(pool)
	schedService := schedsvc.New(schedrepo.New(pool))

	workers := river.NewWorkers()
	river.AddWorker(workers, &vocabjobs.EnrollWorker{Store: vRepo, Cards: schedService})

	client, err := jobs.NewClient(pool, workers)
	if err != nil {
		log.Error("river client failed", "err", err)
		os.Exit(1)
	}
	if err := client.Start(ctx); err != nil {
		log.Error("river worker start failed", "err", err)
		os.Exit(1)
	}
	log.Info("river worker started: enroll_deck registered")

	// --- Identity GDPR purge (Sprint 1): ticker giữ tiến trình sống ---
	repos := repo.New(pool)
	// Purge chạy trực tiếp qua repo (không cần full Service graph cho job hạ tầng).
	tick := time.NewTicker(24 * time.Hour)
	defer tick.Stop()
	log.Info("worker started: daily GDPR purge scheduled", "retention", purgeRetention.String())
	for {
		n, err := repos.Users.PurgeDeletedBefore(ctx, time.Now().Add(-purgeRetention))
		if err != nil {
			log.Error("purge failed", "err", err)
		} else if n > 0 {
			log.Info("purged deleted accounts", "count", n)
		}
		<-tick.C
	}
}
