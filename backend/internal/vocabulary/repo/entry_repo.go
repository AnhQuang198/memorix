// Package repo là adapter Postgres của vocabulary (pgx).
package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
)

// cursorTimeLayout dùng cho SortKey của cursor (created_at).
const cursorTimeLayout = time.RFC3339Nano

const pgUniqueViolation = "23505"

type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

// Insert ghi entry + bảng con trong 1 transaction; set e.ID/CreatedAt.
func (r *Repo) Insert(ctx context.Context, e *domain.Entry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx,
		`INSERT INTO vocabulary.entries (owner_id, curated_deck_id, term, part_of_speech, notes, source)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, created_at, updated_at`,
		e.OwnerID, e.CuratedDeckID, e.Term, e.PartOfSpeech, e.Notes, e.Source).
		Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if isUnique(err) {
			return domain.ErrDuplicateTerm
		}
		return err
	}
	if err := insertChildren(ctx, tx, e); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertChildren(ctx context.Context, tx pgx.Tx, e *domain.Entry) error {
	for _, m := range e.Meanings {
		if _, err := tx.Exec(ctx,
			`INSERT INTO vocabulary.meanings (entry_id, part_of_speech, definition, position)
			 VALUES ($1,$2,$3,$4)`, e.ID, m.PartOfSpeech, m.Definition, m.Position); err != nil {
			return err
		}
	}
	for _, ex := range e.Examples {
		if _, err := tx.Exec(ctx,
			`INSERT INTO vocabulary.examples (entry_id, text, position) VALUES ($1,$2,$3)`,
			e.ID, ex.Text, ex.Position); err != nil {
			return err
		}
	}
	for _, p := range e.Pronunciations {
		if _, err := tx.Exec(ctx,
			`INSERT INTO vocabulary.pronunciations (entry_id, ipa, dialect, audio_url) VALUES ($1,$2,$3,$4)`,
			e.ID, p.IPA, p.Dialect, p.AudioURL); err != nil {
			return err
		}
	}
	for _, s := range e.Relations {
		if _, err := tx.Exec(ctx,
			`INSERT INTO vocabulary.synonyms_antonyms (entry_id, relation, value) VALUES ($1,$2,$3)`,
			e.ID, string(s.Relation), s.Value); err != nil {
			return err
		}
	}
	return nil
}

// FindByID trả entry + toàn bộ bảng con (cho màn detail).
func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Entry, error) {
	var e domain.Entry
	err := r.pool.QueryRow(ctx,
		`SELECT id, owner_id, curated_deck_id, term, part_of_speech, notes, source,
		        created_at, updated_at, deleted_at
		 FROM vocabulary.entries WHERE id = $1 AND deleted_at IS NULL`, id).
		Scan(&e.ID, &e.OwnerID, &e.CuratedDeckID, &e.Term, &e.PartOfSpeech, &e.Notes, &e.Source,
			&e.CreatedAt, &e.UpdatedAt, &e.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrEntryNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := loadChildren(ctx, r.pool, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

func loadChildren(ctx context.Context, q *pgxpool.Pool, e *domain.Entry) error {
	mrows, err := q.Query(ctx,
		`SELECT id, part_of_speech, definition, position FROM vocabulary.meanings
		 WHERE entry_id = $1 ORDER BY position, id`, e.ID)
	if err != nil {
		return err
	}
	for mrows.Next() {
		var m domain.Meaning
		if err := mrows.Scan(&m.ID, &m.PartOfSpeech, &m.Definition, &m.Position); err != nil {
			mrows.Close()
			return err
		}
		e.Meanings = append(e.Meanings, m)
	}
	mrows.Close()

	exrows, err := q.Query(ctx,
		`SELECT id, text, position FROM vocabulary.examples WHERE entry_id = $1 ORDER BY position, id`, e.ID)
	if err != nil {
		return err
	}
	for exrows.Next() {
		var x domain.Example
		if err := exrows.Scan(&x.ID, &x.Text, &x.Position); err != nil {
			exrows.Close()
			return err
		}
		e.Examples = append(e.Examples, x)
	}
	exrows.Close()

	prows, err := q.Query(ctx,
		`SELECT id, ipa, dialect, audio_url FROM vocabulary.pronunciations WHERE entry_id = $1 ORDER BY id`, e.ID)
	if err != nil {
		return err
	}
	for prows.Next() {
		var p domain.Pronunciation
		if err := prows.Scan(&p.ID, &p.IPA, &p.Dialect, &p.AudioURL); err != nil {
			prows.Close()
			return err
		}
		e.Pronunciations = append(e.Pronunciations, p)
	}
	prows.Close()

	srows, err := q.Query(ctx,
		`SELECT id, relation, value FROM vocabulary.synonyms_antonyms WHERE entry_id = $1 ORDER BY id`, e.ID)
	if err != nil {
		return err
	}
	for srows.Next() {
		var s domain.SynAnt
		var rel string
		if err := srows.Scan(&s.ID, &rel, &s.Value); err != nil {
			srows.Close()
			return err
		}
		s.Relation = domain.Relation(rel)
		e.Relations = append(e.Relations, s)
	}
	srows.Close()
	return srows.Err()
}

// ExistingID tìm entry chưa xóa của owner có term chuẩn hóa trùng (FR-10).
func (r *Repo) ExistingID(ctx context.Context, ownerID uuid.UUID, term string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM vocabulary.entries
		 WHERE owner_id = $1
		   AND term_normalized = vocabulary.immutable_unaccent(lower($2))
		   AND deleted_at IS NULL
		 LIMIT 1`, ownerID, term).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return id, true, nil
}

// Update ghi lại scalar fields + thay toàn bộ bảng con (giữ nguyên card FSRS).
func (r *Repo) Update(ctx context.Context, e *domain.Entry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`UPDATE vocabulary.entries
		 SET term=$2, part_of_speech=$3, notes=$4, source=$5, updated_at=now()
		 WHERE id=$1 AND deleted_at IS NULL`,
		e.ID, e.Term, e.PartOfSpeech, e.Notes, e.Source)
	if err != nil {
		if isUnique(err) {
			return domain.ErrDuplicateTerm
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEntryNotFound
	}
	for _, tbl := range []string{"meanings", "examples", "pronunciations", "synonyms_antonyms"} {
		if _, err := tx.Exec(ctx, "DELETE FROM vocabulary."+tbl+" WHERE entry_id=$1", e.ID); err != nil {
			return err
		}
	}
	if err := insertChildren(ctx, tx, e); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SoftDelete đặt deleted_at (FR-9; card + log giữ tới purge).
func (r *Repo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE vocabulary.entries SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEntryNotFound
	}
	return nil
}

// ListPage phân trang entries của owner theo (created_at DESC, id DESC). q FTS optional.
func (r *Repo) ListPage(ctx context.Context, ownerID uuid.UUID, q string, cur httpx.Cursor, limit int) ([]domain.Entry, error) {
	var curTime *time.Time
	var curID *uuid.UUID
	if cur.ID != "" {
		t, err := time.Parse(cursorTimeLayout, cur.SortKey)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(cur.ID)
		if err != nil {
			return nil, err
		}
		curTime, curID = &t, &id
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, owner_id, curated_deck_id, term, part_of_speech, notes, source, created_at, updated_at
		 FROM vocabulary.entries
		 WHERE owner_id = $1 AND deleted_at IS NULL
		   AND ($2 = '' OR to_tsvector('english', term || ' ' || notes) @@ plainto_tsquery('english', $2))
		   AND ($3::timestamptz IS NULL OR (created_at, id) < ($3, $4))
		 ORDER BY created_at DESC, id DESC
		 LIMIT $5`,
		ownerID, q, curTime, curID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntryList(rows)
}

// ListPageByIDs như ListPage nhưng giới hạn trong tập id (lọc theo status từ scheduling).
func (r *Repo) ListPageByIDs(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID, cur httpx.Cursor, limit int) ([]domain.Entry, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var curTime *time.Time
	var curID *uuid.UUID
	if cur.ID != "" {
		t, err := time.Parse(cursorTimeLayout, cur.SortKey)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(cur.ID)
		if err != nil {
			return nil, err
		}
		curTime, curID = &t, &id
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, owner_id, curated_deck_id, term, part_of_speech, notes, source, created_at, updated_at
		 FROM vocabulary.entries
		 WHERE owner_id = $1 AND deleted_at IS NULL AND id = ANY($2)
		   AND ($3::timestamptz IS NULL OR (created_at, id) < ($3, $4))
		 ORDER BY created_at DESC, id DESC
		 LIMIT $5`,
		ownerID, ids, curTime, curID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntryList(rows)
}

func scanEntryList(rows pgx.Rows) ([]domain.Entry, error) {
	var out []domain.Entry
	for rows.Next() {
		var e domain.Entry
		if err := rows.Scan(&e.ID, &e.OwnerID, &e.CuratedDeckID, &e.Term, &e.PartOfSpeech,
			&e.Notes, &e.Source, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
