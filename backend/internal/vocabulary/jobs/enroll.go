// Package jobs chứa River worker của vocabulary (adapter tầng job).
// Orchestrate: đọc curated entry (vocabulary) + nhờ scheduling bulk-create card.
package jobs

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

// EnrollDeckArgs là payload job enroll (River serialize JSON).
type EnrollDeckArgs struct {
	OwnerID uuid.UUID `json:"owner_id"`
	DeckID  uuid.UUID `json:"deck_id"`
}

func (EnrollDeckArgs) Kind() string { return "enroll_deck" }

// EnrollStore là phần vocabulary mà worker cần.
type EnrollStore interface {
	CuratedEntryIDs(ctx context.Context, deckID uuid.UUID) ([]uuid.UUID, error)
	CompleteEnrollment(ctx context.Context, ownerID, deckID uuid.UUID, cardCount int) error
}

// BulkCardCreator là phần scheduling mà worker cần (AD-9).
type BulkCardCreator interface {
	BulkCreateForDeck(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (int, error)
}

type EnrollWorker struct {
	river.WorkerDefaults[EnrollDeckArgs]
	Store EnrollStore
	Cards BulkCardCreator
}

// Work bulk-create card New cho toàn bộ entry curated của deck; idempotent
// (BulkCreateForDeck ON CONFLICT DO NOTHING) nên retry an toàn (FR-11b).
func (w *EnrollWorker) Work(ctx context.Context, job *river.Job[EnrollDeckArgs]) error {
	entryIDs, err := w.Store.CuratedEntryIDs(ctx, job.Args.DeckID)
	if err != nil {
		return err
	}
	created, err := w.Cards.BulkCreateForDeck(ctx, job.Args.OwnerID, entryIDs)
	if err != nil {
		return err
	}
	return w.Store.CompleteEnrollment(ctx, job.Args.OwnerID, job.Args.DeckID, created)
}

// Enqueuer đẩy job enroll (service.EnrollEnqueuer). Bọc River client.
type Enqueuer struct {
	Client *river.Client[pgx.Tx]
}

func (e *Enqueuer) EnqueueEnroll(ctx context.Context, ownerID, deckID uuid.UUID) error {
	_, err := e.Client.Insert(ctx, EnrollDeckArgs{OwnerID: ownerID, DeckID: deckID}, nil)
	return err
}
