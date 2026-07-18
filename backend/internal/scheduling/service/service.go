// Package service là use case của scheduling. Service hiện thực cổng mà
// vocabulary cần (vocabulary/ports.CardService) — ráp ở cmd/api (AD-9).
package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/scheduling/domain"
)

// CardRepo là cổng lưu trữ (repo implements).
type CardRepo interface {
	CreateCardsForEntry(ctx context.Context, ownerID, entryID uuid.UUID, directions []string) error
	CardStatusesByEntry(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]string, error)
	EntryIDsByStatus(ctx context.Context, ownerID uuid.UUID, status string) ([]uuid.UUID, error)
	BulkCreateForDeck(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (int, error)
}

// CreateCardsInput khớp vocabulary/ports.CreateCardsInput (cùng shape).
type CreateCardsInput struct {
	OwnerID    uuid.UUID
	EntryID    uuid.UUID
	Directions []string
}

type Service struct {
	repo CardRepo
}

func New(repo CardRepo) *Service { return &Service{repo: repo} }

func (s *Service) CreateCardsForEntry(ctx context.Context, in CreateCardsInput) error {
	dirs := validDirections(in.Directions)
	return s.repo.CreateCardsForEntry(ctx, in.OwnerID, in.EntryID, dirs)
}

func (s *Service) CardStatusesByEntry(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	return s.repo.CardStatusesByEntry(ctx, ownerID, entryIDs)
}

func (s *Service) EntryIDsByStatus(ctx context.Context, ownerID uuid.UUID, status string) ([]uuid.UUID, error) {
	return s.repo.EntryIDsByStatus(ctx, ownerID, status)
}

func (s *Service) BulkCreateForDeck(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (int, error) {
	return s.repo.BulkCreateForDeck(ctx, ownerID, entryIDs)
}

func validDirections(in []string) []string {
	var out []string
	for _, d := range in {
		if domain.Direction(d).Valid() {
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		return []string{string(domain.DirectionFrontBack)}
	}
	return out
}
