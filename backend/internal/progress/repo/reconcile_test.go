package repo

import (
	"context"
	"testing"
	"time"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/progress/domain"
	"github.com/memorix/memorix/internal/progress/service"
)

// TestReconcile_RebuildsDailyStatsFromLogs chứng minh reconcile job (Task 9) rebuild
// progress.daily_stats + study_profiles từ nguồn chân lý review_logs (AD-4): drift được
// chữa (hàng bogus không có log bị xoá), các ngày có log được dựng lại chính xác.
func TestReconcile_RebuildsDailyStatsFromLogs(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	r := New(pool)
	owner := "55555555-5555-5555-5555-555555555555"

	// Nguồn chân lý: cùng card "a", 2 ngày liên tiếp, cả hai retained.
	//   day 7: good / 10d (≥ N=7 → retained), lần ôn đầu → new_done.
	//   day 8: easy / 20d (retained).
	card := "aaaaaaaa-0000-0000-0000-00000000000a"
	seedLog(t, ctx, pool, card, owner, domain.GradeGood, 10, time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC))
	seedLog(t, ctx, pool, card, owner, domain.GradeEasy, 20, time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC))

	// Drift: một hàng daily_stats bogus cho ngày KHÔNG có log → reconcile phải xoá nó.
	bogus := domain.Day{Year: 2026, Month: 7, Day: 9}
	if err := r.BumpDailyStat(ctx, owner, bogus, true, domain.GradeAgain, false); err != nil {
		t.Fatalf("seed drift: %v", err)
	}

	rc := service.NewReconciler(r, service.UTCResolver{})
	if err := rc.ReconcileAll(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// daily_stats phải đúng bằng 2 ngày (7, 8); ngày bogus 9 đã bị xoá.
	var nRows int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM progress.daily_stats WHERE user_id = $1`, owner).Scan(&nRows); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if nRows != 2 {
		t.Fatalf("daily_stats rebuilt = %d ngày, want 2 (bogus day phải bị xoá)", nRows)
	}

	// Đọc thẳng bảng để xác nhận mọi cột phái sinh được dựng lại đúng.
	readDay := func(day string) (rev, nw, ret, again, hard, good, easy int) {
		if err := pool.QueryRow(ctx,
			`SELECT reviews_done, new_done, retained, again, hard, good, easy
			 FROM progress.daily_stats WHERE user_id = $1 AND day = $2`,
			owner, day).Scan(&rev, &nw, &ret, &again, &hard, &good, &easy); err != nil {
			t.Fatalf("read %s: %v", day, err)
		}
		return
	}
	if rev, nw, ret, _, _, good, _ := readDay("2026-07-07"); rev != 1 || nw != 1 || ret != 1 || good != 1 {
		t.Errorf("day7 = reviews %d/new %d/retained %d/good %d, want 1/1/1/1", rev, nw, ret, good)
	}
	if rev, nw, ret, _, _, _, easy := readDay("2026-07-08"); rev != 1 || nw != 0 || ret != 1 || easy != 1 {
		t.Errorf("day8 = reviews %d/new %d/retained %d/easy %d, want 1/0/1/1", rev, nw, ret, easy)
	}

	// study_profile dựng từ chuỗi daily_stats: 2 ngày liên tiếp → streak 2, retained 2.
	p, found, err := r.GetStudyProfile(ctx, owner)
	if err != nil || !found {
		t.Fatalf("get profile: found=%v err=%v", found, err)
	}
	last := domain.Day{Year: 2026, Month: 7, Day: 8}
	if p.StreakCurrent != 2 || p.StreakBest != 2 || p.TotalReviews != 2 || p.TotalRetained != 2 ||
		p.LastStudyDate == nil || *p.LastStudyDate != last {
		t.Errorf("profile = %+v, want streak 2/2, reviews 2, retained 2, last %v", p, last)
	}
}
