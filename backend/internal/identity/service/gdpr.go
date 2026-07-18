package service

import (
	"context"
	"time"

	"github.com/memorix/memorix/internal/identity/domain"
)

// ExportUser là bản chụp dữ liệu identity của user cho GDPR export (JSON).
type ExportUser struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	DisplayName     string     `json:"display_name"`
	Timezone        string     `json:"timezone"`
	Locale          string     `json:"locale"`
	Theme           string     `json:"theme"`
	Plan            string     `json:"plan"`
	Role            string     `json:"role"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Export struct {
	User       ExportUser `json:"user"`
	ExportedAt time.Time  `json:"exported_at"`
}

// ExportData trả toàn bộ dữ liệu identity sau khi re-auth bằng mật khẩu (Story
// 1.8, NFR-14). Dữ liệu module khác (vocabulary/review...) ghép qua port của
// chúng ở V1 — MVP export identity scope.
func (s *Service) ExportData(ctx context.Context, userID, password string) (Export, error) {
	u, err := s.deps.Users.ByID(ctx, userID)
	if err != nil {
		return Export{}, err
	}
	ok, err := s.deps.Hasher.Verify(password, u.PasswordHash)
	if err != nil {
		return Export{}, err
	}
	if !ok {
		return Export{}, domain.ErrInvalidCredentials
	}
	return Export{
		User: ExportUser{
			ID: u.ID, Email: u.Email, DisplayName: u.DisplayName,
			Timezone: u.Timezone, Locale: u.Locale, Theme: u.Theme,
			Plan: string(u.Plan), Role: string(u.Role),
			EmailVerifiedAt: u.EmailVerifiedAt, CreatedAt: u.CreatedAt,
		},
		ExportedAt: s.deps.Clock.Now(),
	}, nil
}

// DeleteAccount soft-delete tài khoản + thu hồi mọi session ngay (Story 1.8).
// Purge cứng theo lịch do worker (Task 20). Không log PII/token.
func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	now := s.deps.Clock.Now()
	if err := s.deps.Users.SoftDelete(ctx, userID, now); err != nil {
		return err
	}
	return s.deps.Sessions.RevokeAllForUser(ctx, userID, now)
}
