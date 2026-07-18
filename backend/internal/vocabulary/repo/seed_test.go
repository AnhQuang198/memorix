package repo

import (
	"context"
	"testing"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
)

func TestSeed_StarterDeckPresent(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()

	decks, err := r.ListActiveDecks(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var deckID string
	for _, d := range decks {
		if d.Slug == "ielts-starter" {
			deckID = d.ID.String()
		}
	}
	if deckID == "" {
		t.Fatal("ielts-starter deck not seeded")
	}
	var entryCount, meaningCount int
	_ = pool.QueryRow(ctx,
		`SELECT count(*) FROM vocabulary.entries
		 WHERE curated_deck_id=$1 AND owner_id IS NULL`, deckID).Scan(&entryCount)
	_ = pool.QueryRow(ctx,
		`SELECT count(*) FROM vocabulary.meanings m
		 JOIN vocabulary.entries e ON e.id=m.entry_id WHERE e.curated_deck_id=$1`, deckID).Scan(&meaningCount)
	if entryCount != 8 {
		t.Errorf("curated entries = %d, want 8", entryCount)
	}
	if meaningCount != 8 {
		t.Errorf("curated meanings = %d, want 8", meaningCount)
	}
}
