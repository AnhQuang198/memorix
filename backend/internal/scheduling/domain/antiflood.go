package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// AntiFloodSpreadDays = số ngày tối đa rải phần overdue dư (FR-28: ≤7 ngày).
const AntiFloodSpreadDays = 7

// DeferredCard = một thẻ overdue bị hoãn sang ngày mới để chống nổ queue.
type DeferredCard struct {
	CardID   uuid.UUID
	NewDueAt time.Time
}

// AntiFloodPlan = kết quả PlanAntiFlood: phần hôm nay + phần hoãn rải nhiều ngày.
type AntiFloodPlan struct {
	Today    []Card         // hiển thị hôm nay (≤ 2×review-limit, R thấp trước)
	Deferred []DeferredCard // phần dư, due mới rải qua ≤7 ngày
	DayStart time.Time
}

// PlanAntiFlood chống nổ queue sau nghỉ dài (Story 4.4, FR-28) — THUẦN. Giữ tối đa
// 2×review-limit thẻ overdue (ưu tiên R thấp nhất) cho hôm nay; rải phần dư đều qua
// tối đa 7 ngày kế, R thấp hơn nhận ngày sớm hơn. Không dồn toàn bộ overdue một ngày.
func PlanAntiFlood(overdue []Card, prefs SchedulerPrefs, now time.Time) AntiFloodPlan {
	dayStart, err := StartOfStudyDay(now, prefs.Timezone)
	if err != nil {
		dayStart = now.UTC().Truncate(24 * time.Hour)
	}

	sorted := make([]Card, len(overdue))
	copy(sorted, overdue)
	sort.SliceStable(sorted, func(i, j int) bool {
		return Retrievability(sorted[i], now) < Retrievability(sorted[j], now)
	})

	limit := 2 * prefs.DailyReviewLimit
	if limit < 0 {
		limit = 0
	}
	if len(sorted) <= limit {
		return AntiFloodPlan{Today: sorted, DayStart: dayStart}
	}

	today := sorted[:limit]
	rest := sorted[limit:]
	perDay := (len(rest) + AntiFloodSpreadDays - 1) / AntiFloodSpreadDays
	if perDay < 1 {
		perDay = 1
	}
	deferred := make([]DeferredCard, 0, len(rest))
	for i, c := range rest {
		day := i/perDay + 1
		if day > AntiFloodSpreadDays {
			day = AntiFloodSpreadDays
		}
		deferred = append(deferred, DeferredCard{CardID: c.ID, NewDueAt: dayStart.AddDate(0, 0, day)})
	}
	return AntiFloodPlan{Today: today, Deferred: deferred, DayStart: dayStart}
}
