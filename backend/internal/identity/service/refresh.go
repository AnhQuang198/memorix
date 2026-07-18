package service

import (
	"context"

	"github.com/memorix/memorix/internal/identity/domain"
)

// Refresh xoay vòng refresh token (Story 1.4, AD-11). Token opaque, tra bằng
// hash. Nếu token đã bị revoke (đã xoay) mà bị dùng lại → reuse-detection:
// revoke toàn bộ family. Token hợp lệ → tạo session kế cùng family, đánh dấu
// token cũ rotated.
func (s *Service) Refresh(ctx context.Context, rawToken string) (TokenPair, error) {
	hash := s.deps.Secrets.Hash(rawToken)
	sess, err := s.deps.Sessions.ByTokenHash(ctx, hash)
	if err != nil {
		return TokenPair{}, domain.ErrTokenInvalid
	}
	now := s.deps.Clock.Now()

	if sess.RevokedAt != nil {
		// Token đã xoay/thu hồi mà xuất hiện lại = bị đánh cắp → nuke family.
		_ = s.deps.Sessions.RevokeFamily(ctx, sess.FamilyID, now)
		return TokenPair{}, domain.ErrReuseDetected
	}
	if !now.Before(sess.ExpiresAt) {
		return TokenPair{}, domain.ErrTokenInvalid
	}

	u, err := s.deps.Users.ByID(ctx, sess.UserID)
	if err != nil || u.DeletedAt != nil {
		return TokenPair{}, domain.ErrTokenInvalid
	}

	raw, newHash := s.deps.Secrets.New()
	next := &domain.Session{
		ID:               newID(),
		UserID:           sess.UserID,
		FamilyID:         sess.FamilyID,
		RefreshTokenHash: newHash,
		ExpiresAt:        now.Add(s.deps.RefreshTTL),
		CreatedAt:        now,
	}
	if err := s.deps.Sessions.Create(ctx, next); err != nil {
		return TokenPair{}, err
	}
	if err := s.deps.Sessions.MarkRotated(ctx, sess.ID, next.ID, now); err != nil {
		return TokenPair{}, err
	}
	access, exp, err := s.deps.Issuer.Issue(u.ID, string(u.Role), string(u.Plan))
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, AccessExpiresAt: exp, RefreshToken: raw}, nil
}
