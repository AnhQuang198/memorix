package service

import (
	"context"
	"time"

	"github.com/memorix/memorix/internal/progress/domain"
)

// ReadRepo — read side (dashboard/stats).
type ReadRepo interface {
	DueCount(ctx context.Context, ownerID string, now time.Time) (int, error)
	WeekRetentionLogs(ctx context.Context, ownerID string, from, to time.Time) ([]domain.RetentionLog, error)
	Forecast(ctx context.Context, ownerID string, from, to time.Time, tz string) (map[string]int, error)
	TodayStat(ctx context.Context, userID string, day domain.Day) (int, int, error)
	GetStudyProfile(ctx context.Context, userID string) (domain.StudyProfile, bool, error)
	Heatmap(ctx context.Context, userID string, from, to domain.Day) ([]domain.DailyStat, error)
	Distribution(ctx context.Context, userID string, from, to domain.Day) (int, int, int, int, error)
}

// HeatCell là một ô heatmap.
type HeatCell struct {
	Day      string `json:"day"`
	Reviews  int    `json:"reviews"`
	Retained int    `json:"retained"`
}

// ForecastCell là tải dự báo một ngày.
type ForecastCell struct {
	Day string `json:"day"`
	Due int    `json:"due"`
}

// Distribution phân bố mức chấm.
type Distribution struct {
	Again int `json:"again"`
	Hard  int `json:"hard"`
	Good  int `json:"good"`
	Easy  int `json:"easy"`
}

// DashboardView — FR-30/31.
type DashboardView struct {
	DueCount         int        `json:"due_count"`
	NewToday         int        `json:"new_today"`
	StreakCurrent    int        `json:"streak_current"`
	NorthStar        int        `json:"north_star"`
	Heatmap          []HeatCell `json:"heatmap"`
	TomorrowForecast int        `json:"tomorrow_forecast"`
}

// StatsView — FR-33.
type StatsView struct {
	ReviewedToday int            `json:"reviewed_today"`
	Distribution  Distribution   `json:"distribution"`
	Forecast      []ForecastCell `json:"forecast"`
	Heatmap       []HeatCell     `json:"heatmap"`
	StreakCurrent int            `json:"streak_current"`
	StreakBest    int            `json:"streak_best"`
	Retention     float64        `json:"retention"`
	TotalRetained int            `json:"total_retained"`
	NorthStar     int            `json:"north_star"`
}

// Reader dựng view model dashboard/stats.
type Reader struct{ repo ReadRepo }

func NewReader(repo ReadRepo) *Reader { return &Reader{repo: repo} }

func dayStart(now time.Time, loc *time.Location) time.Time {
	n := now.In(loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
}

// weekStart = thứ Hai 00:00 theo TZ user.
func weekStart(now time.Time, loc *time.Location) time.Time {
	d := dayStart(now, loc)
	wd := int(d.Weekday())
	if wd == 0 {
		wd = 7 // Chủ nhật
	}
	return d.AddDate(0, 0, -(wd - 1))
}

func heatCells(stats []domain.DailyStat) []HeatCell {
	out := make([]HeatCell, len(stats))
	for i, s := range stats {
		out[i] = HeatCell{Day: s.Day.String(), Reviews: s.ReviewsDone, Retained: s.Retained}
	}
	return out
}

func (r *Reader) northStar(ctx context.Context, userID string, now time.Time, loc *time.Location) (int, error) {
	logs, err := r.repo.WeekRetentionLogs(ctx, userID, weekStart(now, loc), now)
	if err != nil {
		return 0, err
	}
	return domain.CountWordsRetained(logs), nil // đọc thẳng review_logs (AD-8)
}

func (r *Reader) Dashboard(ctx context.Context, userID string, now time.Time, loc *time.Location) (DashboardView, error) {
	var v DashboardView
	var err error
	if v.DueCount, err = r.repo.DueCount(ctx, userID, now); err != nil {
		return v, err
	}
	today := domain.DayOf(now, loc)
	if _, v.NewToday, err = r.repo.TodayStat(ctx, userID, today); err != nil {
		return v, err
	}
	prof, _, err := r.repo.GetStudyProfile(ctx, userID)
	if err != nil {
		return v, err
	}
	v.StreakCurrent = prof.StreakCurrent
	if v.NorthStar, err = r.northStar(ctx, userID, now, loc); err != nil {
		return v, err
	}
	from28 := domain.DayOf(dayStart(now, loc).AddDate(0, 0, -27), loc)
	hm, err := r.repo.Heatmap(ctx, userID, from28, today)
	if err != nil {
		return v, err
	}
	v.Heatmap = heatCells(hm)

	fcTo := dayStart(now, loc).AddDate(0, 0, 8)
	fc, err := r.repo.Forecast(ctx, userID, dayStart(now, loc), fcTo, loc.String())
	if err != nil {
		return v, err
	}
	tomorrow := domain.DayOf(dayStart(now, loc).AddDate(0, 0, 1), loc)
	v.TomorrowForecast = fc[tomorrow.String()]
	return v, nil
}

func (r *Reader) Stats(ctx context.Context, userID string, now time.Time, loc *time.Location) (StatsView, error) {
	var v StatsView
	today := domain.DayOf(now, loc)
	rev, _, err := r.repo.TodayStat(ctx, userID, today)
	if err != nil {
		return v, err
	}
	v.ReviewedToday = rev

	from90 := domain.DayOf(dayStart(now, loc).AddDate(0, 0, -89), loc)
	a, h, g, e, err := r.repo.Distribution(ctx, userID, from90, today)
	if err != nil {
		return v, err
	}
	v.Distribution = Distribution{Again: a, Hard: h, Good: g, Easy: e}
	total := a + h + g + e
	if total > 0 {
		v.Retention = float64(h+g+e) / float64(total)
	}

	prof, _, err := r.repo.GetStudyProfile(ctx, userID)
	if err != nil {
		return v, err
	}
	v.StreakCurrent, v.StreakBest, v.TotalRetained = prof.StreakCurrent, prof.StreakBest, prof.TotalRetained
	if v.NorthStar, err = r.northStar(ctx, userID, now, loc); err != nil {
		return v, err
	}

	hm, err := r.repo.Heatmap(ctx, userID, from90, today)
	if err != nil {
		return v, err
	}
	v.Heatmap = heatCells(hm)

	// Forecast 30 ngày tới — luôn đủ 30 ô (0 cho ngày trống) để FE render nhất quán.
	start := dayStart(now, loc)
	fc, err := r.repo.Forecast(ctx, userID, start, start.AddDate(0, 0, 30), loc.String())
	if err != nil {
		return v, err
	}
	v.Forecast = make([]ForecastCell, 30)
	for i := 0; i < 30; i++ {
		d := domain.DayOf(start.AddDate(0, 0, i), loc)
		v.Forecast[i] = ForecastCell{Day: d.String(), Due: fc[d.String()]}
	}
	return v, nil
}
