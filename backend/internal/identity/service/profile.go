package service

import (
	"context"

	"github.com/memorix/memorix/internal/identity/domain"
)

// ProfileInput: field nil = giữ nguyên (partial update).
type ProfileInput struct {
	DisplayName *string
	Timezone    *string
	Locale      *string
	Theme       *string
}

// UpdateProfile cập nhật tên/múi giờ/ngôn ngữ/theme (Story 1.7). Timezone dùng
// cho "ngày học" downstream (AD-12); validate whitelist deny-by-default.
func (s *Service) UpdateProfile(ctx context.Context, userID string, in ProfileInput) (*domain.User, error) {
	u, err := s.deps.Users.ByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if in.DisplayName != nil {
		u.DisplayName = *in.DisplayName
	}
	if in.Timezone != nil {
		if !domain.ValidTimezone(*in.Timezone) {
			return nil, domain.ErrInvalidProfile
		}
		u.Timezone = *in.Timezone
	}
	if in.Locale != nil {
		if *in.Locale != "vi" && *in.Locale != "en" {
			return nil, domain.ErrInvalidProfile
		}
		u.Locale = *in.Locale
	}
	if in.Theme != nil {
		switch *in.Theme {
		case "light", "dark", "system":
			u.Theme = *in.Theme
		default:
			return nil, domain.ErrInvalidProfile
		}
	}
	u.UpdatedAt = s.deps.Clock.Now()
	if err := s.deps.Users.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}
