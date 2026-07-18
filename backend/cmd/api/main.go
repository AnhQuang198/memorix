package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/memorix/memorix/internal/identity/handler"
	"github.com/memorix/memorix/internal/identity/mailer"
	identityrepo "github.com/memorix/memorix/internal/identity/repo"
	"github.com/memorix/memorix/internal/identity/service"
	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/platform/config"
	"github.com/memorix/memorix/internal/platform/eventbus"
	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/platform/jobs"
	"github.com/memorix/memorix/internal/platform/logger"
	"github.com/memorix/memorix/internal/platform/ratelimit"
	"github.com/memorix/memorix/internal/platform/security"
	schedrepo "github.com/memorix/memorix/internal/scheduling/repo"
	schedsvc "github.com/memorix/memorix/internal/scheduling/service"
	vocabhandler "github.com/memorix/memorix/internal/vocabulary/handler"
	vocabjobs "github.com/memorix/memorix/internal/vocabulary/jobs"
	vocabports "github.com/memorix/memorix/internal/vocabulary/ports"
	vocabrepo "github.com/memorix/memorix/internal/vocabulary/repo"
	vocabsvc "github.com/memorix/memorix/internal/vocabulary/service"
)

type sysClock struct{}

func (sysClock) Now() time.Time { return time.Now() }

// schedCardAdapter khớp scheduling.Service với vocabulary/ports.CardService.
// CreateCardsForEntry cần convert DTO (2 struct đồng shape, khác package);
// 3 method còn lại đồng signature nên được promote từ embedded Service (AD-9).
type schedCardAdapter struct{ *schedsvc.Service }

func (a schedCardAdapter) CreateCardsForEntry(ctx context.Context, in vocabports.CreateCardsInput) error {
	return a.Service.CreateCardsForEntry(ctx, schedsvc.CreateCardsInput(in))
}

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

	repos := identityrepo.New(pool)
	jwt := authmw.NewJWTManager([]byte(cfg.JWTSecret), cfg.AccessTTL, cfg.JWTIssuer)
	bus := eventbus.NewInProcess()

	svc := service.New(service.Deps{
		Users: repos.Users, Sessions: repos.Sessions, Tokens: repos.Tokens, OAuth: repos.OAuth,
		Hasher:  security.NewArgon2Hasher(),
		Issuer:  jwt,
		Secrets: security.TokenFactory{},
		Clock:   sysClock{},
		Limiter: ratelimit.NewWindow(10, time.Minute),
		Bus:     bus,
		RefreshTTL: cfg.RefreshTTL, VerifyTTL: cfg.VerifyTTL, ResetTTL: cfg.ResetTTL,
	})

	r := httpx.NewRouter()
	v1 := r.Group("/api/v1")
	h := handler.New(svc, mailer.NewLogMailer(log), jwt, cfg.RefreshTTL, cfg.AppEnv != "development", nil)
	h.RegisterRoutes(v1)
	// OAuth wiring: khi có GOOGLE_CLIENT_ID, dựng oauthx.New(ctx, ...) và truyền OAuthDeps.
	// Bỏ qua ở bootstrap tối thiểu nếu chưa cấu hình provider.

	_ = service.NewPort(repos.Users) // IdentityPort — module khác consume ở sprint sau

	// --- Vocabulary (Sprint 2) ---
	// River insert-only ở API; đảm bảo schema River tồn tại trước khi tạo client.
	if err := jobs.Migrate(ctx, pool); err != nil {
		log.Error("river migrate failed", "err", err)
		os.Exit(1)
	}
	riverClient, err := jobs.NewClient(pool, nil)
	if err != nil {
		log.Error("river client failed", "err", err)
		os.Exit(1)
	}

	cards := schedCardAdapter{schedsvc.New(schedrepo.New(pool))}
	var _ vocabports.CardService = cards // compile-time: adapter thỏa port
	vRepo := vocabrepo.New(pool)
	enqueuer := &vocabjobs.Enqueuer{Client: riverClient}
	vService := vocabsvc.New(vRepo, vRepo, cards, enqueuer)
	vHandler := vocabhandler.New(vService)

	// Route vocabulary cần principal → group riêng guard bằng RequireAuth (Sprint 1).
	// Tách khỏi group identity (login/register public) dù cùng prefix /api/v1.
	secured := r.Group("/api/v1")
	secured.Use(authmw.RequireAuth(jwt))
	vocabhandler.RegisterRoutes(secured, vHandler)

	log.Info("api starting", "port", cfg.HTTPPort, "env", cfg.AppEnv)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, r); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
