package authmw

import (
	"testing"
	"time"
)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestJWTManager_IssueVerifyRoundTrip(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	m := NewJWTManager([]byte("test-secret-please-change"), 15*time.Minute, "memorix")
	m.now = fixedClock(base)

	tok, exp, err := m.Issue("user-1", "user", "free")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !exp.Equal(base.Add(15 * time.Minute)) {
		t.Errorf("exp = %v, want base+15m", exp)
	}
	p, err := m.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if p.UserID != "user-1" || p.Role != "user" || p.Plan != "free" {
		t.Errorf("principal = %+v", p)
	}
}

func TestJWTManager_RejectsExpired(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	m := NewJWTManager([]byte("s3cret"), 15*time.Minute, "memorix")
	m.now = fixedClock(base)
	tok, _, _ := m.Issue("u", "user", "free")
	m.now = fixedClock(base.Add(20 * time.Minute)) // access 15m đã hết hạn
	if _, err := m.Verify(tok); err == nil {
		t.Error("expected expired token rejected")
	}
}

func TestJWTManager_RejectsWrongSecret(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	issuer := NewJWTManager([]byte("secret-A"), 15*time.Minute, "memorix")
	issuer.now = fixedClock(base)
	tok, _, _ := issuer.Issue("u", "user", "free")

	attacker := NewJWTManager([]byte("secret-B"), 15*time.Minute, "memorix")
	attacker.now = fixedClock(base)
	if _, err := attacker.Verify(tok); err == nil {
		t.Error("expected wrong-secret token rejected")
	}
}
