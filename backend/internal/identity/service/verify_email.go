package service

import (
	"context"

	"github.com/memorix/memorix/internal/identity/domain"
)

// VerifyEmail tiêu thụ token kind=verify: set email_verified_at, đánh dấu used
// (1-lần, TTL 24h) — Story 1.3.
func (s *Service) VerifyEmail(ctx context.Context, rawToken string) error {
	hash := s.deps.Secrets.Hash(rawToken)
	tok, err := s.deps.Tokens.ByTokenHash(ctx, hash, domain.KindVerify)
	if err != nil {
		return domain.ErrTokenInvalid
	}
	now := s.deps.Clock.Now()
	if !tok.Usable(now) {
		return domain.ErrTokenInvalid
	}
	if err := s.deps.Tokens.MarkUsed(ctx, tok.ID, now); err != nil {
		return err
	}
	u, err := s.deps.Users.ByID(ctx, tok.UserID)
	if err != nil {
		return err
	}
	u.EmailVerifiedAt = &now
	u.UpdatedAt = now
	return s.deps.Users.Update(ctx, u)
}
