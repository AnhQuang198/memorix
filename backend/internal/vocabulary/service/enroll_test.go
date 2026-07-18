package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/vocabulary/domain"
)

type fakeDeckRepo struct {
	decks     []domain.CuratedDeck
	enrolled  map[uuid.UUID]bool // deckID -> enrolled
	enrollErr error
}

func (f *fakeDeckRepo) ListActiveDecks(context.Context) ([]domain.CuratedDeck, error) {
	return f.decks, nil
}
func (f *fakeDeckRepo) FindDeckByID(_ context.Context, id uuid.UUID) (domain.CuratedDeck, error) {
	for _, d := range f.decks {
		if d.ID == id {
			return d, nil
		}
	}
	return domain.CuratedDeck{}, domain.ErrDeckNotFound
}
func (f *fakeDeckRepo) CuratedEntryIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *fakeDeckRepo) InsertEnrollment(_ context.Context, _, deckID uuid.UUID) (uuid.UUID, error) {
	if f.enrollErr != nil {
		return uuid.Nil, f.enrollErr
	}
	if f.enrolled[deckID] {
		return uuid.Nil, domain.ErrAlreadyEnrolled
	}
	f.enrolled[deckID] = true
	return uuid.New(), nil
}
func (f *fakeDeckRepo) CompleteEnrollment(context.Context, uuid.UUID, uuid.UUID, int) error {
	return nil
}

type fakeEnqueuer struct{ calls int }

func (f *fakeEnqueuer) EnqueueEnroll(context.Context, uuid.UUID, uuid.UUID) error {
	f.calls++
	return nil
}

func TestEnroll_CreatesEnrollmentAndEnqueues(t *testing.T) {
	deck := domain.CuratedDeck{ID: uuid.New(), Slug: "ielts-starter", Name: "IELTS", IsActive: true}
	decks := &fakeDeckRepo{decks: []domain.CuratedDeck{deck}, enrolled: map[uuid.UUID]bool{}}
	q := &fakeEnqueuer{}
	svc := New(newFakeEntryRepo(), decks, &fakeCards{}, q)

	if _, err := svc.Enroll(context.Background(), uuid.New(), deck.ID); err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if q.calls != 1 {
		t.Errorf("enqueue calls = %d, want 1", q.calls)
	}
}

func TestEnroll_AlreadyEnrolled(t *testing.T) {
	deck := domain.CuratedDeck{ID: uuid.New(), IsActive: true}
	decks := &fakeDeckRepo{decks: []domain.CuratedDeck{deck}, enrolled: map[uuid.UUID]bool{deck.ID: true}}
	q := &fakeEnqueuer{}
	svc := New(newFakeEntryRepo(), decks, &fakeCards{}, q)

	if _, err := svc.Enroll(context.Background(), uuid.New(), deck.ID); err != domain.ErrAlreadyEnrolled {
		t.Errorf("err = %v, want ErrAlreadyEnrolled", err)
	}
	if q.calls != 0 {
		t.Errorf("should not enqueue on duplicate enroll")
	}
}

func TestEnroll_UnknownDeck(t *testing.T) {
	decks := &fakeDeckRepo{decks: nil, enrolled: map[uuid.UUID]bool{}}
	svc := New(newFakeEntryRepo(), decks, &fakeCards{}, &fakeEnqueuer{})
	if _, err := svc.Enroll(context.Background(), uuid.New(), uuid.New()); err != domain.ErrDeckNotFound {
		t.Errorf("err = %v, want ErrDeckNotFound", err)
	}
}

func TestListCuratedDecks(t *testing.T) {
	deck := domain.CuratedDeck{ID: uuid.New(), Slug: "ielts-starter", IsActive: true}
	svc := New(newFakeEntryRepo(), &fakeDeckRepo{decks: []domain.CuratedDeck{deck}, enrolled: map[uuid.UUID]bool{}}, &fakeCards{}, &fakeEnqueuer{})
	got, err := svc.ListCuratedDecks(context.Background())
	if err != nil || len(got) != 1 || got[0].Slug != "ielts-starter" {
		t.Errorf("ListCuratedDecks = %+v, %v", got, err)
	}
}
