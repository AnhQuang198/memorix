package repo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
)

// TestMigration0008_ProgressReadModel chứng minh 0008 (áp cùng 0001-0007 qua
// db.Migrate) tạo read model progress.daily_stats + progress.study_profiles với
// đúng cột và PRIMARY KEY. FK chỉ trong schema progress (AD-10): user_id là ref
// logic tới identity.users, KHÔNG có FK chéo schema. Skip khi -short.
func TestMigration0008_ProgressReadModel(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()

	// Cả hai bảng read model tồn tại trong schema progress.
	var n int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema='progress' AND table_name IN ('daily_stats','study_profiles')
	`).Scan(&n))
	require.Equal(t, 2, n, "progress.daily_stats + progress.study_profiles phải tồn tại")

	// daily_stats có đủ cột đếm (FR-30..34).
	var dsCols int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema='progress' AND table_name='daily_stats'
		  AND column_name IN ('user_id','day','reviews_done','new_done','retained',
		                      'again','hard','good','easy','updated_at')
	`).Scan(&dsCols))
	require.Equal(t, 10, dsCols, "daily_stats phải có 10 cột read model")

	// study_profiles có đủ cột động lực (FR-32: total_retained tích lũy).
	var spCols int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema='progress' AND table_name='study_profiles'
		  AND column_name IN ('user_id','streak_current','streak_best',
		                      'last_study_date','total_reviews','total_retained','updated_at')
	`).Scan(&spCols))
	require.Equal(t, 7, spCols, "study_profiles phải có 7 cột động lực")

	// PRIMARY KEY (user_id, day) trên daily_stats: cùng (user,day) chèn lại bị chặn.
	u := "11111111-1111-1111-1111-111111111111"
	_, err := pool.Exec(ctx,
		`INSERT INTO progress.daily_stats (user_id, day) VALUES ($1, '2026-07-08')`, u)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO progress.daily_stats (user_id, day) VALUES ($1, '2026-07-08')`, u)
	require.Error(t, err, "PK (user_id, day) trùng phải bị chặn")

	// PRIMARY KEY (user_id) trên study_profiles: user_id trùng bị chặn.
	_, err = pool.Exec(ctx, `INSERT INTO progress.study_profiles (user_id) VALUES ($1)`, u)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO progress.study_profiles (user_id) VALUES ($1)`, u)
	require.Error(t, err, "PK user_id trùng phải bị chặn")
}
