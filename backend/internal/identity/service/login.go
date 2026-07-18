package service

import (
	"context"
	"errors"

	"github.com/memorix/memorix/internal/identity/domain"
)

// Login xác thực email+password, trả token pair (Story 1.4). Rate-limit theo
// email (NFR-10) + constant-time compare khi email không tồn tại (chống
// enumeration): mọi thất bại credential trả CÙNG ErrInvalidCredentials.
func (s *Service) Login(ctx context.Context, email, password string) (TokenPair, error) {
	email = domain.NormalizeEmail(email)
	key := "login:" + email
	if ok, _ := s.deps.Limiter.Allow(ctx, key); !ok {
		return TokenPair{}, domain.ErrRateLimited
	}

	u, err := s.deps.Users.ByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return TokenPair{}, err
		}
		// user không tồn tại: burn thời gian để không lộ qua timing.
		_, _ = s.deps.Hasher.Verify(password, s.dummyHash)
		return TokenPair{}, domain.ErrInvalidCredentials
	}

	ok, err := s.deps.Hasher.Verify(password, u.PasswordHash)
	if err != nil {
		return TokenPair{}, err
	}
	if !ok {
		return TokenPair{}, domain.ErrInvalidCredentials
	}

	s.deps.Limiter.Reset(ctx, key)
	return s.issueSession(ctx, u)
}
