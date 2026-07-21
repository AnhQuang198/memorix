package domain

import "testing"

func dayPtr(d Day) *Day { return &d }

func TestApplyStudyDay_FirstRetainedStartsStreak(t *testing.T) {
	got := ApplyStudyDay(StudyProfile{}, Day{2026, 7, 7}, 1, 1)
	if got.StreakCurrent != 1 || got.StreakBest != 1 {
		t.Errorf("streak = %d/%d, want 1/1", got.StreakCurrent, got.StreakBest)
	}
	if got.LastStudyDate == nil || *got.LastStudyDate != (Day{2026, 7, 7}) {
		t.Errorf("last = %v", got.LastStudyDate)
	}
	if got.TotalReviews != 1 || got.TotalRetained != 1 {
		t.Errorf("totals = %d/%d, want 1/1", got.TotalReviews, got.TotalRetained)
	}
}

func TestApplyStudyDay_ConsecutiveIncrements(t *testing.T) {
	p := StudyProfile{StreakCurrent: 3, StreakBest: 3, LastStudyDate: dayPtr(Day{2026, 7, 7}), TotalRetained: 10}
	got := ApplyStudyDay(p, Day{2026, 7, 8}, 1, 1)
	if got.StreakCurrent != 4 || got.StreakBest != 4 {
		t.Errorf("streak = %d/%d, want 4/4", got.StreakCurrent, got.StreakBest)
	}
}

func TestApplyStudyDay_MissedDayResets_ButRetainedNotReset(t *testing.T) {
	p := StudyProfile{StreakCurrent: 9, StreakBest: 9, LastStudyDate: dayPtr(Day{2026, 7, 7}), TotalRetained: 100}
	got := ApplyStudyDay(p, Day{2026, 7, 10}, 1, 1) // gap 3 ngày
	if got.StreakCurrent != 1 {
		t.Errorf("streak reset = %d, want 1", got.StreakCurrent)
	}
	if got.StreakBest != 9 {
		t.Errorf("best = %d, want 9 (giữ)", got.StreakBest)
	}
	if got.TotalRetained != 101 {
		t.Errorf("total_retained = %d, want 101 (KHÔNG reset, cộng dồn)", got.TotalRetained)
	}
}

func TestApplyStudyDay_SameDaySecondEvent_NoStreakChange(t *testing.T) {
	p := StudyProfile{StreakCurrent: 2, StreakBest: 2, LastStudyDate: dayPtr(Day{2026, 7, 8}), TotalReviews: 5, TotalRetained: 3}
	got := ApplyStudyDay(p, Day{2026, 7, 8}, 1, 1)
	if got.StreakCurrent != 2 {
		t.Errorf("streak = %d, want 2 (không đổi cùng ngày)", got.StreakCurrent)
	}
	if got.TotalReviews != 6 || got.TotalRetained != 4 {
		t.Errorf("totals = %d/%d, want 6/4", got.TotalReviews, got.TotalRetained)
	}
}

func TestApplyStudyDay_NonRetained_NoStreakNoLastDate(t *testing.T) {
	p := StudyProfile{StreakCurrent: 5, StreakBest: 5, LastStudyDate: dayPtr(Day{2026, 7, 6}), TotalReviews: 20, TotalRetained: 12}
	got := ApplyStudyDay(p, Day{2026, 7, 8}, 1, 0) // ôn nhưng không nhớ được
	if got.StreakCurrent != 5 || *got.LastStudyDate != (Day{2026, 7, 6}) {
		t.Errorf("ngày không recall thật không được đụng streak/last: %+v", got)
	}
	if got.TotalReviews != 21 || got.TotalRetained != 12 {
		t.Errorf("totals = %d/%d, want 21/12", got.TotalReviews, got.TotalRetained)
	}
}

func TestRebuildStudyProfile_FoldsDays(t *testing.T) {
	stats := []DailyStat{
		{Day: Day{2026, 7, 7}, ReviewsDone: 4, Retained: 2},
		{Day: Day{2026, 7, 8}, ReviewsDone: 3, Retained: 1}, // liên tiếp → streak 2
		{Day: Day{2026, 7, 12}, ReviewsDone: 5, Retained: 3}, // gap → reset 1
	}
	got := RebuildStudyProfile(stats)
	if got.StreakCurrent != 1 || got.StreakBest != 2 {
		t.Errorf("streak = %d/%d, want 1/2", got.StreakCurrent, got.StreakBest)
	}
	if got.TotalReviews != 12 || got.TotalRetained != 6 {
		t.Errorf("totals = %d/%d, want 12/6", got.TotalReviews, got.TotalRetained)
	}
}
