package service

import (
	"context"
	"testing"
	"time"

	"github.com/memorix/memorix/internal/progress/domain"
	"github.com/memorix/memorix/internal/shared/events"
)

// fakeIngestRepo lưu trạng thái trong bộ nhớ để kiểm chứng ghi read model.
type fakeIngestRepo struct {
	bumps    []bumpCall
	profiles map[string]domain.StudyProfile
}
type bumpCall struct {
	owner    string
	day      domain.Day
	wasNew   bool
	grade    int
	retained bool
}

func newFakeIngestRepo() *fakeIngestRepo {
	return &fakeIngestRepo{profiles: map[string]domain.StudyProfile{}}
}
func (f *fakeIngestRepo) BumpDailyStat(_ context.Context, owner string, day domain.Day, wasNew bool, grade int, retained bool) error {
	f.bumps = append(f.bumps, bumpCall{owner, day, wasNew, grade, retained})
	return nil
}
func (f *fakeIngestRepo) GetStudyProfile(_ context.Context, userID string) (domain.StudyProfile, bool, error) {
	p, ok := f.profiles[userID]
	return p, ok, nil
}
func (f *fakeIngestRepo) UpsertStudyProfile(_ context.Context, userID string, p domain.StudyProfile) error {
	f.profiles[userID] = p
	return nil
}

func TestIngest_HandleCardGraded_RetainedUpdatesStatsAndStreak(t *testing.T) {
	repo := newFakeIngestRepo()
	ing := NewIngestor(repo, UTCResolver{}, nil)
	e := events.CardGraded{
		OwnerID: "u1", CardID: "c1", Grade: domain.GradeGood, ScheduledDays: 12,
		WasNew: true, ReviewedAt: time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC),
	}
	if err := ing.HandleCardGraded(context.Background(), e); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(repo.bumps) != 1 {
		t.Fatalf("bumps = %d, want 1", len(repo.bumps))
	}
	b := repo.bumps[0]
	if b.owner != "u1" || b.day != (domain.Day{Year: 2026, Month: 7, Day: 8}) || !b.wasNew || !b.retained {
		t.Errorf("bump = %+v", b)
	}
	p := repo.profiles["u1"]
	if p.StreakCurrent != 1 || p.TotalRetained != 1 || p.TotalReviews != 1 {
		t.Errorf("profile = %+v", p)
	}
}

func TestIngest_HandleCardGraded_AgainNoStreak(t *testing.T) {
	repo := newFakeIngestRepo()
	ing := NewIngestor(repo, UTCResolver{}, nil)
	e := events.CardGraded{OwnerID: "u2", CardID: "c9", Grade: domain.GradeAgain, ScheduledDays: 0, ReviewedAt: time.Now()}
	if err := ing.HandleCardGraded(context.Background(), e); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if repo.bumps[0].retained {
		t.Error("Again không được tính retained")
	}
	p := repo.profiles["u2"]
	if p.StreakCurrent != 0 || p.TotalReviews != 1 {
		t.Errorf("profile = %+v, want streak 0 / reviews 1", p)
	}
}
