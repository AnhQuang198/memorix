// Package ports khai báo cổng (interface) mà review service phụ thuộc: repo
// append-only log, receipt idempotency, và VocabularyPort chéo module (AD-9).
package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/db"
	revdom "github.com/memorix/memorix/internal/review/domain"
)

// ReviewLogRepo append + đọc log (AD-4).
type ReviewLogRepo interface {
	Append(ctx context.Context, q db.Querier, row revdom.ReviewLogRow) error
	// ListForOwnerSince trả log của owner từ mốc `sinceRFC3339` (dùng cho summary + replay).
	ListForOwnerSince(ctx context.Context, q db.Querier, ownerID uuid.UUID, sinceRFC3339 string) ([]revdom.ReviewLogRow, error)
	// ListForCard trả log 1 card theo thứ tự reviewed_at tăng dần (replay AD-4).
	ListForCard(ctx context.Context, q db.Querier, cardID uuid.UUID) ([]revdom.ReviewLogRow, error)
}

// ReceiptRepo = idempotency guard (AD-3): unique(card_id, client_review_id).
type ReceiptRepo interface {
	// Insert trả (true) nếu chèn mới, (false) nếu đã tồn tại (ON CONFLICT DO NOTHING).
	Insert(ctx context.Context, q db.Querier, r revdom.GradeResult, reviewLogID uuid.UUID, clientReviewID string) (bool, error)
	// Get trả kết quả cũ để idempotent-return; ok=false nếu chưa có.
	Get(ctx context.Context, q db.Querier, cardID uuid.UUID, clientReviewID string) (revdom.GradeResult, bool, error)
}

// EntryContent = nội dung entry batch-load qua VocabularyPort (AD-9). Chỉ field
// cần cho mặt sau thẻ; review KHÔNG join bảng vocabulary.
type EntryContent struct {
	EntryID uuid.UUID
	Term    string
	IPA     string
	Meaning string
	Example string
}

// VocabularyPort = port chéo module (định nghĩa ở caller, addendum §chống import cycle).
type VocabularyPort interface {
	BatchGet(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]EntryContent, error)
}

// VocabularyFunc adapter hàm → VocabularyPort (cmd wiring bọc port thật của vocabulary).
type VocabularyFunc func(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]EntryContent, error)

func (f VocabularyFunc) BatchGet(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]EntryContent, error) {
	return f(ctx, ownerID, entryIDs)
}
