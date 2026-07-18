package jobs

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type fakeStore struct {
	entryIDs []uuid.UUID
	done     int
}

func (f *fakeStore) CuratedEntryIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return f.entryIDs, nil
}
func (f *fakeStore) CompleteEnrollment(_ context.Context, _, _ uuid.UUID, cardCount int) error {
	f.done = cardCount
	return nil
}

type fakeBulk struct{ created int }

func (f *fakeBulk) BulkCreateForDeck(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (int, error) {
	f.created += len(ids)
	return len(ids), nil
}

func TestEnrollWorker_BulkCreatesAndCompletes(t *testing.T) {
	store := &fakeStore{entryIDs: []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}}
	bulk := &fakeBulk{}
	w := &EnrollWorker{Store: store, Cards: bulk}

	job := &river.Job[EnrollDeckArgs]{Args: EnrollDeckArgs{OwnerID: uuid.New(), DeckID: uuid.New()}}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("work: %v", err)
	}
	if bulk.created != 3 {
		t.Errorf("bulk created = %d, want 3", bulk.created)
	}
	if store.done != 3 {
		t.Errorf("CompleteEnrollment card_count = %d, want 3", store.done)
	}
}

func TestEnrollDeckArgs_Kind(t *testing.T) {
	if (EnrollDeckArgs{}).Kind() != "enroll_deck" {
		t.Errorf("kind = %q", EnrollDeckArgs{}.Kind())
	}
}
