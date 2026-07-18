package repo

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startPG(t *testing.T) string {
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
	t.Cleanup(func() { _ = pg.Terminate(ctx) })
	dsn, _ := pg.ConnectionString(ctx, "sslmode=disable")
	if err := db.Migrate("file://../../../migrations", dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return dsn
}

func TestIdentitySchema_TablesExist(t *testing.T) {
	dsn := startPG(t)
	ctx := context.Background()
	conn, _ := pgx.Connect(ctx, dsn)
	defer func() { _ = conn.Close(ctx) }()

	var count int
	err := conn.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables
		 WHERE table_schema = 'identity'
		 AND table_name IN ('users','email_tokens','sessions','oauth_identities')`).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 identity tables, got %d", count)
	}
}
