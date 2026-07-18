package jobs

import (
	"context"
	"testing"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
)

func TestMigrate_CreatesRiverTables(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("river migrate: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_name = 'river_job'`).Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 1 {
		t.Errorf("river_job table missing (n=%d)", n)
	}
}
