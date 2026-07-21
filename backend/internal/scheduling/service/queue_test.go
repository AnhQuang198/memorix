package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/scheduling/domain"
)

type fakeCardRepo struct {
	cards    []domain.Card
	deferred []domain.DeferredCard
}

func (f *fakeCardRepo) LoadCandidates(_ context.Context, _ uuid.UUID, _ time.Time) ([]domain.Card, error) {
	return f.cards, nil
}

func (f *fakeCardRepo) BulkDefer(_ context.Context, d []domain.DeferredCard) error {
	f.deferred = append(f.deferred, d...)
	return nil
}

type fakeQueuePrefs struct{ p domain.SchedulerPrefs }

func (f fakeQueuePrefs) Get(_ context.Context, _ uuid.UUID) (domain.SchedulerPrefs, error) {
	return f.p, nil
}

func (f fakeQueuePrefs) UpdateLimits(_ context.Context, _ uuid.UUID, n, r int) (domain.SchedulerPrefs, error) {
	f.p.DailyNewLimit, f.p.DailyReviewLimit = n, r
	return f.p, nil
}

type fakeActivity struct{ counts domain.DayCounts }

func (f fakeActivity) CountServedSince(_ context.Context, _ uuid.UUID, _ time.Time) (domain.DayCounts, error) {
	return f.counts, nil
}

func TestQueueService_SubtractsServedToday(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := domain.SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	due := now.Add(-time.Hour)
	last := now.Add(-48 * time.Hour)
	var cards []domain.Card
	for i := 0; i < 30; i++ {
		d, l := due, last
		cards = append(cards, domain.Card{ID: uuid.New(), Status: domain.StatusReview, Stability: 5, DueAt: &d, LastReviewAt: &l})
	}
	repo := &fakeCardRepo{cards: cards}
	svc := NewQueueService(repo, fakeQueuePrefs{p: prefs}, fakeActivity{counts: domain.DayCounts{ReviewServed: 195}})

	res, err := svc.BuildToday(context.Background(), uuid.New(), now)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// còn 200-195 = 5 review được phục vụ hôm nay.
	if res.ReviewCount != 5 {
		t.Errorf("ReviewCount = %d, want 5 (200-195 served)", res.ReviewCount)
	}
}

func TestQueueService_TriggersAntiFloodAndDefers(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := domain.SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	// 1000 overdue (due trước đầu ngày) ⇒ > 2×200 ⇒ rải bớt.
	due := now.AddDate(0, 0, -3)
	last := now.AddDate(0, 0, -4)
	var cards []domain.Card
	for i := 0; i < 1000; i++ {
		d, l := due, last
		cards = append(cards, domain.Card{ID: uuid.New(), Status: domain.StatusReview, Stability: float64(i + 1), DueAt: &d, LastReviewAt: &l})
	}
	repo := &fakeCardRepo{cards: cards}
	svc := NewQueueService(repo, fakeQueuePrefs{p: prefs}, fakeActivity{})

	res, err := svc.BuildToday(context.Background(), uuid.New(), now)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(repo.deferred) != 600 {
		t.Errorf("BulkDefer nhận %d, want 600 (1000-400)", len(repo.deferred))
	}
	// sau khi rải, queue hôm nay còn ≤ review limit (200).
	if res.ReviewCount != 200 {
		t.Errorf("ReviewCount = %d, want 200", res.ReviewCount)
	}
}
