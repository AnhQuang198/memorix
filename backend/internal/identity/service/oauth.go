package service

import (
	"context"
	"errors"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/platform/eventbus"
)

// OAuthLogin xử lý Authorization Code + PKCE sau khi verifier đã xác minh
// id_token (sig/aud/iss/nonce) — Story 1.5, AD-11.
//   - provider_uid đã link → đăng nhập user đó.
//   - chưa link + email trùng user hiện có → CHỈ link khi cả provider-email
//     verified LẪN account đã verified; ngược lại từ chối (không auto-merge).
//   - chưa link + email mới → tạo user mới (+ link), phát UserRegistered.
func (s *Service) OAuthLogin(ctx context.Context, provider, code, codeVerifier, redirectURI, nonce string) (TokenPair, error) {
	claims, err := s.deps.OIDC.Verify(ctx, provider, code, codeVerifier, redirectURI, nonce)
	if err != nil {
		return TokenPair{}, domain.ErrOAuthFailed
	}
	now := s.deps.Clock.Now()

	if oid, err := s.deps.OAuth.ByProviderUID(ctx, provider, claims.ProviderUID); err == nil {
		u, err := s.deps.Users.ByID(ctx, oid.UserID)
		if err != nil {
			return TokenPair{}, err
		}
		return s.issueSession(ctx, u)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return TokenPair{}, err
	}

	email := domain.NormalizeEmail(claims.Email)
	existing, err := s.deps.Users.ByEmail(ctx, email)
	switch {
	case err == nil:
		if !claims.EmailVerified || !existing.IsVerified() {
			return TokenPair{}, domain.ErrOAuthNoMerge
		}
		if err := s.deps.OAuth.Create(ctx, &domain.OAuthIdentity{
			ID: newID(), UserID: existing.ID, Provider: provider,
			ProviderUID: claims.ProviderUID, CreatedAt: now,
		}); err != nil {
			return TokenPair{}, err
		}
		return s.issueSession(ctx, existing)

	case errors.Is(err, domain.ErrNotFound):
		u := &domain.User{
			ID: newID(), Email: email, Timezone: "UTC", Locale: "vi", Theme: "system",
			Plan: domain.PlanFree, Role: domain.RoleUser, CreatedAt: now, UpdatedAt: now,
		}
		if claims.EmailVerified {
			u.EmailVerifiedAt = &now
		}
		if err := s.deps.Users.Create(ctx, u); err != nil {
			return TokenPair{}, err
		}
		if err := s.deps.OAuth.Create(ctx, &domain.OAuthIdentity{
			ID: newID(), UserID: u.ID, Provider: provider,
			ProviderUID: claims.ProviderUID, CreatedAt: now,
		}); err != nil {
			return TokenPair{}, err
		}
		s.deps.Bus.Publish(ctx, eventbus.Event{Name: "UserRegistered", Payload: u.ID})
		return s.issueSession(ctx, u)

	default:
		return TokenPair{}, err
	}
}
