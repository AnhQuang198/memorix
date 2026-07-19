package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/memorix/memorix/internal/platform/db"
	revdom "github.com/memorix/memorix/internal/review/domain"
	"github.com/memorix/memorix/internal/review/ports"
)

// ReceiptRepo = idempotency guard (AD-3): unique(card_id, client_review_id).
type ReceiptRepo struct{}

// NewReceiptRepo tạo ReceiptRepo (stateless).
func NewReceiptRepo() *ReceiptRepo { return &ReceiptRepo{} }

// Insert chèn receipt; trả true nếu chèn mới, false nếu đã tồn tại
// (ON CONFLICT DO NOTHING → RETURNING không có row).
func (r *ReceiptRepo) Insert(ctx context.Context, q db.Querier, res revdom.GradeResult, reviewLogID uuid.UUID, clientReviewID string) (bool, error) {
	var out uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO review.grade_receipts
		  (card_id, client_review_id, review_log_id, new_stability, new_difficulty,
		   new_status, new_reps, new_lapses, new_due_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (card_id, client_review_id) DO NOTHING
		RETURNING card_id`,
		res.CardID, clientReviewID, reviewLogID, res.Stability, res.Difficulty,
		statusCode(res.Status), res.Reps, res.Lapses, res.DueAt).Scan(&out)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // đã tồn tại
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Get trả receipt cũ để idempotent-return; ok=false nếu chưa có.
func (r *ReceiptRepo) Get(ctx context.Context, q db.Querier, cardID uuid.UUID, clientReviewID string) (revdom.GradeResult, bool, error) {
	res := revdom.GradeResult{CardID: cardID}
	var status int16
	err := q.QueryRow(ctx, `
		SELECT new_stability, new_difficulty, new_status, new_reps, new_lapses, new_due_at
		FROM review.grade_receipts WHERE card_id=$1 AND client_review_id=$2`,
		cardID, clientReviewID).Scan(&res.Stability, &res.Difficulty, &status, &res.Reps, &res.Lapses, &res.DueAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return revdom.GradeResult{}, false, nil
	}
	if err != nil {
		return revdom.GradeResult{}, false, err
	}
	res.Status = statusFrom(status)
	return res, true, nil
}

// compile-time: ReceiptRepo thỏa ports.ReceiptRepo.
var _ ports.ReceiptRepo = (*ReceiptRepo)(nil)
