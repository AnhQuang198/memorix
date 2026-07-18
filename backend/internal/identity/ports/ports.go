package ports

import (
	"context"
	"time"

	"github.com/memorix/memorix/internal/identity/domain"
)

// --- Driven ports (repo) — chỉ FK trong schema identity (AD-10) ---

type UserRepo interface {
	Create(ctx context.Context, u *domain.User) error
	ByEmail(ctx context.Context, email string) (*domain.User, error)
	ByID(ctx context.Context, id string) (*domain.User, error)
	Update(ctx context.Context, u *domain.User) error
	SoftDelete(ctx context.Context, id string, at time.Time) error
	PurgeDeletedBefore(ctx context.Context, cutoff time.Time) (int, error)
}

type SessionRepo interface {
	Create(ctx context.Context, s *domain.Session) error
	ByTokenHash(ctx context.Context, hash string) (*domain.Session, error)
	MarkRotated(ctx context.Context, id, successorID string, at time.Time) error
	RevokeFamily(ctx context.Context, familyID string, at time.Time) error
	RevokeAllForUser(ctx context.Context, userID string, at time.Time) error
}

type EmailTokenRepo interface {
	Create(ctx context.Context, t *domain.EmailToken) error
	ByTokenHash(ctx context.Context, hash string, kind domain.TokenKind) (*domain.EmailToken, error)
	MarkUsed(ctx context.Context, id string, at time.Time) error
}

type OAuthRepo interface {
	ByProviderUID(ctx context.Context, provider, uid string) (*domain.OAuthIdentity, error)
	Create(ctx context.Context, o *domain.OAuthIdentity) error
}

// --- Security / infra ports ---

type Hasher interface {
	Hash(plain string) (string, error)
	Verify(plain, hash string) (bool, error)
}

type TokenFactory interface {
	New() (raw, hash string)
	Hash(raw string) string
}

type TokenIssuer interface {
	Issue(userID, role, plan string) (accessToken string, expiresAt time.Time, err error)
}

type Clock interface{ Now() time.Time }

type LoginLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
	Reset(ctx context.Context, key string)
}

type Mailer interface {
	SendVerification(ctx context.Context, email, rawToken string) error
	SendPasswordReset(ctx context.Context, email, rawToken string) error
}

// OIDCClaims là kết quả verify id_token (AD-11).
type OIDCClaims struct {
	ProviderUID   string
	Email         string
	EmailVerified bool
}

type OIDCVerifier interface {
	Verify(ctx context.Context, provider, code, codeVerifier, redirectURI, nonce string) (OIDCClaims, error)
}

// --- Driving port expose ra module khác (AD-1) ---

type IdentityPort interface {
	UserExists(ctx context.Context, id string) (bool, error)
	UserTimezone(ctx context.Context, id string) (string, error)
}
