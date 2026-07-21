package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// mkCard dựng Card test theo REAL type Sprint 3 (Status string, DueAt/LastReviewAt pointer).
func mkCard(status CardStatus, dueOffsetDays float64, stability float64, created time.Time, now time.Time) Card {
	due := now.Add(time.Duration(dueOffsetDays * 24 * float64(time.Hour)))
	last := due.Add(-time.Duration(stability * 24 * float64(time.Hour)))
	return Card{
		ID:           uuid.New(),
		OwnerID:      uuid.Nil,
		Status:       status,
		Stability:    stability,
		DueAt:        &due,
		LastReviewAt: &last,
		CreatedAt:    created,
	}
}

func TestBuildQueue_PriorityOrder(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 5, DailyReviewLimit: 50}

	// overdue: due trước đầu ngày hôm nay; R thấp (stability nhỏ) phải đứng trước.
	overdueLowR := mkCard(StatusReview, -3, 1, now, now)    // R rất thấp
	overdueHighR := mkCard(StatusReview, -4, 100, now, now) // quá hạn lâu hơn nhưng S lớn ⇒ R cao hơn
	relearn := mkCard(StatusRelearning, -0.02, 2, now, now) // due ~30' trước, trong ngày
	reviewDue := mkCard(StatusReview, -0.05, 5, now, now)   // due trong hôm nay, sau đầu ngày
	newA := mkCard(StatusNew, 0, 0, now.Add(-2*time.Hour), now)
	newB := mkCard(StatusNew, 0, 0, now.Add(-1*time.Hour), now)

	res := BuildQueue([]Card{newB, reviewDue, overdueHighR, relearn, overdueLowR, newA}, prefs, now)

	order := make([]CardStatus, len(res.Cards))
	for i, c := range res.Cards {
		order[i] = c.Status
	}
	// Kỳ vọng: overdue(2) → relearning(1) → review-due(1) → new(2).
	if len(res.Cards) != 6 {
		t.Fatalf("len = %d, want 6 (%v)", len(res.Cards), order)
	}
	if res.Cards[0].ID != overdueLowR.ID {
		t.Errorf("overdue R-thấp phải đứng đầu, got %v", res.Cards[0].ID)
	}
	if res.Cards[1].ID != overdueHighR.ID {
		t.Errorf("overdue R-cao phải sau, got %v", res.Cards[1].ID)
	}
	if res.Cards[2].Status != StatusRelearning {
		t.Errorf("vị trí 2 phải relearning, got %v", res.Cards[2].Status)
	}
	if res.Cards[3].Status != StatusReview || res.Cards[3].ID != reviewDue.ID {
		t.Errorf("vị trí 3 phải review-due, got %v", res.Cards[3].ID)
	}
	if res.Cards[4].Status != StatusNew || res.Cards[5].Status != StatusNew {
		t.Errorf("2 thẻ cuối phải là new, got %v", order)
	}
	if res.Cards[4].ID != newA.ID {
		t.Errorf("new sắp theo created_at (newA trước), got %v", res.Cards[4].ID)
	}
	if res.NewCount != 2 || res.ReviewCount != 4 {
		t.Errorf("counts = new %d review %d, want 2/4", res.NewCount, res.ReviewCount)
	}
}

func TestBuildQueue_RespectsDailyLimits(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 2, DailyReviewLimit: 3}
	var cards []Card
	for i := 0; i < 10; i++ {
		cards = append(cards, mkCard(StatusReview, -1, 5, now, now))
	}
	for i := 0; i < 10; i++ {
		cards = append(cards, mkCard(StatusNew, 0, 0, now.Add(-time.Duration(i)*time.Minute), now))
	}
	res := BuildQueue(cards, prefs, now)
	if res.ReviewCount != 3 {
		t.Errorf("review cap = %d, want 3", res.ReviewCount)
	}
	if res.NewCount != 2 {
		t.Errorf("new cap = %d, want 2", res.NewCount)
	}
	if len(res.Cards) != 5 {
		t.Errorf("total = %d, want 5", len(res.Cards))
	}
}

func TestBuildQueue_ExcludesFutureAndDeleted(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	future := mkCard(StatusReview, 2, 5, now, now) // due 2 ngày tới ⇒ loại
	res := BuildQueue([]Card{future}, prefs, now)
	if len(res.Cards) != 0 {
		t.Errorf("thẻ tương lai không được vào queue, got %d", len(res.Cards))
	}
}

func TestBuildQueue_SkipsSuspended(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	suspended := mkCard(StatusSuspended, -1, 5, now, now) // due nhưng suspended ⇒ loại
	res := BuildQueue([]Card{suspended}, prefs, now)
	if len(res.Cards) != 0 {
		t.Errorf("thẻ suspended không được vào queue, got %d", len(res.Cards))
	}
}

func TestApplyDayCounts_SubtractsServed(t *testing.T) {
	prefs := SchedulerPrefs{DailyNewLimit: 20, DailyReviewLimit: 200}
	got := ApplyDayCounts(prefs, DayCounts{NewServed: 5, ReviewServed: 50})
	if got.DailyNewLimit != 15 || got.DailyReviewLimit != 150 {
		t.Errorf("effective = new %d review %d, want 15/150", got.DailyNewLimit, got.DailyReviewLimit)
	}
	// không âm khi đã vượt hạn
	got2 := ApplyDayCounts(prefs, DayCounts{NewServed: 999, ReviewServed: 999})
	if got2.DailyNewLimit != 0 || got2.DailyReviewLimit != 0 {
		t.Errorf("phải kẹp về 0, got %d/%d", got2.DailyNewLimit, got2.DailyReviewLimit)
	}
}
