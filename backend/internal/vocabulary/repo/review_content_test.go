package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/vocabulary/domain"
)

func TestBatchReviewContent_OwnerAndCurated(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	owner := uuid.New()

	// entry của owner với đầy đủ IPA/nghĩa/ví dụ.
	own := &domain.Entry{
		OwnerID:        ptr(owner),
		Term:           "ephemeral",
		Meanings:       []domain.Meaning{{PartOfSpeech: "adj", Definition: "chóng tàn", Position: 0}},
		Examples:       []domain.Example{{Text: "an ephemeral trend", Position: 0}},
		Pronunciations: []domain.Pronunciation{{IPA: "/ɪˈfɛm(ə)rəl/", Dialect: "GA"}},
	}
	if err := r.Insert(ctx, own); err != nil {
		t.Fatalf("insert own: %v", err)
	}

	// entry curated (owner_id NULL) — thẻ enroll trỏ tới, phải nằm trong kết quả.
	curated := &domain.Entry{Term: "ubiquitous"}
	if err := r.Insert(ctx, curated); err != nil {
		t.Fatalf("insert curated: %v", err)
	}

	// entry của owner KHÁC — không được trả về.
	other := &domain.Entry{OwnerID: ptr(uuid.New()), Term: "trespass"}
	if err := r.Insert(ctx, other); err != nil {
		t.Fatalf("insert other: %v", err)
	}

	got, err := r.BatchReviewContent(ctx, owner, []uuid.UUID{own.ID, curated.ID, other.ID})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}

	byID := make(map[uuid.UUID]ReviewContent, len(got))
	for _, rc := range got {
		byID[rc.EntryID] = rc
	}
	if _, ok := byID[other.ID]; ok {
		t.Error("entry của owner khác không được trả về")
	}
	if _, ok := byID[curated.ID]; !ok {
		t.Error("entry curated (owner_id NULL) phải được trả về")
	}
	o := byID[own.ID]
	if o.Term != "ephemeral" || o.IPA != "/ɪˈfɛm(ə)rəl/" || o.Meaning != "chóng tàn" || o.Example != "an ephemeral trend" {
		t.Errorf("nội dung owner sai: %+v", o)
	}

	// tập rỗng → nil, không lỗi.
	empty, err := r.BatchReviewContent(ctx, owner, nil)
	if err != nil || empty != nil {
		t.Errorf("empty ids = %v, %v; want nil, nil", empty, err)
	}
}
