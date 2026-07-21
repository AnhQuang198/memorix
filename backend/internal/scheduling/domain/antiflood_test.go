package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// overdueSet dựng n thẻ overdue với stability tăng dần ⇒ R tăng dần ⇒ thẻ index nhỏ
// có R thấp = cấp thiết hơn. DueAt/LastReviewAt là con trỏ (Card thật, AD-6).
func overdueSet(n int, now time.Time) []Card {
	due := now.AddDate(0, 0, -5)
	last := now.AddDate(0, 0, -6)
	cards := make([]Card, n)
	for i := 0; i < n; i++ {
		d, l := due, last
		cards[i] = Card{
			ID:           uuid.New(),
			Status:       StatusReview,
			Stability:    float64(i + 1),
			DueAt:        &d,
			LastReviewAt: &l,
		}
	}
	return cards
}

func TestPlanAntiFlood_300OverdueAfter5Days_UnderCap(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyReviewLimit: 200} // cap = 2×200 = 400
	overdue := overdueSet(300, now)

	plan := PlanAntiFlood(overdue, prefs, now)
	// 300 ≤ 400 ⇒ tất cả hiển thị hôm nay, không rải; và sắp R thấp trước.
	if len(plan.Today) != 300 {
		t.Fatalf("Today = %d, want 300 (dưới cap)", len(plan.Today))
	}
	if len(plan.Deferred) != 0 {
		t.Errorf("Deferred = %d, want 0", len(plan.Deferred))
	}
	if Retrievability(plan.Today[0], now) > Retrievability(plan.Today[len(plan.Today)-1], now) {
		t.Error("Today phải sắp theo R tăng dần (R thấp nhất đứng đầu)")
	}
}

func TestPlanAntiFlood_SpreadsRemainderOver7Days(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyReviewLimit: 200} // cap 400
	overdue := overdueSet(1000, now)

	plan := PlanAntiFlood(overdue, prefs, now)
	if len(plan.Today) != 400 {
		t.Fatalf("Today = %d, want 400 (cap)", len(plan.Today))
	}
	if len(plan.Deferred) != 600 {
		t.Fatalf("Deferred = %d, want 600", len(plan.Deferred))
	}
	// mọi deferred rơi vào 1..7 ngày sau đầu ngày học, không dồn 1 ngày.
	dayStart, _ := StartOfStudyDay(now, prefs.Timezone)
	minDue, maxDue := plan.Deferred[0].NewDueAt, plan.Deferred[0].NewDueAt
	for _, d := range plan.Deferred {
		off := int(d.NewDueAt.Sub(dayStart).Hours() / 24)
		if off < 1 || off > AntiFloodSpreadDays {
			t.Errorf("deferred offset %d ngoài [1,%d]", off, AntiFloodSpreadDays)
		}
		if d.NewDueAt.Before(minDue) {
			minDue = d.NewDueAt
		}
		if d.NewDueAt.After(maxDue) {
			maxDue = d.NewDueAt
		}
	}
	if minDue.Equal(maxDue) {
		t.Error("phần dư phải rải nhiều ngày, không dồn 1 ngày")
	}
	// thẻ R thấp nhất trong phần dư được xếp ngày sớm nhất.
	if !plan.Deferred[0].NewDueAt.Equal(minDue) {
		t.Error("phần dư R thấp nhất phải vào ngày sớm nhất")
	}
}
