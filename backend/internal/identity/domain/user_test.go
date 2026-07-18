package domain

import (
	"testing"
	"time"
)

func TestUser_IsVerified(t *testing.T) {
	u := User{}
	if u.IsVerified() {
		t.Error("new user must be unverified")
	}
	now := time.Now()
	u.EmailVerifiedAt = &now
	if !u.IsVerified() {
		t.Error("user with EmailVerifiedAt should be verified")
	}
}

func TestSession_Active(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	s := Session{ExpiresAt: now.Add(time.Hour)}
	if !s.Active(now) {
		t.Error("session should be active before expiry")
	}
	if s.Active(now.Add(2 * time.Hour)) {
		t.Error("session should be inactive after expiry")
	}
	revoked := now
	s.RevokedAt = &revoked
	if s.Active(now) {
		t.Error("revoked session must be inactive")
	}
}

func TestEmailToken_Usable(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	tok := EmailToken{ExpiresAt: now.Add(time.Hour)}
	if !tok.Usable(now) {
		t.Error("fresh token should be usable")
	}
	if tok.Usable(now.Add(2 * time.Hour)) {
		t.Error("expired token unusable")
	}
	used := now
	tok.UsedAt = &used
	if tok.Usable(now) {
		t.Error("used token unusable")
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  Linh@Example.COM "); got != "linh@example.com" {
		t.Errorf("NormalizeEmail = %q", got)
	}
}

func TestValidTimezone(t *testing.T) {
	if !ValidTimezone("Asia/Ho_Chi_Minh") {
		t.Error("valid IANA tz rejected")
	}
	if ValidTimezone("Mars/Phobos") {
		t.Error("bogus tz accepted")
	}
}
