package service

import (
	"context"
	"testing"
	"time"

	"github.com/memorix/memorix/internal/progress/domain"
)

type fakeReconcileRepo struct {
	owners   []string
	logs     map[string][]domain.LogRow
	replaced map[string][]domain.DailyStat
	profiles map[string]domain.StudyProfile
}

func (f *fakeReconcileRepo) DistinctOwners(context.Context) ([]string, error) { return f.owners, nil }
func (f *fakeReconcileRepo) AllLogsForOwner(_ context.Context, o string) ([]domain.LogRow, error) {
	return f.logs[o], nil
}
func (f *fakeReconcileRepo) ReplaceDailyStats(_ context.Context, o string, s []domain.DailyStat) error {
	f.replaced[o] = s
	return nil
}
func (f *fakeReconcileRepo) UpsertStudyProfile(_ context.Context, o string, p domain.StudyProfile) error {
	f.profiles[o] = p
	return nil
}

func TestReconcile_RebuildsDailyStatsAndProfile(t *testing.T) {
	ts := func(d int) time.Time { return time.Date(2026, 7, d, 10, 0, 0, 0, time.UTC) }
	repo := &fakeReconcileRepo{
		owners: []string{"u1"},
		logs: map[string][]domain.LogRow{"u1": {
			{CardID: "a", Grade: domain.GradeGood, ScheduledDays: 10, ReviewedAt: ts(7)},
			{CardID: "a", Grade: domain.GradeEasy, ScheduledDays: 20, ReviewedAt: ts(8)},
		}},
		replaced: map[string][]domain.DailyStat{},
		profiles: map[string]domain.StudyProfile{},
	}
	rc := NewReconciler(repo, UTCResolver{})
	if err := rc.ReconcileAll(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(repo.replaced["u1"]) != 2 {
		t.Fatalf("daily_stats rebuilt = %d days, want 2", len(repo.replaced["u1"]))
	}
	p := repo.profiles["u1"]
	if p.StreakCurrent != 2 || p.TotalRetained != 2 {
		t.Errorf("profile = %+v, want streak 2 / retained 2", p)
	}
}
