package repo

import (
	"context"
	"testing"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/progress/domain"
)

// TestRepo_BumpDailyStat_Accumulates chứng minh BumpDailyStat upsert ON CONFLICT
// cộng dồn counters cho cùng (user, day): 2 lần bump → reviews_done=2, new_done=1
// (chỉ lần đầu wasNew), phân bố grade cộng đúng từng cột.
func TestRepo_BumpDailyStat_Accumulates(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	u := "11111111-1111-1111-1111-111111111111"
	day := domain.Day{Year: 2026, Month: 7, Day: 8}

	if err := r.BumpDailyStat(ctx, u, day, true, domain.GradeGood, true); err != nil {
		t.Fatalf("bump1: %v", err)
	}
	if err := r.BumpDailyStat(ctx, u, day, false, domain.GradeAgain, false); err != nil {
		t.Fatalf("bump2: %v", err)
	}

	var reviews, newDone, retained, again, good int
	err := pool.QueryRow(ctx,
		`SELECT reviews_done, new_done, retained, again, good
		 FROM progress.daily_stats WHERE user_id = $1 AND day = $2`,
		u, day.String()).Scan(&reviews, &newDone, &retained, &again, &good)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if reviews != 2 || newDone != 1 || retained != 1 || again != 1 || good != 1 {
		t.Errorf("accumulated = reviews %d / new %d / retained %d / again %d / good %d, want 2/1/1/1/1",
			reviews, newDone, retained, again, good)
	}
}

// TestRepo_StudyProfile_RoundTrip chứng minh Get (miss → found=false), Upsert insert,
// Upsert lại (ON CONFLICT) ghi đè tại chỗ; last_study_date nullable round-trip đúng.
func TestRepo_StudyProfile_RoundTrip(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	u := "22222222-2222-2222-2222-222222222222"

	if _, found, err := r.GetStudyProfile(ctx, u); err != nil || found {
		t.Fatalf("mới phải không tồn tại: found=%v err=%v", found, err)
	}

	last := domain.Day{Year: 2026, Month: 7, Day: 8}
	p := domain.StudyProfile{StreakCurrent: 3, StreakBest: 5, LastStudyDate: &last, TotalReviews: 20, TotalRetained: 12}
	if err := r.UpsertStudyProfile(ctx, u, p); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, found, err := r.GetStudyProfile(ctx, u)
	if err != nil || !found {
		t.Fatalf("get: found=%v err=%v", found, err)
	}
	if got.StreakCurrent != 3 || got.StreakBest != 5 || got.TotalReviews != 20 || got.TotalRetained != 12 ||
		got.LastStudyDate == nil || *got.LastStudyDate != last {
		t.Errorf("round-trip mismatch: %+v", got)
	}

	// Upsert lại cùng user → ON CONFLICT (user_id) DO UPDATE ghi đè.
	p2 := domain.StudyProfile{StreakCurrent: 4, StreakBest: 5, LastStudyDate: &last, TotalReviews: 25, TotalRetained: 15}
	if err := r.UpsertStudyProfile(ctx, u, p2); err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	got2, _, err := r.GetStudyProfile(ctx, u)
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if got2.StreakCurrent != 4 || got2.TotalReviews != 25 || got2.TotalRetained != 15 {
		t.Errorf("upsert conflict update mismatch: %+v", got2)
	}
}
