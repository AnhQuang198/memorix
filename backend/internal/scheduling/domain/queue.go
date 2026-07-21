package domain

import (
	"sort"
	"time"
)

// QueueResult là queue đã sắp xếp + đếm loại.
type QueueResult struct {
	Cards       []Card
	NewCount    int
	ReviewCount int
}

// DayCounts = số thẻ đã phục vụ hôm nay (đếm theo ngày học TZ user), từ review_logs.
type DayCounts struct {
	NewServed    int
	ReviewServed int
}

// ApplyDayCounts trừ hạn ngày còn lại theo số đã phục vụ hôm nay (kẹp ≥0).
func ApplyDayCounts(prefs SchedulerPrefs, counts DayCounts) SchedulerPrefs {
	p := prefs
	p.DailyNewLimit = clampMin0(prefs.DailyNewLimit - counts.NewServed)
	p.DailyReviewLimit = clampMin0(prefs.DailyReviewLimit - counts.ReviewServed)
	return p
}

func clampMin0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

// BuildQueue dựng queue ưu tiên THUẦN (Story 4.1): overdue nặng (R thấp) → relearning
// → review đến hạn → new. Tôn trọng hạn ngày trong prefs — gọi ApplyDayCounts trước
// để prefs là "ngân sách còn lại hôm nay" (Story 4.2/4.3: rải new, giữ phần dư qua ngày sau).
// Bỏ qua thẻ suspended và thẻ có due tương lai (chưa tới hạn).
func BuildQueue(cards []Card, prefs SchedulerPrefs, now time.Time) QueueResult {
	dayStart, err := StartOfStudyDay(now, prefs.Timezone)
	if err != nil {
		dayStart = now.UTC().Truncate(24 * time.Hour)
	}

	var overdue, relearning, reviewDue, newCards []Card
	for _, c := range cards {
		switch c.Status {
		case StatusNew:
			newCards = append(newCards, c)
		case StatusRelearning:
			if c.DueAt != nil && !c.DueAt.After(now) {
				relearning = append(relearning, c)
			}
		case StatusLearning, StatusReview:
			if c.DueAt == nil {
				continue
			}
			switch {
			case c.DueAt.Before(dayStart):
				overdue = append(overdue, c)
			case !c.DueAt.After(now):
				reviewDue = append(reviewDue, c)
			}
		case StatusSuspended:
			// thẻ tạm ngưng không vào queue.
		}
	}

	sort.SliceStable(overdue, func(i, j int) bool {
		return Retrievability(overdue[i], now) < Retrievability(overdue[j], now)
	})
	sort.SliceStable(relearning, func(i, j int) bool { return relearning[i].DueAt.Before(*relearning[j].DueAt) })
	sort.SliceStable(reviewDue, func(i, j int) bool { return reviewDue[i].DueAt.Before(*reviewDue[j].DueAt) })
	sort.SliceStable(newCards, func(i, j int) bool { return newCards[i].CreatedAt.Before(newCards[j].CreatedAt) })

	review := make([]Card, 0, len(overdue)+len(relearning)+len(reviewDue))
	review = append(review, overdue...)
	review = append(review, relearning...)
	review = append(review, reviewDue...)
	if len(review) > prefs.DailyReviewLimit {
		review = review[:prefs.DailyReviewLimit]
	}
	if len(newCards) > prefs.DailyNewLimit {
		newCards = newCards[:prefs.DailyNewLimit]
	}

	out := make([]Card, 0, len(review)+len(newCards))
	out = append(out, review...)
	out = append(out, newCards...)
	return QueueResult{Cards: out, NewCount: len(newCards), ReviewCount: len(review)}
}
