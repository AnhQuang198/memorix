package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/memorix/memorix/internal/vocabulary/domain"
)

// ListActiveDecks trả bộ curated đang bật (onboarding + empty-state).
func (r *Repo) ListActiveDecks(ctx context.Context) ([]domain.CuratedDeck, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, slug, name, description, is_active FROM vocabulary.curated_decks
		 WHERE is_active = true ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.CuratedDeck
	for rows.Next() {
		var d domain.CuratedDeck
		if err := rows.Scan(&d.ID, &d.Slug, &d.Name, &d.Description, &d.IsActive); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// FindDeckByID trả deck theo id.
func (r *Repo) FindDeckByID(ctx context.Context, id uuid.UUID) (domain.CuratedDeck, error) {
	var d domain.CuratedDeck
	err := r.pool.QueryRow(ctx,
		`SELECT id, slug, name, description, is_active FROM vocabulary.curated_decks WHERE id = $1`, id).
		Scan(&d.ID, &d.Slug, &d.Name, &d.Description, &d.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CuratedDeck{}, domain.ErrDeckNotFound
	}
	return d, err
}

// CuratedEntryIDs trả id các entry curated (owner NULL) trong deck.
func (r *Repo) CuratedEntryIDs(ctx context.Context, deckID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id FROM vocabulary.entries
		 WHERE curated_deck_id = $1 AND owner_id IS NULL AND deleted_at IS NULL ORDER BY id`, deckID)
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

// InsertEnrollment tạo bản ghi enroll; trùng (owner, deck) -> ErrAlreadyEnrolled (FR-11b).
func (r *Repo) InsertEnrollment(ctx context.Context, ownerID, deckID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`INSERT INTO vocabulary.deck_enrollments (owner_id, curated_deck_id) VALUES ($1,$2) RETURNING id`,
		ownerID, deckID).Scan(&id)
	if err != nil {
		if isUnique(err) {
			return uuid.Nil, domain.ErrAlreadyEnrolled
		}
		return uuid.Nil, err
	}
	return id, nil
}

// CompleteEnrollment đánh dấu enroll hoàn tất + số card đã tạo (job idempotent).
func (r *Repo) CompleteEnrollment(ctx context.Context, ownerID, deckID uuid.UUID, cardCount int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE vocabulary.deck_enrollments
		 SET status='completed', card_count=$3, completed_at=now()
		 WHERE owner_id=$1 AND curated_deck_id=$2`,
		ownerID, deckID, cardCount)
	return err
}
