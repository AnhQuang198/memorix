package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/repo"
)

// seedCard chèn 1 card New (front_back) và trả id. status là text (0004_cards)
// nên seed 'new', không phải số.
func seedCard(t *testing.T, ctx context.Context, q db.Querier, owner, entry uuid.UUID, due time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := q.Exec(ctx, `
		INSERT INTO scheduling.cards (id, owner_id, entry_id, direction, status, due_at, created_at, updated_at)
		VALUES ($1,$2,$3,'front_back','new',$4,now(),now())`, id, owner, entry, due)
	require.NoError(t, err)
	return id
}

func TestCardStore_LoadApplyResult(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	owner, entry := uuid.New(), uuid.New()
	due := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	id := seedCard(t, ctx, pool, owner, entry, due)

	cs := repo.NewCardStore()
	card, err := cs.Load(ctx, pool, id, owner)
	require.NoError(t, err)
	require.Equal(t, entry, card.EntryID)
	require.Equal(t, domain.StatusNew, card.Status)

	// ownership: owner khác → not found (deny-by-default).
	_, err = cs.Load(ctx, pool, id, uuid.New())
	require.ErrorIs(t, err, domain.ErrCardNotFound)

	res := domain.ScheduleResult{
		Stability: 12.5, Difficulty: 6.0, Status: domain.StatusReview,
		Reps: 1, Lapses: 0, DueAt: due.AddDate(0, 0, 12), LastReviewAt: due,
	}
	require.NoError(t, cs.ApplyResult(ctx, pool, id, res))

	got, err := cs.Load(ctx, pool, id, owner)
	require.NoError(t, err)
	require.InDelta(t, 12.5, got.Stability, 1e-9)
	require.Equal(t, domain.StatusReview, got.Status)
	require.NotNil(t, got.LastReviewAt)
}

func TestCardStore_DueCards(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	owner := uuid.New()
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	seedCard(t, ctx, pool, owner, uuid.New(), now.Add(-time.Hour)) // due
	seedCard(t, ctx, pool, owner, uuid.New(), now.Add(time.Hour))  // chưa due

	cs := repo.NewCardStore()
	cards, err := cs.DueCards(ctx, pool, owner, now, 50)
	require.NoError(t, err)
	require.Len(t, cards, 1)
}

func TestPrefsStore_UpsertGetDefault(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	ps := repo.NewPrefsStore()
	uid := uuid.New()

	// chưa cấu hình → default (không rewrite quá khứ).
	p, err := ps.Get(ctx, pool, uid)
	require.NoError(t, err)
	require.Equal(t, uid, p.UserID)
	require.InDelta(t, 0.90, p.DesiredRetention, 1e-9)

	p.DesiredRetention = 0.85
	p.Timezone = "Asia/Bangkok"
	require.NoError(t, ps.Upsert(ctx, pool, p))

	got, err := ps.Get(ctx, pool, uid)
	require.NoError(t, err)
	require.InDelta(t, 0.85, got.DesiredRetention, 1e-9)
	require.Equal(t, "Asia/Bangkok", got.Timezone)
}
