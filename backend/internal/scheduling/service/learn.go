package service

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/ports"
)

// LearnSession là luồng học thẻ mới cho hôm nay kèm cờ hiển thị coach lần đầu.
type LearnSession struct {
	Cards     []domain.Card
	ShowCoach bool
}

// LearnService điều phối luồng học thẻ New RIÊNG (Story 4.5): nạp thẻ New, tôn trọng
// giới hạn new còn lại hôm nay (AD-9), và quản cờ coach lần đầu (FR-29). Không chứa toán lịch.
type LearnService struct {
	cards    ports.CardRepo
	prefs    ports.PrefsRepo
	review   ports.ReviewActivityPort
	profiles ports.StudyProfileRepo
}

func NewLearnService(c ports.CardRepo, p ports.PrefsRepo, r ports.ReviewActivityPort, sp ports.StudyProfileRepo) *LearnService {
	return &LearnService{cards: c, prefs: p, review: r, profiles: sp}
}

// StartSession trả luồng học thẻ mới RIÊNG (Story 4.5): chỉ thẻ New tới hạn new còn
// lại hôm nay (rải theo giới hạn — FR-27), kèm cờ ShowCoach cho lần đầu (FR-29).
func (s *LearnService) StartSession(ctx context.Context, userID uuid.UUID, now time.Time) (LearnSession, error) {
	prefs, err := s.prefs.Get(ctx, userID)
	if err != nil {
		return LearnSession{}, err
	}
	dayStart, err := domain.StartOfStudyDay(now, prefs.Timezone)
	if err != nil {
		return LearnSession{}, err
	}
	counts, err := s.review.CountServedSince(ctx, userID, dayStart)
	if err != nil {
		return LearnSession{}, err
	}
	remaining := prefs.DailyNewLimit - counts.NewServed
	if remaining < 0 {
		remaining = 0
	}

	cards, err := s.cards.LoadCandidates(ctx, userID, now)
	if err != nil {
		return LearnSession{}, err
	}
	newCards := make([]domain.Card, 0, remaining)
	for _, c := range cards {
		if c.Status == domain.StatusNew {
			newCards = append(newCards, c)
		}
	}
	sort.SliceStable(newCards, func(i, j int) bool { return newCards[i].CreatedAt.Before(newCards[j].CreatedAt) })
	if len(newCards) > remaining {
		newCards = newCards[:remaining]
	}

	seen, err := s.profiles.CoachSeen(ctx, userID)
	if err != nil {
		return LearnSession{}, err
	}
	return LearnSession{Cards: newCards, ShowCoach: !seen}, nil
}

// AckCoach ghi nhận đã xem hướng dẫn chấm lần đầu (persist "seen coach"), để mini-onboarding
// chỉ hiển thị một lần (FR-29).
func (s *LearnService) AckCoach(ctx context.Context, userID uuid.UUID) error {
	return s.profiles.MarkCoachSeen(ctx, userID)
}
