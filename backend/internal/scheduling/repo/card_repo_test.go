package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
)

func TestCardRepo_CreateAndStatuses(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	owner := uuid.New()
	entry := uuid.New()

	// Tạo 2 direction, gọi lại lần 2 phải idempotent.
	for i := 0; i < 2; i++ {
		if err := r.CreateCardsForEntry(ctx, owner, entry, []string{"front_back", "back_front"}); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	statuses, err := r.CardStatusesByEntry(ctx, owner, []uuid.UUID{entry})
	if err != nil {
		t.Fatalf("statuses: %v", err)
	}
	if statuses[entry] != "new" {
		t.Errorf("primary status = %q, want new", statuses[entry])
	}

	ids, err := r.EntryIDsByStatus(ctx, owner, "new")
	if err != nil {
		t.Fatalf("byStatus: %v", err)
	}
	if len(ids) != 1 || ids[0] != entry {
		t.Errorf("EntryIDsByStatus = %v, want [%s]", ids, entry)
	}
}

func TestCardRepo_BulkCreateIdempotent(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	owner := uuid.New()
	e1, e2 := uuid.New(), uuid.New()

	n, err := r.BulkCreateForDeck(ctx, owner, []uuid.UUID{e1, e2})
	if err != nil || n != 2 {
		t.Fatalf("first bulk = %d, %v; want 2", n, err)
	}
	n2, err := r.BulkCreateForDeck(ctx, owner, []uuid.UUID{e1, e2})
	if err != nil || n2 != 0 {
		t.Fatalf("second bulk = %d, %v; want 0 (idempotent)", n2, err)
	}
}
