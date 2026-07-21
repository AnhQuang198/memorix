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

// insertCardFull chèn 1 card với status/due/deleted tuỳ ý (schema 0004+0006: status
// là text, không phải int). Trả id để assert sau.
func insertCardFull(t *testing.T, ctx context.Context, q db.Querier, owner uuid.UUID,
	status string, due *time.Time, deleted *time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := q.Exec(ctx, `
		INSERT INTO scheduling.cards
			(id, owner_id, entry_id, direction, status, stability, difficulty, due_at, deleted_at, created_at, updated_at)
		VALUES ($1,$2,$3,'front_back',$4,5,5,$5,$6,now(),now())`,
		id, owner, uuid.New(), status, due, deleted)
	require.NoError(t, err)
	return id
}

func TestQueueRepo_LoadCandidates_Ordered(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	owner := uuid.New()
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)

	overdue := now.Add(-72 * time.Hour)
	dueToday := now.Add(-1 * time.Hour)
	future := now.Add(48 * time.Hour)
	deleted := now

	overdueID := insertCardFull(t, ctx, pool, owner, "review", &overdue, nil)
	newID := insertCardFull(t, ctx, pool, owner, "new", nil, nil)             // due NULL, vẫn nạp (new)
	dueID := insertCardFull(t, ctx, pool, owner, "review", &dueToday, nil)    // due hôm nay ⇒ nạp
	_ = insertCardFull(t, ctx, pool, owner, "review", &future, nil)           // tương lai + không new ⇒ loại
	_ = insertCardFull(t, ctx, pool, owner, "suspended", &dueToday, nil)      // suspended ⇒ loại
	_ = insertCardFull(t, ctx, pool, owner, "review", &dueToday, &deleted)    // deleted ⇒ loại
	// nhiễu owner khác:
	_ = insertCardFull(t, ctx, pool, uuid.New(), "review", &dueToday, nil)

	r := repo.NewQueueRepo(pool)
	cards, err := r.LoadCandidates(ctx, owner, now)
	require.NoError(t, err)
	require.Len(t, cards, 3, "chỉ nạp new + due, không suspended/deleted/future-non-new")

	got := map[uuid.UUID]bool{}
	for _, c := range cards {
		got[c.ID] = true
	}
	require.True(t, got[overdueID] && got[newID] && got[dueID])

	// ORDER BY due_at (NULLS LAST): các thẻ có due phải tăng dần; new (NULL) cuối.
	var lastDue time.Time
	seenNil := false
	for i, c := range cards {
		if c.DueAt == nil {
			seenNil = true
			continue
		}
		require.Falsef(t, seenNil, "thẻ có due sau thẻ due NULL tại %d (NULLS LAST sai)", i)
		if i > 0 {
			require.Falsef(t, c.DueAt.Before(lastDue), "due giảm dần tại %d", i)
		}
		lastDue = *c.DueAt
	}

	// Card được nạp full FSRS state (scanCard tái dùng).
	for _, c := range cards {
		require.Equal(t, owner, c.OwnerID)
		require.True(t, c.Status.Valid())
	}
}

func TestQueueRepo_BulkDefer(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	owner := uuid.New()
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	overdue := now.Add(-72 * time.Hour)
	id := insertCardFull(t, ctx, pool, owner, "review", &overdue, nil)

	r := repo.NewQueueRepo(pool)
	newDue := now.AddDate(0, 0, 3)
	require.NoError(t, r.BulkDefer(ctx, []domain.DeferredCard{{CardID: id, NewDueAt: newDue}}))

	var got time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT due_at FROM scheduling.cards WHERE id=$1`, id).Scan(&got))
	require.WithinDuration(t, newDue, got, time.Second)

	// Batch rỗng ⇒ no-op không lỗi.
	require.NoError(t, r.BulkDefer(ctx, nil))
}

func TestQueueRepo_Prefs_UpdateLimits_Get(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	owner := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO scheduling.user_scheduler_prefs (user_id, timezone) VALUES ($1,'Asia/Bangkok')`, owner)
	require.NoError(t, err)

	r := repo.NewQueueRepo(pool)
	p, err := r.UpdateLimits(ctx, owner, 30, 150)
	require.NoError(t, err)
	require.Equal(t, 30, p.DailyNewLimit)
	require.Equal(t, 150, p.DailyReviewLimit)
	require.Equal(t, "Asia/Bangkok", p.Timezone)
	require.Equal(t, owner, p.UserID)

	got, err := r.Get(ctx, owner)
	require.NoError(t, err)
	require.Equal(t, 30, got.DailyNewLimit)
	require.Equal(t, 150, got.DailyReviewLimit)
}

func TestQueueRepo_CoachSeen_Idempotent(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	owner := uuid.New()
	r := repo.NewQueueRepo(pool)

	seen, err := r.CoachSeen(ctx, owner)
	require.NoError(t, err)
	require.False(t, seen, "chưa mark ⇒ false (không row)")

	require.NoError(t, r.MarkCoachSeen(ctx, owner))
	require.NoError(t, r.MarkCoachSeen(ctx, owner), "upsert idempotent")

	seen, err = r.CoachSeen(ctx, owner)
	require.NoError(t, err)
	require.True(t, seen)
}
