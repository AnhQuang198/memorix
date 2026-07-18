package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type fakeRepo struct {
	created   [][]string
	bulkCalls int
}

func (f *fakeRepo) CreateCardsForEntry(_ context.Context, _ uuid.UUID, _ uuid.UUID, dirs []string) error {
	f.created = append(f.created, dirs)
	return nil
}
func (f *fakeRepo) CardStatusesByEntry(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	m := map[uuid.UUID]string{}
	for _, id := range ids {
		m[id] = "new"
	}
	return m, nil
}
func (f *fakeRepo) EntryIDsByStatus(_ context.Context, _ uuid.UUID, _ string) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *fakeRepo) BulkCreateForDeck(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (int, error) {
	f.bulkCalls++
	return len(ids), nil
}

func TestService_CreateCardsForEntry_DefaultsDirection(t *testing.T) {
	f := &fakeRepo{}
	svc := New(f)
	in := CreateCardsInput{OwnerID: uuid.New(), EntryID: uuid.New(), Directions: nil}
	if err := svc.CreateCardsForEntry(context.Background(), in); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(f.created) != 1 || len(f.created[0]) != 1 || f.created[0][0] != "front_back" {
		t.Errorf("expected default front_back, got %v", f.created)
	}
}

func TestService_CreateCardsForEntry_FiltersInvalid(t *testing.T) {
	f := &fakeRepo{}
	svc := New(f)
	in := CreateCardsInput{OwnerID: uuid.New(), EntryID: uuid.New(), Directions: []string{"front_back", "back_front", "junk"}}
	if err := svc.CreateCardsForEntry(context.Background(), in); err != nil {
		t.Fatalf("create: %v", err)
	}
	if got := f.created[0]; len(got) != 2 {
		t.Errorf("invalid direction not filtered: %v", got)
	}
}
