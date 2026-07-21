package domain

import (
	"sort"
	"time"
)

// LogRow là dòng review_logs tối thiểu để rebuild daily_stats.
type LogRow struct {
	CardID        string
	Grade         int
	ScheduledDays int
	ReviewedAt    time.Time
}

// DailyStat = một hàng progress.daily_stats.
type DailyStat struct {
	Day         Day
	ReviewsDone int
	NewDone     int
	Retained    int
	Again       int
	Hard        int
	Good        int
	Easy        int
}

// RebuildDailyStats gộp toàn bộ log của MỘT user thành daily_stats theo TZ user,
// trả slice sort tăng theo ngày. new_done = số thẻ có lần ôn ĐẦU TIÊN rơi vào ngày đó.
func RebuildDailyStats(logs []LogRow, loc *time.Location) []DailyStat {
	byDay := make(map[Day]*DailyStat)
	firstSeen := make(map[string]bool)

	// Đảm bảo thứ tự thời gian để xác định "lần ôn đầu" đúng.
	ordered := append([]LogRow(nil), logs...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].ReviewedAt.Before(ordered[j].ReviewedAt) })

	for _, l := range ordered {
		d := DayOf(l.ReviewedAt, loc)
		s := byDay[d]
		if s == nil {
			s = &DailyStat{Day: d}
			byDay[d] = s
		}
		s.ReviewsDone++
		if !firstSeen[l.CardID] {
			firstSeen[l.CardID] = true
			s.NewDone++
		}
		switch l.Grade {
		case GradeAgain:
			s.Again++
		case GradeHard:
			s.Hard++
		case GradeGood:
			s.Good++
		case GradeEasy:
			s.Easy++
		}
		if IsRetained(l.Grade, l.ScheduledDays) {
			s.Retained++
		}
	}

	out := make([]DailyStat, 0, len(byDay))
	for _, s := range byDay {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return DaysBetween(out[j].Day, out[i].Day) < 0 })
	return out
}
