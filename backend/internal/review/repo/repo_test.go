package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
	revdom "github.com/memorix/memorix/internal/review/domain"
	"github.com/memorix/memorix/internal/review/repo"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
)

func sampleRow(owner, card uuid.UUID, at time.Time) revdom.ReviewLogRow {
	return revdom.ReviewLogRow{
		ID: uuid.New(), CardID: card, OwnerID: owner, ClientReviewID: "cr-" + at.String(),
		Grade: scheddom.GradeGood, PrevStability: 0, PrevDifficulty: 0, PrevStatus: scheddom.StatusNew,
		Retrievability: 0.9, NewStability: 5, NewDifficulty: 5, NewStatus: scheddom.StatusReview,
		NewReps: 1, NewLapses: 0, NewDueAt: at.AddDate(0, 0, 5), ElapsedDays: 0, ReviewedAt: at,
	}
}

func TestReviewLog_AppendAndList(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	owner, card := uuid.New(), uuid.New()
	lr := repo.NewReviewLogRepo()

	t0 := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	require.NoError(t, lr.Append(ctx, pool, sampleRow(owner, card, t0)))
	require.NoError(t, lr.Append(ctx, pool, sampleRow(owner, card, t0.Add(time.Minute))))

	byCard, err := lr.ListForCard(ctx, pool, card)
	require.NoError(t, err)
	require.Len(t, byCard, 2)
	require.True(t, !byCard[1].ReviewedAt.Before(byCard[0].ReviewedAt), "phải sort tăng dần")
	require.Equal(t, scheddom.StatusNew, byCard[0].PrevStatus)
	require.Equal(t, scheddom.StatusReview, byCard[0].NewStatus)
	require.Equal(t, scheddom.GradeGood, byCard[0].Grade)

	sinceStart := t0.Add(-time.Hour).Format(time.RFC3339Nano)
	byOwner, err := lr.ListForOwnerSince(ctx, pool, owner, sinceStart)
	require.NoError(t, err)
	require.Len(t, byOwner, 2)
}

func TestReceipt_InsertIdempotentAndGet(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	card := uuid.New()
	rr := repo.NewReceiptRepo()

	res := revdom.GradeResult{
		CardID: card, Stability: 7, Difficulty: 5, Status: scheddom.StatusReview,
		Reps: 1, Lapses: 0, DueAt: time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC),
	}
	inserted, err := rr.Insert(ctx, pool, res, uuid.New(), "cr-1")
	require.NoError(t, err)
	require.True(t, inserted)

	// trùng → không chèn
	inserted2, err := rr.Insert(ctx, pool, res, uuid.New(), "cr-1")
	require.NoError(t, err)
	require.False(t, inserted2)

	got, ok, err := rr.Get(ctx, pool, card, "cr-1")
	require.NoError(t, err)
	require.True(t, ok)
	require.InDelta(t, 7, got.Stability, 1e-9)
	require.Equal(t, scheddom.StatusReview, got.Status)

	// chưa có → ok=false
	_, ok, err = rr.Get(ctx, pool, card, "missing")
	require.NoError(t, err)
	require.False(t, ok)
}
