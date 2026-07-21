package service

import (
	"context"
	"testing"
	"time"

	"github.com/memorix/memorix/internal/progress/domain"
)

type fakeReadRepo struct {
	weekLogs []domain.RetentionLog
	due      int
	forecast map[string]int
	todayRev int
	todayNew int
	profile  domain.StudyProfile
	heatmap  []domain.DailyStat
	dist     [4]int // again,hard,good,easy
}

func (f *fakeReadRepo) DueCount(context.Context, string, time.Time) (int, error) { return f.due, nil }
func (f *fakeReadRepo) WeekRetentionLogs(context.Context, string, time.Time, time.Time) ([]domain.RetentionLog, error) {
	return f.weekLogs, nil
}
func (f *fakeReadRepo) Forecast(context.Context, string, time.Time, time.Time, string) (map[string]int, error) {
	return f.forecast, nil
}
func (f *fakeReadRepo) TodayStat(context.Context, string, domain.Day) (int, int, error) {
	return f.todayRev, f.todayNew, nil
}
func (f *fakeReadRepo) GetStudyProfile(context.Context, string) (domain.StudyProfile, bool, error) {
	return f.profile, true, nil
}
func (f *fakeReadRepo) Heatmap(context.Context, string, domain.Day, domain.Day) ([]domain.DailyStat, error) {
	return f.heatmap, nil
}
func (f *fakeReadRepo) Distribution(context.Context, string, domain.Day, domain.Day) (int, int, int, int, error) {
	return f.dist[0], f.dist[1], f.dist[2], f.dist[3], nil
}

func TestReader_Dashboard_NorthStarFromLogs(t *testing.T) {
	repo := &fakeReadRepo{
		due:      24,
		todayNew: 5,
		profile:  domain.StudyProfile{StreakCurrent: 3},
		forecast: map[string]int{"2026-07-09": 8},
		weekLogs: []domain.RetentionLog{
			{CardID: "a", Grade: 3, ScheduledDays: 10},
			{CardID: "b", Grade: 4, ScheduledDays: 8},
			{CardID: "c", Grade: 2, ScheduledDays: 3}, // không retained
		},
	}
	rd := NewReader(repo)
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	v, err := rd.Dashboard(context.Background(), "u1", now, time.UTC)
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if v.DueCount != 24 || v.NewToday != 5 || v.StreakCurrent != 3 {
		t.Errorf("dashboard basics = %+v", v)
	}
	if v.NorthStar != 2 { // a,b distinct; đọc thẳng review_logs (AD-8)
		t.Errorf("NorthStar = %d, want 2", v.NorthStar)
	}
	if v.TomorrowForecast != 8 {
		t.Errorf("TomorrowForecast = %d, want 8", v.TomorrowForecast)
	}
}

func TestReader_Stats_RetentionAndDistribution(t *testing.T) {
	repo := &fakeReadRepo{
		todayRev: 20,
		dist:     [4]int{2, 3, 10, 5}, // again2 hard3 good10 easy5 → retention=18/20=0.9
		profile:  domain.StudyProfile{StreakCurrent: 3, StreakBest: 9, TotalRetained: 120},
		forecast: map[string]int{"2026-07-09": 8, "2026-07-20": 4},
	}
	rd := NewReader(repo)
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	v, err := rd.Stats(context.Background(), "u1", now, time.UTC)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if v.ReviewedToday != 20 || v.Distribution.Good != 10 || v.StreakBest != 9 || v.TotalRetained != 120 {
		t.Errorf("stats basics = %+v", v)
	}
	if v.Retention < 0.899 || v.Retention > 0.901 {
		t.Errorf("Retention = %v, want ~0.9", v.Retention)
	}
	if len(v.Forecast) != 30 {
		t.Errorf("forecast length = %d, want 30 (đủ 30 ngày, 0 cho ngày trống)", len(v.Forecast))
	}
}
