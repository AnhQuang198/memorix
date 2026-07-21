package domain

import (
	"testing"
	"time"
)

func TestRebuildDailyStats_AggregatesByDayAndCountsNew(t *testing.T) {
	loc := time.UTC
	ts := func(d, h int) time.Time { return time.Date(2026, 7, d, h, 0, 0, 0, time.UTC) }
	logs := []LogRow{
		{CardID: "a", Grade: 3, ScheduledDays: 10, ReviewedAt: ts(7, 8)}, // day7: new(a), retained
		{CardID: "b", Grade: 1, ScheduledDays: 0, ReviewedAt: ts(7, 9)},  // day7: new(b), again
		{CardID: "a", Grade: 4, ScheduledDays: 30, ReviewedAt: ts(8, 8)}, // day8: a lại (không new), retained
		{CardID: "b", Grade: 2, ScheduledDays: 6, ReviewedAt: ts(8, 9)},  // day8: b lại, hard, interval<7 → không retained
	}
	got := RebuildDailyStats(logs, loc)
	if len(got) != 2 {
		t.Fatalf("days = %d, want 2", len(got))
	}
	d7 := got[0]
	if d7.Day != (Day{2026, 7, 7}) || d7.ReviewsDone != 2 || d7.NewDone != 2 || d7.Retained != 1 || d7.Good != 1 || d7.Again != 1 {
		t.Errorf("day7 = %+v", d7)
	}
	d8 := got[1]
	if d8.Day != (Day{2026, 7, 8}) || d8.ReviewsDone != 2 || d8.NewDone != 0 || d8.Retained != 1 || d8.Easy != 1 || d8.Hard != 1 {
		t.Errorf("day8 = %+v", d8)
	}
}

func TestRebuildDailyStats_Empty(t *testing.T) {
	if got := RebuildDailyStats(nil, time.UTC); len(got) != 0 {
		t.Errorf("empty → %d rows", len(got))
	}
}
