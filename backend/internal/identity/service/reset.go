package service

import (
	"context"
	"errors"

	"github.com/memorix/memorix/internal/identity/domain"
)

// RequestReset phát token kind=reset (TTL 1h). Response phía handler GIỐNG NHAU
// dù email tồn tại hay không (Story 1.6, chống enumeration); raw chỉ để gửi mail.
func (s *Service) RequestReset(ctx context.Context, email string) (string, error) {
	email = domain.NormalizeEmail(email)
	u, err := s.deps.Users.ByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	now := s.deps.Clock.Now()
	raw, hash := s.deps.Secrets.New()
	tok := &domain.EmailToken{
		ID: newID(), UserID: u.ID, Kind: domain.KindReset,
		TokenHash: hash, ExpiresAt: now.Add(s.deps.ResetTTL), CreatedAt: now,
	}
	if err := s.deps.Tokens.Create(ctx, tok); err != nil {
		return "", err
	}
	return raw, nil
}

// ResetPassword đặt mật khẩu mới, tiêu thụ token, và THU HỒI mọi session (Story 1.6).
func (s *Service) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	if !domain.PasswordStrongEnough(newPassword) {
		return domain.ErrWeakPassword
	}
	hash := s.deps.Secrets.Hash(rawToken)
	tok, err := s.deps.Tokens.ByTokenHash(ctx, hash, domain.KindReset)
	if err != nil {
		return domain.ErrTokenInvalid
	}
	now := s.deps.Clock.Now()
	if !tok.Usable(now) {
		return domain.ErrTokenInvalid
	}
	ph, err := s.deps.Hasher.Hash(newPassword)
	if err != nil {
		return err
	}
	u, err := s.deps.Users.ByID(ctx, tok.UserID)
	if err != nil {
		return err
	}
	u.PasswordHash = ph
	u.UpdatedAt = now
	if err := s.deps.Users.Update(ctx, u); err != nil {
		return err
	}
	if err := s.deps.Tokens.MarkUsed(ctx, tok.ID, now); err != nil {
		return err
	}
	return s.deps.Sessions.RevokeAllForUser(ctx, u.ID, now)
}
