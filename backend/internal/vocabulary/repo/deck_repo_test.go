package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/vocabulary/domain"
)

func seedDeck(ctx context.Context, r *Repo, slug string, entryTerms []string) (uuid.UUID, error) {
	var deckID uuid.UUID
	err := r.pool.QueryRow(ctx,
		`INSERT INTO vocabulary.curated_decks (slug, name) VALUES ($1, $1) RETURNING id`, slug).Scan(&deckID)
	if err != nil {
		return uuid.Nil, err
	}
	for _, term := range entryTerms {
		if _, err := r.pool.Exec(ctx,
			`INSERT INTO vocabulary.entries (owner_id, curated_deck_id, term) VALUES (NULL, $1, $2)`,
			deckID, term); err != nil {
			return uuid.Nil, err
		}
	}
	return deckID, nil
}

func TestDeckRepo_ListActiveAndCuratedEntries(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()

	deckID, err := seedDeck(ctx, r, "test-deck", []string{"one", "two", "three"})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	decks, err := r.ListActiveDecks(ctx)
	if err != nil {
		t.Fatalf("list decks: %v", err)
	}
	// Ít nhất deck vừa seed (migration seed cũng có thể thêm nữa).
	found := false
	for _, d := range decks {
		if d.ID == deckID && d.Slug == "test-deck" {
			found = true
		}
	}
	if !found {
		t.Errorf("seeded deck not in ListActiveDecks: %+v", decks)
	}
	ids, err := r.CuratedEntryIDs(ctx, deckID)
	if err != nil {
		t.Fatalf("curated ids: %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("curated entry count = %d, want 3", len(ids))
	}
}

func TestDeckRepo_EnrollmentUnique(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	deckID, err := seedDeck(ctx, r, "enroll-deck", []string{"a"})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	owner := uuid.New()
	if _, err := r.InsertEnrollment(ctx, owner, deckID); err != nil {
		t.Fatalf("first enroll: %v", err)
	}
	if _, err := r.InsertEnrollment(ctx, owner, deckID); err != domain.ErrAlreadyEnrolled {
		t.Errorf("second enroll err = %v, want ErrAlreadyEnrolled", err)
	}
	if err := r.CompleteEnrollment(ctx, owner, deckID, 5); err != nil {
		t.Fatalf("complete: %v", err)
	}
}
