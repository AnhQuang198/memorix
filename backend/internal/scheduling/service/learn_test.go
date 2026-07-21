package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/scheduling/domain"
)

type fakeProfiles struct {
	seen   bool
	marked bool
}

func (f *fakeProfiles) CoachSeen(_ context.Context, _ uuid.UUID) (bool, error) { return f.seen, nil }

func (f *fakeProfiles) MarkCoachSeen(_ context.Context, _ uuid.UUID) error {
	f.marked = true
	return nil
}

func TestLearnService_NewCardsUpToRemainingLimit(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := domain.SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	var cards []domain.Card
	for i := 0; i < 50; i++ {
		cards = append(cards, domain.Card{ID: uuid.New(), Status: domain.StatusNew, CreatedAt: now.Add(-time.Duration(i) * time.Minute)})
	}
	due := now.Add(-time.Hour)
	cards = append(cards, domain.Card{ID: uuid.New(), Status: domain.StatusReview, DueAt: &due}) // không phải new
	prof := &fakeProfiles{seen: false}
	svc := NewLearnService(&fakeCardRepo{cards: cards}, fakeQueuePrefs{p: prefs}, fakeActivity{counts: domain.DayCounts{NewServed: 5}}, prof)

	sess, err := svc.StartSession(context.Background(), uuid.New(), now)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(sess.Cards) != 15 { // 20 - 5 đã học hôm nay
		t.Errorf("new session = %d thẻ, want 15", len(sess.Cards))
	}
	for _, c := range sess.Cards {
		if c.Status != domain.StatusNew {
			t.Errorf("chỉ được trả thẻ New, got status %v", c.Status)
		}
	}
	if !sess.ShowCoach {
		t.Error("lần đầu phải ShowCoach=true")
	}
}

func TestLearnService_AckHidesCoach(t *testing.T) {
	prof := &fakeProfiles{seen: false}
	svc := NewLearnService(&fakeCardRepo{}, fakeQueuePrefs{}, fakeActivity{}, prof)
	if err := svc.AckCoach(context.Background(), uuid.New()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !prof.marked {
		t.Error("AckCoach phải gọi MarkCoachSeen")
	}
}

func TestLearnService_CoachHiddenWhenSeen(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := domain.SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	svc := NewLearnService(&fakeCardRepo{}, fakeQueuePrefs{p: prefs}, fakeActivity{}, &fakeProfiles{seen: true})
	sess, _ := svc.StartSession(context.Background(), uuid.New(), now)
	if sess.ShowCoach {
		t.Error("đã xem coach ⇒ ShowCoach=false")
	}
}
