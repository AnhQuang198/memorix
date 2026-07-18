package service

import (
	"context"
	"errors"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/platform/eventbus"
)

type RegisterInput struct {
	Email       string
	Password    string
	DisplayName string
}

type RegisterResult struct {
	UserID      string
	Tokens      TokenPair
	VerifyToken string // raw — handler gửi email; cũng trả cho test
}

// Register tạo tài khoản email+password (Story 1.2): argon2id hash, phát access
// token, phát email-token verify (kind=verify, TTL 24h — Story 1.3).
func (s *Service) Register(ctx context.Context, in RegisterInput) (RegisterResult, error) {
	email := domain.NormalizeEmail(in.Email)
	if !domain.PasswordStrongEnough(in.Password) {
		return RegisterResult{}, domain.ErrWeakPassword
	}

	if _, err := s.deps.Users.ByEmail(ctx, email); err == nil {
		return RegisterResult{}, domain.ErrEmailTaken
	} else if !errors.Is(err, domain.ErrNotFound) {
		return RegisterResult{}, err
	}

	hash, err := s.deps.Hasher.Hash(in.Password)
	if err != nil {
		return RegisterResult{}, err
	}

	now := s.deps.Clock.Now()
	u := &domain.User{
		ID:           newID(),
		Email:        email,
		PasswordHash: hash,
		DisplayName:  in.DisplayName,
		Timezone:     "UTC",
		Locale:       "vi",
		Theme:        "system",
		Plan:         domain.PlanFree,
		Role:         domain.RoleUser,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.deps.Users.Create(ctx, u); err != nil {
		return RegisterResult{}, err
	}

	rawTok, tokHash := s.deps.Secrets.New()
	vt := &domain.EmailToken{
		ID:        newID(),
		UserID:    u.ID,
		Kind:      domain.KindVerify,
		TokenHash: tokHash,
		ExpiresAt: now.Add(s.deps.VerifyTTL),
		CreatedAt: now,
	}
	if err := s.deps.Tokens.Create(ctx, vt); err != nil {
		return RegisterResult{}, err
	}

	tokens, err := s.issueSession(ctx, u)
	if err != nil {
		return RegisterResult{}, err
	}

	s.deps.Bus.Publish(ctx, eventbus.Event{Name: "UserRegistered", Payload: u.ID})
	return RegisterResult{UserID: u.ID, Tokens: tokens, VerifyToken: rawTok}, nil
}
