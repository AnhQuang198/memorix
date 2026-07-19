package db_test

import (
	"context"
	"testing"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/stretchr/testify/require"
)

func TestMigration0006_Schema(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()

	// cards có đủ cột FSRS
	var cols int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema='scheduling' AND table_name='cards'
		  AND column_name IN ('stability','difficulty','status','reps','lapses','due_at','last_review_at')
	`).Scan(&cols))
	require.Equal(t, 7, cols)

	// user_scheduler_prefs tồn tại
	var prefsExists bool
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM information_schema.tables
			WHERE table_schema='scheduling' AND table_name='user_scheduler_prefs')
	`).Scan(&prefsExists))
	require.True(t, prefsExists, "user_scheduler_prefs phải tồn tại")

	// review_logs là bảng partitioned
	var isPartitioned bool
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM pg_partitioned_table pt
			JOIN pg_class c ON c.oid=pt.partrelid
			JOIN pg_namespace n ON n.oid=c.relnamespace
			WHERE n.nspname='review' AND c.relname='review_logs')
	`).Scan(&isPartitioned))
	require.True(t, isPartitioned, "review_logs phải partitioned")

	// review_logs có default partition (chấp nhận mọi hàng khi chưa có partition tháng)
	var hasDefault bool
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM pg_class c
			JOIN pg_namespace n ON n.oid=c.relnamespace
			WHERE n.nspname='review' AND c.relname='review_logs_default')
	`).Scan(&hasDefault))
	require.True(t, hasDefault, "review_logs_default partition phải tồn tại")

	// CHECK desired_retention chặn ngoài [0.80, 0.97]
	_, err := pool.Exec(ctx, `
		INSERT INTO scheduling.user_scheduler_prefs(user_id, desired_retention)
		VALUES (gen_random_uuid(), 0.5)`)
	require.Error(t, err, "0.5 phải bị CHECK từ chối")

	// grade_receipts có unique(card_id, client_review_id)
	cid := "11111111-1111-1111-1111-111111111111"
	ins := func() error {
		_, e := pool.Exec(ctx, `
			INSERT INTO review.grade_receipts
			  (card_id, client_review_id, review_log_id, new_stability, new_difficulty,
			   new_status, new_reps, new_lapses, new_due_at)
			VALUES ($1,'cr-1',gen_random_uuid(),1,5,2,1,0,now())`, cid)
		return e
	}
	require.NoError(t, ins())
	require.Error(t, ins(), "trùng (card_id, client_review_id) phải bị chặn")
}
