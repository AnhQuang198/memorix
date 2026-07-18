package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
)

func ptr(u uuid.UUID) *uuid.UUID { return &u }

func TestEntryRepo_InsertFindWithChildren(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	owner := uuid.New()

	e := &domain.Entry{
		OwnerID:      ptr(owner),
		Term:         "ubiquitous",
		PartOfSpeech: "adj",
		Notes:        "note",
		Meanings:     []domain.Meaning{{PartOfSpeech: "adj", Definition: "everywhere", Position: 0}},
		Examples:     []domain.Example{{Text: "It is ubiquitous.", Position: 0}},
		Relations:    []domain.SynAnt{{Relation: domain.RelationSynonym, Value: "omnipresent"}},
	}
	if err := r.Insert(ctx, e); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if e.ID == uuid.Nil {
		t.Fatal("insert did not set ID")
	}
	got, err := r.FindByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Term != "ubiquitous" || len(got.Meanings) != 1 || len(got.Examples) != 1 || len(got.Relations) != 1 {
		t.Errorf("children not loaded: %+v", got)
	}
}

func TestEntryRepo_ExistingID_Normalized(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	owner := uuid.New()

	e := &domain.Entry{OwnerID: ptr(owner), Term: "Café"}
	if err := r.Insert(ctx, e); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// khác dấu/hoa thường vẫn coi là trùng (FR-10).
	id, ok, err := r.ExistingID(ctx, owner, "cafe")
	if err != nil {
		t.Fatalf("existing: %v", err)
	}
	if !ok || id != e.ID {
		t.Errorf("ExistingID = %s, %v; want %s, true", id, ok, e.ID)
	}
	// insert trùng normalized -> ErrDuplicateTerm.
	if err := r.Insert(ctx, &domain.Entry{OwnerID: ptr(owner), Term: "CAFE"}); err != domain.ErrDuplicateTerm {
		t.Errorf("dup insert err = %v, want ErrDuplicateTerm", err)
	}
}

func TestEntryRepo_ListPage_CursorAndSoftDelete(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	owner := uuid.New()
	for _, term := range []string{"alpha", "bravo", "charlie"} {
		if err := r.Insert(ctx, &domain.Entry{OwnerID: ptr(owner), Term: term}); err != nil {
			t.Fatalf("insert %s: %v", term, err)
		}
	}
	page1, err := r.ListPage(ctx, owner, "", httpx.Cursor{}, 2)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 len = %d, want 2", len(page1))
	}
	last := page1[len(page1)-1]
	cur := httpx.Cursor{SortKey: last.CreatedAt.Format(cursorTimeLayout), ID: last.ID.String()}
	page2, err := r.ListPage(ctx, owner, "", cur, 2)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) != 1 {
		t.Errorf("page2 len = %d, want 1", len(page2))
	}

	// soft delete ẩn khỏi list.
	if err := r.SoftDelete(ctx, page1[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	all, _ := r.ListPage(ctx, owner, "", httpx.Cursor{}, 10)
	if len(all) != 2 {
		t.Errorf("after delete len = %d, want 2", len(all))
	}
}
