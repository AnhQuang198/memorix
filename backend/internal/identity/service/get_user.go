package service

import "context"

// UserView là hồ sơ trả cho client (không lộ password_hash).
type UserView struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Timezone    string `json:"timezone"`
	Locale      string `json:"locale"`
	Theme       string `json:"theme"`
	Plan        string `json:"plan"`
	Role        string `json:"role"`
	Verified    bool   `json:"verified"`
}

func (s *Service) GetUser(ctx context.Context, userID string) (UserView, error) {
	u, err := s.deps.Users.ByID(ctx, userID)
	if err != nil {
		return UserView{}, err
	}
	return UserView{
		ID: u.ID, Email: u.Email, DisplayName: u.DisplayName,
		Timezone: u.Timezone, Locale: u.Locale, Theme: u.Theme,
		Plan: string(u.Plan), Role: string(u.Role), Verified: u.IsVerified(),
	}, nil
}
