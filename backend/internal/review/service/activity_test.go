package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/review/service"
)

// insertLog chèn 1 review_logs với prev_status (0=new served, khác 0=review served).
func insertLog(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner uuid.UUID, prevStatus int16, reviewedAt time.Time) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO review.review_logs
			(id, card_id, owner_id, client_review_id, grade,
			 prev_stability, prev_difficulty, prev_status, retrievability,
			 new_stability, new_difficulty, new_status, new_reps, new_lapses, new_due_at,
			 elapsed_days, reviewed_at)
		VALUES ($1,$2,$3,$4,3, 0,0,$5,0.9, 1,1,2,1,0,$6, 0,$6)`,
		uuid.New(), uuid.New(), owner, uuid.NewString(), prevStatus, reviewedAt)
	require.NoError(t, err)
}

func TestActivityAdapter_CountServedSince(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	owner := uuid.New()
	dayStart := time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)

	insertLog(t, ctx, pool, owner, 0, dayStart.Add(1*time.Hour))  // new served
	insertLog(t, ctx, pool, owner, 0, dayStart.Add(2*time.Hour))  // new served
	insertLog(t, ctx, pool, owner, 2, dayStart.Add(3*time.Hour))  // review served
	insertLog(t, ctx, pool, owner, 3, dayStart.Add(4*time.Hour))  // review served (relearning)
	insertLog(t, ctx, pool, owner, 0, dayStart.Add(-2*time.Hour)) // hôm qua ⇒ không đếm
	insertLog(t, ctx, pool, uuid.New(), 0, dayStart.Add(1*time.Hour)) // owner khác ⇒ không đếm

	a := service.NewActivityAdapter(pool)
	c, err := a.CountServedSince(ctx, owner, dayStart)
	require.NoError(t, err)
	require.Equal(t, 2, c.NewServed)
	require.Equal(t, 2, c.ReviewServed)
}
