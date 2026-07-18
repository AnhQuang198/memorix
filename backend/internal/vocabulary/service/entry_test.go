package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
	"github.com/memorix/memorix/internal/vocabulary/ports"
)

// ---- fakes ----

type fakeEntryRepo struct {
	entries  map[uuid.UUID]*domain.Entry
	existing map[string]uuid.UUID // normalized term -> id
}

func newFakeEntryRepo() *fakeEntryRepo {
	return &fakeEntryRepo{entries: map[uuid.UUID]*domain.Entry{}, existing: map[string]uuid.UUID{}}
}
func (f *fakeEntryRepo) Insert(_ context.Context, e *domain.Entry) error {
	if _, ok := f.existing[e.Term]; ok {
		return domain.ErrDuplicateTerm
	}
	e.ID = uuid.New()
	f.entries[e.ID] = e
	f.existing[e.Term] = e.ID
	return nil
}
func (f *fakeEntryRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Entry, error) {
	e, ok := f.entries[id]
	if !ok {
		return nil, domain.ErrEntryNotFound
	}
	return e, nil
}
func (f *fakeEntryRepo) ExistingID(_ context.Context, _ uuid.UUID, term string) (uuid.UUID, bool, error) {
	id, ok := f.existing[term]
	return id, ok, nil
}
func (f *fakeEntryRepo) Update(_ context.Context, e *domain.Entry) error {
	if _, ok := f.entries[e.ID]; !ok {
		return domain.ErrEntryNotFound
	}
	f.entries[e.ID] = e
	return nil
}
func (f *fakeEntryRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.entries[id]; !ok {
		return domain.ErrEntryNotFound
	}
	delete(f.entries, id)
	return nil
}
func (f *fakeEntryRepo) ListPage(_ context.Context, _ uuid.UUID, _ string, _ httpx.Cursor, _ int) ([]domain.Entry, error) {
	var out []domain.Entry
	for _, e := range f.entries {
		out = append(out, *e)
	}
	return out, nil
}
func (f *fakeEntryRepo) ListPageByIDs(_ context.Context, _ uuid.UUID, ids []uuid.UUID, _ httpx.Cursor, _ int) ([]domain.Entry, error) {
	var out []domain.Entry
	for _, id := range ids {
		if e, ok := f.entries[id]; ok {
			out = append(out, *e)
		}
	}
	return out, nil
}

type fakeCards struct {
	created  []ports.CreateCancelHelper
	statuses map[uuid.UUID]string
}

// CreateCancelHelper để test bắt input (đặt tên rõ để tránh nhầm với DTO thật).
type _ = ports.CreateCardsInput

func (f *fakeCards) CreateCardsForEntry(_ context.Context, in ports.CreateCardsInput) error {
	f.created = append(f.created, ports.CreateCancelHelper(in))
	return nil
}
func (f *fakeCards) CardStatusesByEntry(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if s, ok := f.statuses[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}
func (f *fakeCards) EntryIDsByStatus(_ context.Context, _ uuid.UUID, _ string) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *fakeCards) BulkCreateForDeck(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (int, error) {
	return len(ids), nil
}

// ---- tests ----

func TestCreate_TermOnly_CreatesEntryAndCard(t *testing.T) {
	repo := newFakeEntryRepo()
	cards := &fakeCards{statuses: map[uuid.UUID]string{}}
	svc := New(repo, nil, cards, nil)
	owner := uuid.New()

	e, err := svc.Create(context.Background(), owner, CreateEntryInput{Term: "  Hello  "})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if e.Term != "Hello" {
		t.Errorf("term = %q, want trimmed Hello", e.Term)
	}
	if len(cards.created) != 1 || cards.created[0].EntryID != e.ID {
		t.Fatalf("card not created for entry: %+v", cards.created)
	}
	if got := cards.created[0].Directions; len(got) != 1 || got[0] != "front_back" {
		t.Errorf("default direction = %v, want [front_back]", got)
	}
}

func TestCreate_BlankTerm_Rejected(t *testing.T) {
	svc := New(newFakeEntryRepo(), nil, &fakeCards{}, nil)
	if _, err := svc.Create(context.Background(), uuid.New(), CreateEntryInput{Term: "   "}); err != domain.ErrTermRequired {
		t.Errorf("blank term err = %v, want ErrTermRequired", err)
	}
}

func TestCreate_Duplicate_ReturnsDuplicateError(t *testing.T) {
	repo := newFakeEntryRepo()
	existingID := uuid.New()
	repo.existing["dup"] = existingID
	svc := New(repo, nil, &fakeCards{}, nil)

	_, err := svc.Create(context.Background(), uuid.New(), CreateEntryInput{Term: "dup"})
	var de DuplicateError
	if !asDuplicate(err, &de) || de.ExistingID != existingID {
		t.Errorf("err = %v, want DuplicateError{%s}", err, existingID)
	}
}

func TestCreate_EscapesHTML(t *testing.T) {
	repo := newFakeEntryRepo()
	svc := New(repo, nil, &fakeCards{statuses: map[uuid.UUID]string{}}, nil)
	e, err := svc.Create(context.Background(), uuid.New(), CreateEntryInput{
		Term:  "safe",
		Notes: "<script>alert(1)</script>",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if e.Notes == "<script>alert(1)</script>" {
		t.Errorf("notes not escaped: %q", e.Notes)
	}
}

func TestGet_OwnershipEnforced(t *testing.T) {
	repo := newFakeEntryRepo()
	cards := &fakeCards{statuses: map[uuid.UUID]string{}}
	svc := New(repo, nil, cards, nil)
	owner := uuid.New()
	e, _ := svc.Create(context.Background(), owner, CreateEntryInput{Term: "mine"})

	if _, err := svc.Get(context.Background(), uuid.New(), e.ID); err != domain.ErrEntryNotFound {
		t.Errorf("other user Get err = %v, want ErrEntryNotFound (deny-by-default)", err)
	}
	view, err := svc.Get(context.Background(), owner, e.ID)
	if err != nil {
		t.Fatalf("owner Get: %v", err)
	}
	if view.Entry.ID != e.ID {
		t.Errorf("wrong entry returned")
	}
}

func asDuplicate(err error, target *DuplicateError) bool {
	d, ok := err.(DuplicateError)
	if ok {
		*target = d
	}
	return ok
}
