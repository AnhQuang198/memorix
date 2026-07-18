package dbtest

import (
	"context"
	"testing"
)

// TestRunPostgres_AppliesMigrations chứng minh helper: khởi container, áp mọi
// migration, trả pool sẵn sàng dùng (reuse bởi repo test Tasks 6/9/10).
func TestRunPostgres_AppliesMigrations(t *testing.T) {
	pool := RunPostgres(t)
	ctx := context.Background()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_schema='vocabulary'`).Scan(&n); err != nil {
		t.Fatalf("query vocabulary tables: %v", err)
	}
	if n == 0 {
		t.Error("expected vocabulary tables after migrate, got 0")
	}
}
