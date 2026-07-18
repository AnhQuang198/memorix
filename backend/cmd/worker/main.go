package main

import (
	"context"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/memorix/memorix/internal/identity/repo"
	"github.com/memorix/memorix/internal/platform/config"
	"github.com/memorix/memorix/internal/platform/logger"
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
