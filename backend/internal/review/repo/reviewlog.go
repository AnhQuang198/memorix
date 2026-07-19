// Package repo implement pgx adapter cho review: append-only ReviewLogRepo (AD-4)
// và ReceiptRepo idempotency guard (AD-3). Mọi method nhận db.Querier nên chạy
// được cả trong TX chấm điểm (unit-of-work) lẫn ngoài TX (đọc thuần).
package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/memorix/memorix/internal/platform/db"
	revdom "github.com/memorix/memorix/internal/review/domain"
	"github.com/memorix/memorix/internal/review/ports"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
)

// Cột status ở review.review_logs/grade_receipts là smallint, còn domain
// CardStatus là string; repo là ranh giới nên dịch tường minh giữa hai biểu diễn.
// Thứ tự khớp go-fsrs State (New=0..Relearning=3); Suspended (không có ở go-fsrs)
// nối tiếp =4. Bijection ổn định, chỉ repo này đọc/ghi các cột đó.
const (
	statusCodeNew        int16 = 0
	statusCodeLearning   int16 = 1
	statusCodeReview     int16 = 2
	statusCodeRelearning int16 = 3
	statusCodeSuspended  int16 = 4
)

func statusCode(s scheddom.CardStatus) int16 {
	switch s {
	case scheddom.StatusLearning:
		return statusCodeLearning
	case scheddom.StatusReview:
		return statusCodeReview
	case scheddom.StatusRelearning:
		return statusCodeRelearning
	case scheddom.StatusSuspended:
		return statusCodeSuspended
	case scheddom.StatusNew:
		return statusCodeNew
	default:
		return statusCodeNew
	}
}

func statusFrom(code int16) scheddom.CardStatus {
	switch code {
	case statusCodeLearning:
		return scheddom.StatusLearning
	case statusCodeReview:
		return scheddom.StatusReview
	case statusCodeRelearning:
		return scheddom.StatusRelearning
	case statusCodeSuspended:
		return scheddom.StatusSuspended
	case statusCodeNew:
		return scheddom.StatusNew
	default:
		return scheddom.StatusNew
	}
}

// ReviewLogRepo ghi/đọc review.review_logs (append-only).
type ReviewLogRepo struct{}

// NewReviewLogRepo tạo ReviewLogRepo (stateless).
func NewReviewLogRepo() *ReviewLogRepo { return &ReviewLogRepo{} }

const logCols = `id, card_id, owner_id, client_review_id, grade,
	prev_stability, prev_difficulty, prev_status, retrievability,
	new_stability, new_difficulty, new_status, new_reps, new_lapses, new_due_at,
	elapsed_days, reviewed_at`

// Append chèn một dòng log; chạy trong TX chấm điểm qua db.Querier.
func (r *ReviewLogRepo) Append(ctx context.Context, q db.Querier, row revdom.ReviewLogRow) error {
	_, err := q.Exec(ctx, `
		INSERT INTO review.review_logs (`+logCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		row.ID, row.CardID, row.OwnerID, row.ClientReviewID, int16(row.Grade),
		row.PrevStability, row.PrevDifficulty, statusCode(row.PrevStatus), row.Retrievability,
		row.NewStability, row.NewDifficulty, statusCode(row.NewStatus), row.NewReps, row.NewLapses, row.NewDueAt,
		row.ElapsedDays, row.ReviewedAt)
	return err
}

func scanLog(rows pgx.Rows) (revdom.ReviewLogRow, error) {
	var x revdom.ReviewLogRow
	var grade, prevStatus, newStatus int16
	err := rows.Scan(&x.ID, &x.CardID, &x.OwnerID, &x.ClientReviewID, &grade,
		&x.PrevStability, &x.PrevDifficulty, &prevStatus, &x.Retrievability,
		&x.NewStability, &x.NewDifficulty, &newStatus, &x.NewReps, &x.NewLapses, &x.NewDueAt,
		&x.ElapsedDays, &x.ReviewedAt)
	if err != nil {
		return revdom.ReviewLogRow{}, err
	}
	x.Grade = scheddom.Grade(grade)
	x.PrevStatus = statusFrom(prevStatus)
	x.NewStatus = statusFrom(newStatus)
	return x, nil
}

func collect(rows pgx.Rows) ([]revdom.ReviewLogRow, error) {
	defer rows.Close()
	var out []revdom.ReviewLogRow
	for rows.Next() {
		x, err := scanLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// ListForCard trả log một card theo reviewed_at tăng dần (replay AD-4).
func (r *ReviewLogRepo) ListForCard(ctx context.Context, q db.Querier, cardID uuid.UUID) ([]revdom.ReviewLogRow, error) {
	rows, err := q.Query(ctx, `SELECT `+logCols+`
		FROM review.review_logs WHERE card_id=$1 ORDER BY reviewed_at ASC`, cardID)
	if err != nil {
		return nil, err
	}
	return collect(rows)
}

// ListForOwnerSince trả log của owner từ mốc sinceRFC3339 (summary + replay).
func (r *ReviewLogRepo) ListForOwnerSince(ctx context.Context, q db.Querier, ownerID uuid.UUID, sinceRFC3339 string) ([]revdom.ReviewLogRow, error) {
	rows, err := q.Query(ctx, `SELECT `+logCols+`
		FROM review.review_logs
		WHERE owner_id=$1 AND reviewed_at >= $2::timestamptz
		ORDER BY reviewed_at ASC`, ownerID, sinceRFC3339)
	if err != nil {
		return nil, err
	}
	return collect(rows)
}

// compile-time: ReviewLogRepo thỏa ports.ReviewLogRepo.
var _ ports.ReviewLogRepo = (*ReviewLogRepo)(nil)
