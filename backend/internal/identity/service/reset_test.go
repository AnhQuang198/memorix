package service

import (
	"context"
	"errors"
	"testing"

	"github.com/memorix/memorix/internal/identity/domain"
)

func TestRequestReset_KnownEmailIssuesToken(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "p@example.com", "Tr0ub4dour!")
	raw, err := h.svc.RequestReset(context.Background(), "P@Example.com")
	if err != nil {
		t.Fatalf("request reset: %v", err)
	}
	if raw == "" {
		t.Error("known email must produce a reset token to email")
	}
}

func TestRequestReset_UnknownEmailNoError(t *testing.T) {
	h := newHarness()
	raw, err := h.svc.RequestReset(context.Background(), "ghost@example.com")
	if err != nil {
		t.Fatalf("unknown email must not error (no enumeration): %v", err)
	}
	if raw != "" {
		t.Error("unknown email must not produce a token")
	}
}

func TestResetPassword_UpdatesAndRevokesSessions(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "p@example.com", "Tr0ub4dour!")
	login, _ := h.svc.Login(context.Background(), "p@example.com", "Tr0ub4dour!")
	raw, _ := h.svc.RequestReset(context.Background(), "p@example.com")

	if err := h.svc.ResetPassword(context.Background(), raw, "N3wStr0ng!Pass"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	// mật khẩu mới dùng được
	if _, err := h.svc.Login(context.Background(), "p@example.com", "N3wStr0ng!Pass"); err != nil {
		t.Errorf("login with new password failed: %v", err)
	}
	// mọi session cũ bị thu hồi
	if _, err := h.svc.Refresh(context.Background(), login.RefreshToken); err == nil {
		t.Error("existing sessions must be revoked after reset (Story 1.6)")
	}
	// token 1-lần
	if err := h.svc.ResetPassword(context.Background(), raw, "An0ther!Pass9"); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("reused reset token must be invalid, got %v", err)
	}
}

func TestResetPassword_WeakRejected(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "p@example.com", "Tr0ub4dour!")
	raw, _ := h.svc.RequestReset(context.Background(), "p@example.com")
	if err := h.svc.ResetPassword(context.Background(), raw, "weak"); !errors.Is(err, domain.ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got %v", err)
	}
}
