package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/vocabulary/domain"
	"github.com/memorix/memorix/internal/vocabulary/service"
)

type fakeSvc struct {
	created domain.Entry
	dupID   uuid.UUID
	dup     bool
}

func (f *fakeSvc) Create(_ context.Context, owner uuid.UUID, in service.CreateEntryInput) (domain.Entry, error) {
	if f.dup {
		return domain.Entry{}, service.DuplicateError{ExistingID: f.dupID}
	}
	if in.Term == "" {
		return domain.Entry{}, domain.ErrTermRequired
	}
	id := uuid.New()
	f.created = domain.Entry{ID: id, OwnerID: &owner, Term: in.Term}
	return f.created, nil
}
func (f *fakeSvc) Get(_ context.Context, _, id uuid.UUID) (service.EntryView, error) {
	return service.EntryView{Entry: domain.Entry{ID: id, Term: "x"}, Status: "new"}, nil
}
func (f *fakeSvc) Update(_ context.Context, owner, id uuid.UUID, _ service.UpdateEntryInput) (domain.Entry, error) {
	return domain.Entry{ID: id, OwnerID: &owner, Term: "updated"}, nil
}
func (f *fakeSvc) Delete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (f *fakeSvc) List(context.Context, service.ListInput) (service.ListResult, error) {
	return service.ListResult{}, nil
}
func (f *fakeSvc) ListCuratedDecks(context.Context) ([]domain.CuratedDeck, error) {
	return []domain.CuratedDeck{{ID: uuid.New(), Slug: "ielts-starter", Name: "IELTS"}}, nil
}
func (f *fakeSvc) Enroll(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	return uuid.New(), nil
}

// setup dựng engine với principal giả lập qua authmw.SetPrincipal (Auth Contract
// Sprint 1) — KHÔNG dùng key thô "user_id".
func setup(svc VocabService, owner uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		authmw.SetPrincipal(c, authmw.Principal{UserID: owner.String()})
		c.Next()
	})
	h := New(svc)
	RegisterRoutes(r.Group("/api/v1"), h)
	return r
}

func TestCreateEntry_201(t *testing.T) {
	owner := uuid.New()
	r := setup(&fakeSvc{}, owner)
	body, _ := json.Marshal(map[string]any{"term": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vocabulary/entries", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
}

func TestCreateEntry_Duplicate_409(t *testing.T) {
	dupID := uuid.New()
	r := setup(&fakeSvc{dup: true, dupID: dupID}, uuid.New())
	body, _ := json.Marshal(map[string]any{"term": "dup"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vocabulary/entries", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 409 {
		t.Fatalf("status = %d, want 409", w.Code)
	}
	var resp map[string]map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"]["code"] != "CONFLICT" {
		t.Errorf("code = %v, want CONFLICT", resp["error"]["code"])
	}
	fields, _ := resp["error"]["fields"].(map[string]any)
	if fields["existing_id"] != dupID.String() {
		t.Errorf("existing_id = %v, want %s", fields["existing_id"], dupID)
	}
}

func TestCreateEntry_BlankTerm_400(t *testing.T) {
	r := setup(&fakeSvc{}, uuid.New())
	body, _ := json.Marshal(map[string]any{"term": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vocabulary/entries", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestListCuratedDecks_200(t *testing.T) {
	r := setup(&fakeSvc{}, uuid.New())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vocabulary/curated-decks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestEnroll_202(t *testing.T) {
	r := setup(&fakeSvc{}, uuid.New())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vocabulary/curated-decks/"+uuid.New().String()+"/enroll", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 202 {
		t.Errorf("status = %d, want 202", w.Code)
	}
}
