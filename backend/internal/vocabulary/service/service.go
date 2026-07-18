// Package service là use case của vocabulary (light hexagonal).
package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
	"github.com/memorix/memorix/internal/vocabulary/ports"
)

// EntryRepo là cổng lưu trữ entry (repo implements).
type EntryRepo interface {
	Insert(ctx context.Context, e *domain.Entry) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Entry, error)
	ExistingID(ctx context.Context, ownerID uuid.UUID, term string) (uuid.UUID, bool, error)
	Update(ctx context.Context, e *domain.Entry) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListPage(ctx context.Context, ownerID uuid.UUID, q string, cur httpx.Cursor, limit int) ([]domain.Entry, error)
	ListPageByIDs(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID, cur httpx.Cursor, limit int) ([]domain.Entry, error)
}

// DeckRepo là cổng lưu trữ curated deck + enrollment.
type DeckRepo interface {
	ListActiveDecks(ctx context.Context) ([]domain.CuratedDeck, error)
	FindDeckByID(ctx context.Context, id uuid.UUID) (domain.CuratedDeck, error)
	CuratedEntryIDs(ctx context.Context, deckID uuid.UUID) ([]uuid.UUID, error)
	InsertEnrollment(ctx context.Context, ownerID, deckID uuid.UUID) (uuid.UUID, error)
	CompleteEnrollment(ctx context.Context, ownerID, deckID uuid.UUID, cardCount int) error
}

// EnrollEnqueuer đẩy job enroll (River adapter implements).
type EnrollEnqueuer interface {
	EnqueueEnroll(ctx context.Context, ownerID, deckID uuid.UUID) error
}

type Service struct {
	entries EntryRepo
	decks   DeckRepo
	cards   ports.CardService
	jobs    EnrollEnqueuer
}

func New(entries EntryRepo, decks DeckRepo, cards ports.CardService, jobs EnrollEnqueuer) *Service {
	return &Service{entries: entries, decks: decks, cards: cards, jobs: jobs}
}
