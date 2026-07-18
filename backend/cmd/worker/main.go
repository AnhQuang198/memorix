package main

import (
	"log/slog"
	"os"

	"github.com/memorix/memorix/internal/platform/config"
	"github.com/memorix/memorix/internal/platform/logger"
)

// Worker chạy job nền (reconcile daily_stats, forecast, purge) — AD-8, ARCH-12.
// Story sau đăng ký River workers. Sprint 0 chỉ dựng skeleton chạy được.
func main() {
	log := logger.New(os.Stdout, "info")
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("config load failed", slog.Any("err", err))
		os.Exit(1)
	}
	log.Info("worker starting (no jobs registered yet)", "env", cfg.AppEnv)
	// TODO(story sau): khởi tạo river.Client với riverpgxv5 + đăng ký workers.
	select {} // giữ tiến trình sống cho container
}
