package repo

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/progress/domain"
)

// seedCard chèn 1 card tối thiểu vào scheduling.cards (schema Sprint 2/3: 0004+0006).
// entry_id là ref logic (không FK chéo schema, AD-10) nên random uuid là đủ.
func seedCard(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id, owner string, due time.Time) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO scheduling.cards
		(id, owner_id, entry_id, direction, stability, difficulty, status, reps, lapses, due_at, created_at, updated_at)
		VALUES ($1, $2, gen_random_uuid(), 'front_back', 1.0, 5.0, 'review', 1, 0, $3, now(), now())`,
		id, owner, due)
	if err != nil {
		t.Fatalf("seed card: %v", err)
	}
}

// seedLog chèn 1 review_log tối thiểu vào review.review_logs (schema 0006). scheduled_days
// KHÔNG là cột rời — WeekRetentionLogs suy nó từ (new_due_at::date - reviewed_at::date),
// nên đặt new_due_at = reviewed_at + sched ngày để ép "lịch kế" mong muốn.
func seedLog(t *testing.T, ctx context.Context, pool *pgxpool.Pool, card, owner string, grade, sched int, when time.Time) {
	t.Helper()
	newDue := when.Add(time.Duration(sched) * 24 * time.Hour)
	_, err := pool.Exec(ctx, `INSERT INTO review.review_logs
		(id, card_id, owner_id, client_review_id, grade,
		 prev_stability, prev_difficulty, prev_status, retrievability,
		 new_stability, new_difficulty, new_status, new_reps, new_lapses, new_due_at,
		 elapsed_days, reviewed_at)
		VALUES (gen_random_uuid(), $1, $2, gen_random_uuid()::text, $3,
		 1.0, 5.0, 2, 0.9,
		 2.0, 5.0, 2, 1, 0, $4,
		 1, $5)`,
		card, owner, grade, newDue, when)
	if err != nil {
		t.Fatalf("seed log: %v", err)
	}
}

// TestRepo_WeekRetentionLogs_And_DueForecast chứng minh 3 read wrapper "nóng":
// North Star đọc thẳng review_logs (AD-8), DueCount đếm card quá hạn, Forecast gom
// due-per-day tương lai. due_at nullable → wrapper phải bọc pgtype.Timestamptz.
func TestRepo_WeekRetentionLogs_And_DueForecast(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	r := New(pool)
	owner := "33333333-3333-3333-3333-333333333333"

	// 2 log trong tuần: c1 retained (good/10d ≥ N=7), c2 không (hard/3d < 7).
	seedLog(t, ctx, pool, "aaaaaaaa-0000-0000-0000-000000000001", owner, domain.GradeGood, 10, time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC))
	seedLog(t, ctx, pool, "aaaaaaaa-0000-0000-0000-000000000002", owner, domain.GradeHard, 3, time.Date(2026, 7, 8, 11, 0, 0, 0, time.UTC))

	logs, err := r.WeekRetentionLogs(ctx, owner, time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("week: %v", err)
	}
	if domain.CountWordsRetained(logs) != 1 {
		t.Errorf("North Star = %d, want 1", domain.CountWordsRetained(logs))
	}

	// Forecast: 1 card due ngày mai, 1 card đã quá hạn (chỉ tính vào DueCount, không vào forecast tương lai).
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	seedCard(t, ctx, pool, "cccccccc-0000-0000-0000-000000000001", owner, now.Add(24*time.Hour)) // mai
	seedCard(t, ctx, pool, "cccccccc-0000-0000-0000-000000000002", owner, now.Add(-2*time.Hour))  // overdue

	due, err := r.DueCount(ctx, owner, now)
	if err != nil {
		t.Fatalf("due: %v", err)
	}
	if due != 1 {
		t.Errorf("DueCount = %d, want 1 (overdue)", due)
	}
	fc, err := r.Forecast(ctx, owner, now, now.Add(7*24*time.Hour), "UTC")
	if err != nil {
		t.Fatalf("forecast: %v", err)
	}
	if fc["2026-07-09"] != 1 {
		t.Errorf("forecast mai = %d, want 1 (map=%v)", fc["2026-07-09"], fc)
	}
}

// TestRepo_Heatmap_Distribution_TodayStat chứng minh 3 read wrapper trên read model
// progress.daily_stats: seed qua BumpDailyStat (write side Task 6) rồi đọc lại.
func TestRepo_Heatmap_Distribution_TodayStat(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	r := New(pool)
	u := "44444444-4444-4444-4444-444444444444"
	day := domain.Day{Year: 2026, Month: 7, Day: 8}

	// TodayStat khi chưa có hàng → 0/0 (pgx.ErrNoRows nuốt thành zero).
	if rev, nw, err := r.TodayStat(ctx, u, day); err != nil || rev != 0 || nw != 0 {
		t.Fatalf("TodayStat rỗng = %d/%d err=%v, want 0/0", rev, nw, err)
	}

	// 3 review cùng ngày: good(new,retained), easy(retained), again(không).
	if err := r.BumpDailyStat(ctx, u, day, true, domain.GradeGood, true); err != nil {
		t.Fatalf("bump good: %v", err)
	}
	if err := r.BumpDailyStat(ctx, u, day, false, domain.GradeEasy, true); err != nil {
		t.Fatalf("bump easy: %v", err)
	}
	if err := r.BumpDailyStat(ctx, u, day, false, domain.GradeAgain, false); err != nil {
		t.Fatalf("bump again: %v", err)
	}

	rev, nw, err := r.TodayStat(ctx, u, day)
	if err != nil {
		t.Fatalf("today: %v", err)
	}
	if rev != 3 || nw != 1 {
		t.Errorf("TodayStat = %d/%d, want 3/1", rev, nw)
	}

	from := domain.Day{Year: 2026, Month: 7, Day: 1}
	to := domain.Day{Year: 2026, Month: 7, Day: 31}

	hm, err := r.Heatmap(ctx, u, from, to)
	if err != nil {
		t.Fatalf("heatmap: %v", err)
	}
	if len(hm) != 1 {
		t.Fatalf("Heatmap len = %d, want 1 (%+v)", len(hm), hm)
	}
	if hm[0].Day != day || hm[0].ReviewsDone != 3 || hm[0].Retained != 2 {
		t.Errorf("Heatmap[0] = %+v, want day=%v reviews=3 retained=2", hm[0], day)
	}

	again, hard, good, easy, err := r.Distribution(ctx, u, from, to)
	if err != nil {
		t.Fatalf("distribution: %v", err)
	}
	if again != 1 || hard != 0 || good != 1 || easy != 1 {
		t.Errorf("Distribution = again %d/hard %d/good %d/easy %d, want 1/0/1/1", again, hard, good, easy)
	}
}
