package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/identity/ports"
)

// Repos gom 4 repo pgx cho module identity.
type Repos struct {
	Users    *UserRepo
	Sessions *SessionRepo
	Tokens   *EmailTokenRepo
	OAuth    *OAuthRepo
}

func New(pool *pgxpool.Pool) *Repos {
	return &Repos{
		Users:    &UserRepo{pool: pool},
		Sessions: &SessionRepo{pool: pool},
		Tokens:   &EmailTokenRepo{pool: pool},
		OAuth:    &OAuthRepo{pool: pool},
	}
}

func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// --- Users ---

type UserRepo struct{ pool *pgxpool.Pool }

const userCols = `id, email, password_hash, display_name, timezone, locale, theme,
	email_verified_at, plan, role, created_at, updated_at, deleted_at`

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	var plan, role string
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.Timezone,
		&u.Locale, &u.Theme, &u.EmailVerifiedAt, &plan, &role,
		&u.CreatedAt, &u.UpdatedAt, &u.DeletedAt); err != nil {
		return nil, mapNotFound(err)
	}
	u.Plan = domain.Plan(plan)
	u.Role = domain.Role(role)
	return &u, nil
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO identity.users
		(id, email, password_hash, display_name, timezone, locale, theme,
		 email_verified_at, plan, role, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		u.ID, u.Email, u.PasswordHash, u.DisplayName, u.Timezone, u.Locale, u.Theme,
		u.EmailVerifiedAt, string(u.Plan), string(u.Role), u.CreatedAt, u.UpdatedAt)
	return err
}

func (r *UserRepo) ByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userCols+`
		FROM identity.users WHERE email = $1 AND deleted_at IS NULL`, email)
	return scanUser(row)
}

func (r *UserRepo) ByID(ctx context.Context, id string) (*domain.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM identity.users WHERE id = $1`, id)
	return scanUser(row)
}

func (r *UserRepo) Update(ctx context.Context, u *domain.User) error {
	ct, err := r.pool.Exec(ctx, `UPDATE identity.users SET
		email=$2, password_hash=$3, display_name=$4, timezone=$5, locale=$6,
		theme=$7, email_verified_at=$8, plan=$9, role=$10, updated_at=$11
		WHERE id=$1`,
		u.ID, u.Email, u.PasswordHash, u.DisplayName, u.Timezone, u.Locale, u.Theme,
		u.EmailVerifiedAt, string(u.Plan), string(u.Role), u.UpdatedAt)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepo) SoftDelete(ctx context.Context, id string, at time.Time) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE identity.users SET deleted_at=$2, updated_at=$2 WHERE id=$1 AND deleted_at IS NULL`, id, at)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepo) PurgeDeletedBefore(ctx context.Context, cutoff time.Time) (int, error) {
	ct, err := r.pool.Exec(ctx,
		`DELETE FROM identity.users WHERE deleted_at IS NOT NULL AND deleted_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return int(ct.RowsAffected()), nil
}

// --- Sessions ---

type SessionRepo struct{ pool *pgxpool.Pool }

func (r *SessionRepo) Create(ctx context.Context, s *domain.Session) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO identity.sessions
		(id, user_id, family_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		s.ID, s.UserID, s.FamilyID, s.RefreshTokenHash, s.ExpiresAt, s.CreatedAt)
	return err
}

func (r *SessionRepo) ByTokenHash(ctx context.Context, hash string) (*domain.Session, error) {
	var s domain.Session
	err := r.pool.QueryRow(ctx, `SELECT id, user_id, family_id, refresh_token_hash,
		rotated_to, expires_at, revoked_at, created_at
		FROM identity.sessions WHERE refresh_token_hash = $1`, hash).
		Scan(&s.ID, &s.UserID, &s.FamilyID, &s.RefreshTokenHash,
			&s.RotatedTo, &s.ExpiresAt, &s.RevokedAt, &s.CreatedAt)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &s, nil
}

func (r *SessionRepo) MarkRotated(ctx context.Context, id, successorID string, at time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE identity.sessions SET rotated_to=$2, revoked_at=$3 WHERE id=$1`, id, successorID, at)
	return err
}

func (r *SessionRepo) RevokeFamily(ctx context.Context, familyID string, at time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE identity.sessions SET revoked_at=$2 WHERE family_id=$1 AND revoked_at IS NULL`, familyID, at)
	return err
}

func (r *SessionRepo) RevokeAllForUser(ctx context.Context, userID string, at time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE identity.sessions SET revoked_at=$2 WHERE user_id=$1 AND revoked_at IS NULL`, userID, at)
	return err
}

// --- Email tokens ---

type EmailTokenRepo struct{ pool *pgxpool.Pool }

func (r *EmailTokenRepo) Create(ctx context.Context, t *domain.EmailToken) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO identity.email_tokens
		(id, user_id, kind, token_hash, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		t.ID, t.UserID, string(t.Kind), t.TokenHash, t.ExpiresAt, t.CreatedAt)
	return err
}

func (r *EmailTokenRepo) ByTokenHash(ctx context.Context, hash string, kind domain.TokenKind) (*domain.EmailToken, error) {
	var t domain.EmailToken
	var k string
	err := r.pool.QueryRow(ctx, `SELECT id, user_id, kind, token_hash, expires_at, used_at, created_at
		FROM identity.email_tokens WHERE token_hash=$1 AND kind=$2`, hash, string(kind)).
		Scan(&t.ID, &t.UserID, &k, &t.TokenHash, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if err != nil {
		return nil, mapNotFound(err)
	}
	t.Kind = domain.TokenKind(k)
	return &t, nil
}

func (r *EmailTokenRepo) MarkUsed(ctx context.Context, id string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE identity.email_tokens SET used_at=$2 WHERE id=$1`, id, at)
	return err
}

// --- OAuth identities ---

type OAuthRepo struct{ pool *pgxpool.Pool }

func (r *OAuthRepo) ByProviderUID(ctx context.Context, provider, uid string) (*domain.OAuthIdentity, error) {
	var o domain.OAuthIdentity
	err := r.pool.QueryRow(ctx, `SELECT id, user_id, provider, provider_uid, created_at
		FROM identity.oauth_identities WHERE provider=$1 AND provider_uid=$2`, provider, uid).
		Scan(&o.ID, &o.UserID, &o.Provider, &o.ProviderUID, &o.CreatedAt)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &o, nil
}

func (r *OAuthRepo) Create(ctx context.Context, o *domain.OAuthIdentity) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO identity.oauth_identities
		(id, user_id, provider, provider_uid, created_at) VALUES ($1,$2,$3,$4,$5)`,
		o.ID, o.UserID, o.Provider, o.ProviderUID, o.CreatedAt)
	return err
}

// Compile-time checks.
var (
	_ ports.UserRepo       = (*UserRepo)(nil)
	_ ports.SessionRepo    = (*SessionRepo)(nil)
	_ ports.EmailTokenRepo = (*EmailTokenRepo)(nil)
	_ ports.OAuthRepo      = (*OAuthRepo)(nil)
)
