package service

import (
	"context"
	"errors"
	"testing"

	"github.com/memorix/memorix/internal/identity/domain"
)

func TestVerifyEmail_SetsVerifiedAndConsumesToken(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{
		Email: "v@example.com", Password: "Tr0ub4dour!",
	})
	if err := h.svc.VerifyEmail(context.Background(), res.VerifyToken); err != nil {
		t.Fatalf("verify: %v", err)
	}
	u, _ := h.stores.Users.ByEmail(context.Background(), "v@example.com")
	if !u.IsVerified() {
		t.Error("email_verified_at must be set")
	}
	// token 1-lần: dùng lại phải fail
	if err := h.svc.VerifyEmail(context.Background(), res.VerifyToken); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("reused token should be invalid, got %v", err)
	}
}

func TestVerifyEmail_BadTokenRejected(t *testing.T) {
	h := newHarness()
	if err := h.svc.VerifyEmail(context.Background(), "does-not-exist"); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestVerifyEmail_ExpiredTokenRejected(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{
		Email: "exp@example.com", Password: "Tr0ub4dour!",
	})
	h.clock.t = h.clock.t.Add(25 * 60 * 60 * 1e9) // +25h > VerifyTTL 24h
	if err := h.svc.VerifyEmail(context.Background(), res.VerifyToken); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("expired verify token should be invalid, got %v", err)
	}
}
