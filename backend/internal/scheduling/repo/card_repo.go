// Package repo là adapter Postgres của scheduling (implements ports).
package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CardRepo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *CardRepo { return &CardRepo{pool: pool} }

// CreateCardsForEntry tạo 1 card New / direction (idempotent qua unique).
func (r *CardRepo) CreateCardsForEntry(ctx context.Context, ownerID, entryID uuid.UUID, directions []string) error {
	for _, d := range directions {
		_, err := r.pool.Exec(ctx,
			`INSERT INTO scheduling.cards (owner_id, entry_id, direction, status)
			 VALUES ($1, $2, $3, 'new')
			 ON CONFLICT (owner_id, entry_id, direction) DO NOTHING`,
			ownerID, entryID, d)
		if err != nil {
			return err
		}
	}
	return nil
}

// CardStatusesByEntry trả status card primary (front_back) theo entry.
func (r *CardRepo) CardStatusesByEntry(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(entryIDs))
	if len(entryIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT entry_id, status FROM scheduling.cards
		 WHERE owner_id = $1 AND entry_id = ANY($2) AND direction = 'front_back' AND deleted_at IS NULL`,
		ownerID, entryIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var st string
		if err := rows.Scan(&id, &st); err != nil {
			return nil, err
		}
		out[id] = st
	}
	return out, rows.Err()
}

// EntryIDsByStatus trả entry có >=1 card ở status (lọc list, không join chéo schema).
func (r *CardRepo) EntryIDsByStatus(ctx context.Context, ownerID uuid.UUID, status string) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT entry_id FROM scheduling.cards
		 WHERE owner_id = $1 AND status = $2 AND deleted_at IS NULL`,
		ownerID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// BulkCreateForDeck tạo card New (front_back) cho mỗi entry; trả số card mới tạo.
func (r *CardRepo) BulkCreateForDeck(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (int, error) {
	if len(entryIDs) == 0 {
		return 0, nil
	}
	tag, err := r.pool.Exec(ctx,
		`INSERT INTO scheduling.cards (owner_id, entry_id, direction, status)
		 SELECT $1, e, 'front_back', 'new' FROM unnest($2::uuid[]) AS e
		 ON CONFLICT (owner_id, entry_id, direction) DO NOTHING`,
		ownerID, entryIDs)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
