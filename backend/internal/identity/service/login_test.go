package service

import (
	"context"
	"errors"
	"testing"

	"github.com/memorix/memorix/internal/identity/domain"
)

func seedUser(t *testing.T, h *harness, email, pw string) {
	t.Helper()
	if _, err := h.svc.Register(context.Background(), RegisterInput{Email: email, Password: pw}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "l@example.com", "Tr0ub4dour!")
	tok, err := h.svc.Login(context.Background(), "L@Example.com", "Tr0ub4dour!")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if tok.AccessToken == "" || tok.RefreshToken == "" {
		t.Error("expected token pair on success")
	}
	if h.limiter.resets == 0 {
		t.Error("limiter must be reset on successful login")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "l@example.com", "Tr0ub4dour!")
	_, err := h.svc.Login(context.Background(), "l@example.com", "wrong-pass99")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_UnknownEmailSameError(t *testing.T) {
	h := newHarness()
	// không tạo user — phải trả CÙNG lỗi ErrInvalidCredentials (chống enumeration)
	_, err := h.svc.Login(context.Background(), "ghost@example.com", "whatever99A")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("unknown email must return ErrInvalidCredentials (no enumeration), got %v", err)
	}
}

func TestLogin_RateLimited(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "l@example.com", "Tr0ub4dour!")
	h.limiter.allow = false // limiter đã chặn
	_, err := h.svc.Login(context.Background(), "l@example.com", "Tr0ub4dour!")
	if !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}
