package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/vocabulary/domain"
)

// ListCuratedDecks trả bộ khởi đầu (onboarding gợi ý + empty-state) (FR-11a/11c).
func (s *Service) ListCuratedDecks(ctx context.Context) ([]domain.CuratedDeck, error) {
	return s.decks.ListActiveDecks(ctx)
}

// Enroll tạo enrollment (409 nếu đã có) rồi đẩy job bulk-create card New (FR-11b).
// Trả enrollmentID; việc tạo card chạy nền idempotent.
func (s *Service) Enroll(ctx context.Context, ownerID, deckID uuid.UUID) (uuid.UUID, error) {
	if _, err := s.decks.FindDeckByID(ctx, deckID); err != nil {
		return uuid.Nil, err
	}
	enrollmentID, err := s.decks.InsertEnrollment(ctx, ownerID, deckID)
	if err != nil {
		return uuid.Nil, err // ErrAlreadyEnrolled → 409
	}
	if err := s.jobs.EnqueueEnroll(ctx, ownerID, deckID); err != nil {
		return uuid.Nil, err
	}
	return enrollmentID, nil
}
