package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/platform/eventbus"
	revdom "github.com/memorix/memorix/internal/review/domain"
	revrepo "github.com/memorix/memorix/internal/review/repo"
	"github.com/memorix/memorix/internal/review/service"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/repo"
	"github.com/memorix/memorix/internal/scheduling/repo/fsrsadapter"
	"github.com/stretchr/testify/require"
)

func insertNewCard(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner, entry uuid.UUID, due time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO scheduling.cards (id, owner_id, entry_id, direction, status, due_at, created_at, updated_at)
		VALUES ($1,$2,$3,'front_back','new',$4,now(),now())`, id, owner, entry, due)
	require.NoError(t, err)
	return id
}

func realService(pool *pgxpool.Pool, bus *eventbus.InProcess, now time.Time) *service.GradeService {
	return service.NewGradeService(service.GradeDeps{
		Tx:        func(ctx context.Context, fn func(db.Querier) error) error { return db.WithinTx(ctx, pool, fn) },
		Scheduler: fsrsadapter.New(),
		Cards:     repo.NewCardStore(),
		Prefs:     repo.NewPrefsStore(),
		Logs:      revrepo.NewReviewLogRepo(),
		Receipts:  revrepo.NewReceiptRepo(),
		Bus:       bus,
		Clock:     func() time.Time { return now },
	})
}

func TestGrade_AtomicIdempotentOnPostgres(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	owner, entry := uuid.New(), uuid.New()
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	cardID := insertNewCard(t, ctx, pool, owner, entry, now)

	svc := realService(pool, eventbus.NewInProcess(), now)
	cmd := revdom.GradeCommand{CardID: cardID, Grade: scheddom.GradeGood, ClientReviewID: "cr-1"}

	r1, err := svc.Grade(ctx, owner, cmd)
	require.NoError(t, err)
	require.Greater(t, r1.Stability, 0.0)

	r2, err := svc.Grade(ctx, owner, cmd) // idempotent
	require.NoError(t, err)
	// r2 đọc lại từ grade_receipts (pgx trả timestamptz theo time.Local) nên so từng
	// trường + so DueAt theo cùng-thời-điểm để bền vững với TZ máy chạy test.
	require.Equal(t, r1.CardID, r2.CardID)
	require.InDelta(t, r1.Stability, r2.Stability, 1e-9)
	require.InDelta(t, r1.Difficulty, r2.Difficulty, 1e-9)
	require.Equal(t, r1.Status, r2.Status)
	require.Equal(t, r1.Reps, r2.Reps)
	require.Equal(t, r1.Lapses, r2.Lapses)
	require.True(t, r1.DueAt.Equal(r2.DueAt), "idempotent DueAt phải cùng thời điểm")

	var logCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM review.review_logs WHERE card_id=$1`, cardID).Scan(&logCount))
	require.Equal(t, 1, logCount, "chỉ 1 log dù chấm 2 lần")

	// card đã cập nhật đúng theo kết quả server-authoritative
	var dbStab float64
	require.NoError(t, pool.QueryRow(ctx, `SELECT stability FROM scheduling.cards WHERE id=$1`, cardID).Scan(&dbStab))
	require.InDelta(t, r1.Stability, dbStab, 1e-9)
}

// TestReplay_ReproducesCardState: replay chuỗi review_logs từ NewCard qua SchedulerPort
// phải tái tạo đúng S/D/Due/status/reps/lapses của card hiện tại (AD-4, NFR-6).
func TestReplay_ReproducesCardState(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	owner, entry := uuid.New(), uuid.New()
	start := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	cardID := insertNewCard(t, ctx, pool, owner, entry, start)

	// chấm nhiều lần ở các mốc thời gian khác nhau (mô phỏng ôn thật).
	grades := []struct {
		g  scheddom.Grade
		at time.Time
	}{
		{scheddom.GradeGood, start},
		{scheddom.GradeGood, start.AddDate(0, 0, 3)},
		{scheddom.GradeAgain, start.AddDate(0, 0, 10)},
		{scheddom.GradeHard, start.AddDate(0, 0, 11)},
		{scheddom.GradeEasy, start.AddDate(0, 0, 20)},
	}
	for i, step := range grades {
		svc := realService(pool, eventbus.NewInProcess(), step.at)
		_, err := svc.Grade(ctx, owner, revdom.GradeCommand{
			CardID: cardID, Grade: step.g, ClientReviewID: uuidStr(i),
		})
		require.NoError(t, err)
	}

	// đọc trạng thái card hiện tại
	cs := repo.NewCardStore()
	final, err := cs.Load(ctx, pool, cardID, owner)
	require.NoError(t, err)

	// REPLAY: dựng lại từ NewCard, áp từng grade theo reviewed_at.
	logs, err := revrepo.NewReviewLogRepo().ListForCard(ctx, pool, cardID)
	require.NoError(t, err)
	require.Len(t, logs, len(grades))

	sched := fsrsadapter.New()
	replayed := scheddom.Card{ID: cardID, OwnerID: owner, EntryID: entry, Status: scheddom.StatusNew, DueAt: &start}
	for _, lg := range logs {
		out := sched.Apply(replayed, lg.Grade, 0.90, lg.ReviewedAt)
		due := out.DueAt
		last := out.LastReviewAt
		replayed.Stability = out.Stability
		replayed.Difficulty = out.Difficulty
		replayed.Status = out.Status
		replayed.Reps = out.Reps
		replayed.Lapses = out.Lapses
		replayed.DueAt = &due
		replayed.LastReviewAt = &last
	}

	require.NotNil(t, final.DueAt)
	require.NotNil(t, replayed.DueAt)
	require.InDelta(t, final.Stability, replayed.Stability, 1e-9, "replay S phải khớp")
	require.InDelta(t, final.Difficulty, replayed.Difficulty, 1e-9, "replay D phải khớp")
	require.Equal(t, final.Status, replayed.Status)
	require.Equal(t, final.Reps, replayed.Reps)
	require.Equal(t, final.Lapses, replayed.Lapses)
	require.WithinDuration(t, *final.DueAt, *replayed.DueAt, time.Second, "replay Due phải khớp (fuzz TẮT)")
}

func uuidStr(i int) string {
	return "replay-cr-" + time.Unix(int64(i), 0).UTC().Format("150405")
}
