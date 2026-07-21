package domain

import (
	"testing"
	"time"
)

func TestDayOf_UsesLocation(t *testing.T) {
	// 2026-07-07T23:30Z là 2026-07-08 06:30 ở Asia/Ho_Chi_Minh (UTC+7).
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	d := DayOf(time.Date(2026, 7, 7, 23, 30, 0, 0, time.UTC), loc)
	if d != (Day{2026, 7, 8}) {
		t.Errorf("DayOf = %v, want 2026-07-08", d)
	}
}

func TestDaysBetween(t *testing.T) {
	a := Day{2026, 7, 7}
	if got := DaysBetween(a, Day{2026, 7, 8}); got != 1 {
		t.Errorf("consecutive = %d, want 1", got)
	}
	if got := DaysBetween(a, Day{2026, 7, 10}); got != 3 {
		t.Errorf("gap = %d, want 3", got)
	}
	if got := DaysBetween(a, a); got != 0 {
		t.Errorf("same = %d, want 0", got)
	}
}

func TestDay_String(t *testing.T) {
	if got := (Day{2026, 7, 8}).String(); got != "2026-07-08" {
		t.Errorf("String = %q", got)
	}
}
