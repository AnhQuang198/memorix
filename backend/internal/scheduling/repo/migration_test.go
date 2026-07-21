package repo_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
)

// TestMigration0007_QueueObjects chứng minh 0007 (áp cùng 0001-0006 qua db.Migrate):
// index nóng NFR-2 cards(owner_id,due_at) tồn tại + bảng scheduling.study_profiles
// (cờ coach Story 4.5) tồn tại với đúng cột. Skip khi -short.
func TestMigration0007_QueueObjects(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()

	// NFR-2: index idx_cards_owner_due tồn tại trên scheduling.cards.
	var idxCount int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_indexes
		WHERE schemaname='scheduling' AND tablename='cards' AND indexname='idx_cards_owner_due'
	`).Scan(&idxCount))
	require.Equal(t, 1, idxCount, "index idx_cards_owner_due phải tồn tại (NFR-2)")

	// Story 4.5: bảng study_profiles tồn tại.
	var tableExists bool
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM information_schema.tables
			WHERE table_schema='scheduling' AND table_name='study_profiles')
	`).Scan(&tableExists))
	require.True(t, tableExists, "scheduling.study_profiles phải tồn tại")

	// study_profiles có đủ cột cờ coach + scaffolding.
	var cols int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema='scheduling' AND table_name='study_profiles'
		  AND column_name IN ('user_id','learn_coach_seen_at','created_at','updated_at')
	`).Scan(&cols))
	require.Equal(t, 4, cols, "study_profiles phải có 4 cột (user_id, learn_coach_seen_at, created_at, updated_at)")

	// user_id là PRIMARY KEY: insert trùng phải bị chặn.
	uid := "22222222-2222-2222-2222-222222222222"
	_, err := pool.Exec(ctx, `INSERT INTO scheduling.study_profiles (user_id) VALUES ($1)`, uid)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO scheduling.study_profiles (user_id) VALUES ($1)`, uid)
	require.Error(t, err, "user_id trùng phải bị PRIMARY KEY chặn")
}
