package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/ports"
)

// CardStore = adapter pgx cho scheduling.cards (implements ports.CardStore).
// Nhận db.Querier để cùng code chạy trong TX (unit-of-work) hoặc ngoài (read).
type CardStore struct{}

// NewCardStore trả adapter stateless (Querier truyền theo call).
func NewCardStore() *CardStore { return &CardStore{} }

// compile-time port check.
var _ ports.CardStore = (*CardStore)(nil)

const cardCols = `id, owner_id, entry_id, direction, stability, difficulty,
	status, reps, lapses, due_at, last_review_at, created_at, updated_at`

func scanCard(row pgx.Row) (domain.Card, error) {
	var c domain.Card
	err := row.Scan(&c.ID, &c.OwnerID, &c.EntryID, &c.Direction, &c.Stability,
		&c.Difficulty, &c.Status, &c.Reps, &c.Lapses, &c.DueAt, &c.LastReviewAt,
		&c.CreatedAt, &c.UpdatedAt)
	return c, err
}

// Load đọc FSRS state của card theo id, owner-scoped (deny-by-default → ErrCardNotFound).
func (s *CardStore) Load(ctx context.Context, q db.Querier, cardID, ownerID uuid.UUID) (domain.Card, error) {
	row := q.QueryRow(ctx, `SELECT `+cardCols+`
		FROM scheduling.cards
		WHERE id=$1 AND owner_id=$2 AND deleted_at IS NULL`, cardID, ownerID)
	c, err := scanCard(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Card{}, domain.ErrCardNotFound
	}
	if err != nil {
		return domain.Card{}, err
	}
	return c, nil
}

// ApplyResult ghi trạng thái card sau khi chấm (S/D/status/reps/lapses/due/last_review).
func (s *CardStore) ApplyResult(ctx context.Context, q db.Querier, cardID uuid.UUID, r domain.ScheduleResult) error {
	_, err := q.Exec(ctx, `
		UPDATE scheduling.cards
		SET stability=$2, difficulty=$3, status=$4, reps=$5, lapses=$6,
		    due_at=$7, last_review_at=$8, updated_at=now()
		WHERE id=$1`,
		cardID, r.Stability, r.Difficulty, r.Status, r.Reps, r.Lapses, r.DueAt, r.LastReviewAt)
	return err
}

// DueCards trả card tới hạn (due_at<=now) của owner, cũ nhất trước, giới hạn limit
// (hot path (owner_id, due_at) — NFR-2).
func (s *CardStore) DueCards(ctx context.Context, q db.Querier, ownerID uuid.UUID, now time.Time, limit int) ([]domain.Card, error) {
	rows, err := q.Query(ctx, `SELECT `+cardCols+`
		FROM scheduling.cards
		WHERE owner_id=$1 AND due_at<=$2 AND deleted_at IS NULL
		ORDER BY due_at ASC
		LIMIT $3`, ownerID, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
