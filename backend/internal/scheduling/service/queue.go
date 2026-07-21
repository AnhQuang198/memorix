package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/ports"
)

// QueueService dựng queue học hôm nay: điều phối các hàm domain THUẦN (BuildQueue,
// PlanAntiFlood, ApplyDayCounts) với dữ liệu từ các cổng (AD-9). Không chứa toán lịch.
type QueueService struct {
	cards  ports.CardRepo
	prefs  ports.PrefsRepo
	review ports.ReviewActivityPort
}

func NewQueueService(c ports.CardRepo, p ports.PrefsRepo, r ports.ReviewActivityPort) *QueueService {
	return &QueueService{cards: c, prefs: p, review: r}
}

// BuildToday dựng queue hôm nay: nạp candidate → đếm đã phục vụ (TZ user) → chống nổ
// (rải overdue dư, persist deferral) → BuildQueue với ngân sách còn lại. Story 4.1/4.2/4.3/4.4.
func (s *QueueService) BuildToday(ctx context.Context, userID uuid.UUID, now time.Time) (domain.QueueResult, error) {
	prefs, err := s.prefs.Get(ctx, userID)
	if err != nil {
		return domain.QueueResult{}, err
	}
	dayStart, err := domain.StartOfStudyDay(now, prefs.Timezone)
	if err != nil {
		return domain.QueueResult{}, err
	}
	counts, err := s.review.CountServedSince(ctx, userID, dayStart)
	if err != nil {
		return domain.QueueResult{}, err
	}
	cards, err := s.cards.LoadCandidates(ctx, userID, now)
	if err != nil {
		return domain.QueueResult{}, err
	}

	overdue := overdueCards(cards, dayStart)
	if len(overdue) > 2*prefs.DailyReviewLimit {
		plan := domain.PlanAntiFlood(overdue, prefs, now)
		if len(plan.Deferred) > 0 {
			if err := s.cards.BulkDefer(ctx, plan.Deferred); err != nil {
				return domain.QueueResult{}, err
			}
			cards = dropDeferred(cards, plan.Deferred)
		}
	}

	effective := domain.ApplyDayCounts(prefs, counts)
	return domain.BuildQueue(cards, effective, now), nil
}

// overdueCards lọc thẻ Learning/Review đã quá hạn trước đầu ngày học (khớp khái niệm
// "overdue" của BuildQueue) — đầu vào cho PlanAntiFlood.
func overdueCards(cards []domain.Card, dayStart time.Time) []domain.Card {
	var out []domain.Card
	for _, c := range cards {
		if c.DueAt == nil {
			continue
		}
		if (c.Status == domain.StatusReview || c.Status == domain.StatusLearning) && c.DueAt.Before(dayStart) {
			out = append(out, c)
		}
	}
	return out
}

// dropDeferred loại các thẻ vừa được hoãn khỏi tập candidate hôm nay.
func dropDeferred(cards []domain.Card, deferred []domain.DeferredCard) []domain.Card {
	skip := make(map[uuid.UUID]bool, len(deferred))
	for _, d := range deferred {
		skip[d.CardID] = true
	}
	kept := make([]domain.Card, 0, len(cards))
	for _, c := range cards {
		if !skip[c.ID] {
			kept = append(kept, c)
		}
	}
	return kept
}
