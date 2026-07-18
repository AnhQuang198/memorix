package service

import (
	"context"
	"errors"
	"testing"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/platform/eventbus"
)

func TestRegister_CreatesHashedUserAndTokens(t *testing.T) {
	h := newHarness()
	res, err := h.svc.Register(context.Background(), RegisterInput{
		Email: "Linh@Example.com", Password: "Tr0ub4dour!", DisplayName: "Linh",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if res.Tokens.AccessToken == "" || res.Tokens.RefreshToken == "" {
		t.Error("expected access + refresh tokens")
	}
	if res.VerifyToken == "" {
		t.Error("expected verify email token issued (Story 1.3)")
	}
	u, err := h.stores.Users.ByEmail(context.Background(), "linh@example.com")
	if err != nil {
		t.Fatalf("user not persisted: %v", err)
	}
	if u.PasswordHash == "Tr0ub4dour!" || u.PasswordHash == "" {
		t.Error("password must be hashed, never raw")
	}
	if u.IsVerified() {
		t.Error("new account must be unverified until email confirmed (FR-2)")
	}
}

func TestRegister_DuplicateEmailConflict(t *testing.T) {
	h := newHarness()
	in := RegisterInput{Email: "dup@example.com", Password: "Tr0ub4dour!"}
	if _, err := h.svc.Register(context.Background(), in); err != nil {
		t.Fatalf("first register: %v", err)
	}
	_, err := h.svc.Register(context.Background(), in)
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestRegister_WeakPasswordRejected(t *testing.T) {
	h := newHarness()
	_, err := h.svc.Register(context.Background(), RegisterInput{
		Email: "weak@example.com", Password: "password",
	})
	if !errors.Is(err, domain.ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got %v", err)
	}
}

func TestRegister_EmitsUserRegistered(t *testing.T) {
	h := newHarness()
	got := make(chan string, 1)
	h.bus.Subscribe("UserRegistered", func(_ context.Context, e eventbus.Event) {
		id, _ := e.Payload.(string)
		got <- id
	})
	res, _ := h.svc.Register(context.Background(), RegisterInput{
		Email: "e@example.com", Password: "Tr0ub4dour!",
	})
	h.bus.Wait()
	select {
	case id := <-got:
		if id != res.UserID {
			t.Errorf("event payload = %q, want %q", id, res.UserID)
		}
	default:
		t.Error("UserRegistered not published")
	}
}
