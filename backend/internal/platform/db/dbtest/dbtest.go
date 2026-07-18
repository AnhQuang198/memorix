// Package dbtest cung cấp Postgres testcontainer + migrate cho integration test.
package dbtest

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/memorix/memorix/internal/platform/db"
)

// migrationsURL trả file://<repo>/migrations tính từ vị trí package này.
func migrationsURL() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile = <repo>/internal/platform/db/dbtest/dbtest.go
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	return "file://" + filepath.Join(root, "migrations")
}

// RunPostgres khởi động Postgres 18, áp mọi migration, trả pool đã sẵn sàng.
// Skip khi -short. Tự dọn qua t.Cleanup.
func RunPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skip container test in -short mode")
	}
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "postgres:18",
		postgres.WithDatabase("memorix"),
		postgres.WithUsername("test"), postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start pg: %v", err)
	}
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	if err := db.Migrate(migrationsURL(), dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		_ = pg.Terminate(ctx)
	})
	return pool
}
