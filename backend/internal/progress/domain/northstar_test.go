package domain

import "testing"

func TestCountWordsRetained(t *testing.T) {
	logs := []RetentionLog{
		{CardID: "a", Grade: 3, ScheduledDays: 10}, // retained
		{CardID: "a", Grade: 4, ScheduledDays: 20}, // same card, không đếm lại
		{CardID: "b", Grade: 2, ScheduledDays: 7},  // retained (biên =7)
		{CardID: "c", Grade: 3, ScheduledDays: 6},  // interval < 7 → không
		{CardID: "d", Grade: 1, ScheduledDays: 30}, // Again → không
	}
	if got := CountWordsRetained(logs); got != 2 {
		t.Errorf("CountWordsRetained = %d, want 2 (a,b distinct)", got)
	}
}

func TestIsRetained_Boundaries(t *testing.T) {
	cases := []struct {
		grade, days int
		want        bool
	}{
		{2, 7, true}, {3, 7, true}, {4, 7, true},
		{2, 6, false}, {1, 30, false}, {3, 100, true},
	}
	for _, c := range cases {
		if got := IsRetained(c.grade, c.days); got != c.want {
			t.Errorf("IsRetained(%d,%d)=%v want %v", c.grade, c.days, got, c.want)
		}
	}
}
