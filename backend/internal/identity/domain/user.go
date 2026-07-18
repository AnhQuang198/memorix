package domain

import (
	"strings"
	"time"
)

type Plan string

const (
	PlanFree Plan = "free"
	PlanPro  Plan = "pro"
)

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type TokenKind string

const (
	KindVerify TokenKind = "verify"
	KindReset  TokenKind = "reset"
)

// User là aggregate tài khoản. password_hash rỗng = tài khoản chỉ-OAuth.
type User struct {
	ID              string
	Email           string
	PasswordHash    string
	DisplayName     string
	Timezone        string
	Locale          string
	Theme           string
	EmailVerifiedAt *time.Time
	Plan            Plan
	Role            Role
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

func (u *User) IsVerified() bool { return u.EmailVerifiedAt != nil }

// Session là refresh session; rotation qua family_id (AD-11).
type Session struct {
	ID               string
	UserID           string
	FamilyID         string
	RefreshTokenHash string
	RotatedTo        *string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
}

func (s *Session) Active(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.ExpiresAt)
}

// EmailToken dùng cho verify (TTL 24h) và reset (TTL 1h); lưu hash, 1-lần.
type EmailToken struct {
	ID        string
	UserID    string
	Kind      TokenKind
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (t *EmailToken) Usable(now time.Time) bool {
	return t.UsedAt == nil && now.Before(t.ExpiresAt)
}

type OAuthIdentity struct {
	ID          string
	UserID      string
	Provider    string
	ProviderUID string
	CreatedAt   time.Time
}

// NormalizeEmail chuẩn hóa cho so khớp logic (DB dùng citext, đây cho service).
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidTimezone kiểm tra IANA name (AD-12 — "ngày học" theo TZ user).
func ValidTimezone(tz string) bool {
	_, err := time.LoadLocation(tz)
	return err == nil
}
