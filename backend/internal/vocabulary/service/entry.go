package service

import (
	"context"
	"html"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
	"github.com/memorix/memorix/internal/vocabulary/ports"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// cursorLayout phải trùng repo.cursorTimeLayout (RFC3339Nano).
const cursorLayout = "2006-01-02T15:04:05.999999999Z07:00"

// DuplicateError mang id entry hiện có để client mở lại (FR-10).
type DuplicateError struct{ ExistingID uuid.UUID }

func (e DuplicateError) Error() string { return "duplicate term" }

type MeaningInput struct {
	PartOfSpeech string
	Definition   string
}
type PronunciationInput struct {
	IPA      string
	Dialect  string
	AudioURL string
}

type CreateEntryInput struct {
	Term           string
	PartOfSpeech   string
	Notes          string
	Source         string
	Directions     []string
	Meanings       []MeaningInput
	Examples       []string
	Pronunciations []PronunciationInput
	Synonyms       []string
	Antonyms       []string
}

type UpdateEntryInput struct {
	Term           string
	PartOfSpeech   string
	Notes          string
	Source         string
	Meanings       []MeaningInput
	Examples       []string
	Pronunciations []PronunciationInput
	Synonyms       []string
	Antonyms       []string
}

// EntryView = entry + status card primary (từ scheduling).
type EntryView struct {
	Entry  domain.Entry
	Status string
}

type ListInput struct {
	OwnerID uuid.UUID
	Status  string
	Query   string
	Cursor  string
	Limit   int
}

type EntryListItem struct {
	Entry  domain.Entry
	Status string
}

type ListResult struct {
	Items []EntryListItem
	Page  httpx.Page
}

// Create tạo entry (term-only <10s) + tự tạo card New qua scheduling (FR-7, FR-8).
func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, in CreateEntryInput) (domain.Entry, error) {
	term, err := domain.ValidateTerm(in.Term)
	if err != nil {
		return domain.Entry{}, err
	}
	// Cảnh báo trùng: trả DuplicateError kèm id hiện có (FR-10).
	if id, ok, err := s.entries.ExistingID(ctx, ownerID, term); err != nil {
		return domain.Entry{}, err
	} else if ok {
		return domain.Entry{}, DuplicateError{ExistingID: id}
	}

	e := buildEntry(&ownerID, term, in)
	if err := s.entries.Insert(ctx, &e); err != nil {
		if err == domain.ErrDuplicateTerm {
			// Race: đọc lại id hiện có.
			if id, ok, _ := s.entries.ExistingID(ctx, ownerID, term); ok {
				return domain.Entry{}, DuplicateError{ExistingID: id}
			}
		}
		return domain.Entry{}, err
	}
	// Cross-module (AD-9): nhờ scheduling tạo card New. Không ghi thẳng cards.
	dirs := validDirs(in.Directions)
	if err := s.cards.CreateCardsForEntry(ctx, ports.CreateCardsInput{
		OwnerID: ownerID, EntryID: e.ID, Directions: dirs,
	}); err != nil {
		return domain.Entry{}, err
	}
	return e, nil
}

func buildEntry(owner *uuid.UUID, term string, in CreateEntryInput) domain.Entry {
	e := domain.Entry{
		OwnerID:      owner,
		Term:         term,
		PartOfSpeech: in.PartOfSpeech,
		Notes:        html.EscapeString(in.Notes),
		Source:       in.Source,
	}
	for i, m := range in.Meanings {
		e.Meanings = append(e.Meanings, domain.Meaning{
			PartOfSpeech: m.PartOfSpeech, Definition: html.EscapeString(m.Definition), Position: i,
		})
	}
	for i, x := range in.Examples {
		e.Examples = append(e.Examples, domain.Example{Text: html.EscapeString(x), Position: i})
	}
	for _, p := range in.Pronunciations {
		e.Pronunciations = append(e.Pronunciations, domain.Pronunciation{IPA: p.IPA, Dialect: p.Dialect, AudioURL: p.AudioURL})
	}
	for _, sy := range in.Synonyms {
		e.Relations = append(e.Relations, domain.SynAnt{Relation: domain.RelationSynonym, Value: html.EscapeString(sy)})
	}
	for _, an := range in.Antonyms {
		e.Relations = append(e.Relations, domain.SynAnt{Relation: domain.RelationAntonym, Value: html.EscapeString(an)})
	}
	return e
}

func validDirs(in []string) []string {
	var ds []domain.Direction
	for _, d := range in {
		ds = append(ds, domain.Direction(d))
	}
	valid := domain.DefaultDirections(ds)
	out := make([]string, len(valid))
	for i, d := range valid {
		out[i] = string(d)
	}
	return out
}

// Get trả entry của owner + status card (ownership deny-by-default → 404).
func (s *Service) Get(ctx context.Context, ownerID, id uuid.UUID) (EntryView, error) {
	e, err := s.entries.FindByID(ctx, id)
	if err != nil {
		return EntryView{}, err
	}
	if e.OwnerID == nil || *e.OwnerID != ownerID {
		return EntryView{}, domain.ErrEntryNotFound
	}
	statuses, err := s.cards.CardStatusesByEntry(ctx, ownerID, []uuid.UUID{id})
	if err != nil {
		return EntryView{}, err
	}
	return EntryView{Entry: *e, Status: statuses[id]}, nil
}

// Update sửa entry của owner, giữ nguyên card FSRS (FR-9); dup vẫn 409.
func (s *Service) Update(ctx context.Context, ownerID, id uuid.UUID, in UpdateEntryInput) (domain.Entry, error) {
	e, err := s.entries.FindByID(ctx, id)
	if err != nil {
		return domain.Entry{}, err
	}
	if e.OwnerID == nil || *e.OwnerID != ownerID {
		return domain.Entry{}, domain.ErrEntryNotFound
	}
	term, err := domain.ValidateTerm(in.Term)
	if err != nil {
		return domain.Entry{}, err
	}
	if existID, ok, err := s.entries.ExistingID(ctx, ownerID, term); err != nil {
		return domain.Entry{}, err
	} else if ok && existID != id {
		return domain.Entry{}, DuplicateError{ExistingID: existID}
	}
	updated := buildEntry(&ownerID, term, CreateEntryInput{
		PartOfSpeech: in.PartOfSpeech, Notes: in.Notes, Source: in.Source,
		Meanings: in.Meanings, Examples: in.Examples, Pronunciations: in.Pronunciations,
		Synonyms: in.Synonyms, Antonyms: in.Antonyms,
	})
	updated.ID = id
	if err := s.entries.Update(ctx, &updated); err != nil {
		if err == domain.ErrDuplicateTerm {
			return domain.Entry{}, DuplicateError{ExistingID: id}
		}
		return domain.Entry{}, err
	}
	return updated, nil
}

// Delete soft-delete entry của owner (FR-9).
func (s *Service) Delete(ctx context.Context, ownerID, id uuid.UUID) error {
	e, err := s.entries.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if e.OwnerID == nil || *e.OwnerID != ownerID {
		return domain.ErrEntryNotFound
	}
	return s.entries.SoftDelete(ctx, id)
}

// List phân trang + lọc status (status batch-load qua port, không join chéo — AD-9).
func (s *Service) List(ctx context.Context, in ListInput) (ListResult, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	cur, err := httpx.DecodeCursor(in.Cursor)
	if err != nil {
		return ListResult{}, err
	}

	var entries []domain.Entry
	if in.Status != "" {
		ids, err := s.cards.EntryIDsByStatus(ctx, in.OwnerID, in.Status)
		if err != nil {
			return ListResult{}, err
		}
		if len(ids) == 0 {
			return ListResult{Items: nil, Page: httpx.Page{Limit: limit}}, nil
		}
		entries, err = s.entries.ListPageByIDs(ctx, in.OwnerID, ids, cur, limit+1)
		if err != nil {
			return ListResult{}, err
		}
	} else {
		entries, err = s.entries.ListPage(ctx, in.OwnerID, in.Query, cur, limit+1)
		if err != nil {
			return ListResult{}, err
		}
	}

	page := httpx.Page{Limit: limit}
	if len(entries) > limit {
		entries = entries[:limit]
		page.HasMore = true
		last := entries[len(entries)-1]
		page.NextCursor = httpx.Cursor{SortKey: last.CreatedAt.UTC().Format(cursorLayout), ID: last.ID.String()}.Encode()
	}

	entryIDs := make([]uuid.UUID, len(entries))
	for i, e := range entries {
		entryIDs[i] = e.ID
	}
	statuses, err := s.cards.CardStatusesByEntry(ctx, in.OwnerID, entryIDs)
	if err != nil {
		return ListResult{}, err
	}
	items := make([]EntryListItem, len(entries))
	for i, e := range entries {
		items[i] = EntryListItem{Entry: e, Status: statuses[e.ID]}
	}
	return ListResult{Items: items, Page: page}, nil
}
