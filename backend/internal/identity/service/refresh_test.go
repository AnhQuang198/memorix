package service

import (
	"context"
	"errors"
	"testing"

	"github.com/memorix/memorix/internal/identity/domain"
)

func TestRefresh_RotatesAndInvalidatesOld(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "r@example.com", "Tr0ub4dour!")
	first, _ := h.svc.Login(context.Background(), "r@example.com", "Tr0ub4dour!")

	second, err := h.svc.Refresh(context.Background(), first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Error("refresh must rotate to a NEW opaque token")
	}
	if second.AccessToken == "" {
		t.Error("refresh must mint a new access token")
	}
	// token cũ đã bị xoay → dùng lại phải bị coi là reuse
	if _, err := h.svc.Refresh(context.Background(), first.RefreshToken); !errors.Is(err, domain.ErrReuseDetected) {
		t.Errorf("old token reuse must be detected, got %v", err)
	}
}

func TestRefresh_ReuseRevokesWholeFamily(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "r@example.com", "Tr0ub4dour!")
	first, _ := h.svc.Login(context.Background(), "r@example.com", "Tr0ub4dour!")
	second, _ := h.svc.Refresh(context.Background(), first.RefreshToken)

	// tấn công: dùng lại token đã xoay (first) → revoke cả family
	if _, err := h.svc.Refresh(context.Background(), first.RefreshToken); !errors.Is(err, domain.ErrReuseDetected) {
		t.Fatalf("expected reuse detected, got %v", err)
	}
	// hệ quả: token hợp lệ kế tiếp (second) cũng bị vô hiệu do revoke family
	if _, err := h.svc.Refresh(context.Background(), second.RefreshToken); err == nil {
		t.Error("successor token must be revoked after family compromise")
	}
}

func TestRefresh_UnknownTokenInvalid(t *testing.T) {
	h := newHarness()
	if _, err := h.svc.Refresh(context.Background(), "nope"); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestRefresh_ExpiredTokenInvalid(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "r@example.com", "Tr0ub4dour!")
	first, _ := h.svc.Login(context.Background(), "r@example.com", "Tr0ub4dour!")
	h.clock.t = h.clock.t.Add(31 * 24 * 60 * 60 * 1e9) // +31d > RefreshTTL 30d
	if _, err := h.svc.Refresh(context.Background(), first.RefreshToken); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("expired refresh must be invalid, got %v", err)
	}
}
