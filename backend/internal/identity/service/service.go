package service

import (
	"context"
	"time"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/identity/ports"
	"github.com/memorix/memorix/internal/platform/eventbus"
)

// Deps gom mọi cộng tác của Service (constructor injection, dễ test bằng fake).
type Deps struct {
	Users      ports.UserRepo
	Sessions   ports.SessionRepo
	Tokens     ports.EmailTokenRepo
	OAuth      ports.OAuthRepo
	Hasher     ports.Hasher
	Issuer     ports.TokenIssuer
	Secrets    ports.TokenFactory
	Clock      ports.Clock
	Limiter    ports.LoginLimiter
	OIDC       ports.OIDCVerifier
	Bus        eventbus.Bus
	RefreshTTL time.Duration
	VerifyTTL  time.Duration
	ResetTTL   time.Duration
}

// Service là use-case layer module identity (hexagonal-nhẹ). Principal đến từ
// handler (authmw), không lộ xuống domain.
type Service struct {
	deps      Deps
	dummyHash string
}

func New(d Deps) *Service {
	// dummyHash cho constant-time compare khi email không tồn tại (chống enumeration).
	dummy, _ := d.Hasher.Hash("memorix-timing-guard")
	return &Service{deps: d, dummyHash: dummy}
}

// TokenPair trả về sau login/register/refresh. RefreshToken là opaque raw,
// handler set vào cookie httpOnly+Secure+SameSite=Strict.
type TokenPair struct {
	AccessToken     string
	AccessExpiresAt time.Time
	RefreshToken    string
}

// issueSession tạo family session mới + access token (dùng chung login/register/oauth).
func (s *Service) issueSession(ctx context.Context, u *domain.User) (TokenPair, error) {
	now := s.deps.Clock.Now()
	raw, hash := s.deps.Secrets.New()
	sess := &domain.Session{
		ID:               newID(),
		UserID:           u.ID,
		FamilyID:         newID(),
		RefreshTokenHash: hash,
		ExpiresAt:        now.Add(s.deps.RefreshTTL),
		CreatedAt:        now,
	}
	if err := s.deps.Sessions.Create(ctx, sess); err != nil {
		return TokenPair{}, err
	}
	access, exp, err := s.deps.Issuer.Issue(u.ID, string(u.Role), string(u.Plan))
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, AccessExpiresAt: exp, RefreshToken: raw}, nil
}
