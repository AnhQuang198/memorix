package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/memorix/memorix/internal/platform/config"
	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/platform/logger"
)

func main() {
	log := logger.New(os.Stdout, "info")
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("config load failed", "err", err)
		os.Exit(1)
	}
	r := httpx.NewRouter()
	log.Info("api starting", "port", cfg.HTTPPort, "env", cfg.AppEnv)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, r); err != nil {
		log.Error("server stopped", slog.Any("err", err))
		os.Exit(1)
	}
}
