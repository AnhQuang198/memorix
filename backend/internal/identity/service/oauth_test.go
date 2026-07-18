package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/identity/ports"
	"github.com/memorix/memorix/internal/identity/repo/memory"
	"github.com/memorix/memorix/internal/platform/eventbus"
)

// harness với OIDC verifier tùy biến.
func newOIDCHarness(claims ports.OIDCClaims, verr error) *harness {
	clk := &fakeClock{t: time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)}
	lim := &fakeLimiter{allow: true}
	bus := eventbus.NewInProcess()
	st := memory.New()
	svc := New(Deps{
		Users: st.Users, Sessions: st.Sessions, Tokens: st.Tokens, OAuth: st.OAuth,
		Hasher: fakeHasher{}, Issuer: fakeIssuer{now: clk.Now}, Secrets: &fakeSecrets{},
		Clock: clk, Limiter: lim, OIDC: stubOIDC{claims: claims, err: verr}, Bus: bus,
		RefreshTTL: 30 * 24 * time.Hour, VerifyTTL: 24 * time.Hour, ResetTTL: time.Hour,
	})
	return &harness{svc: svc, stores: st, clock: clk, limiter: lim, bus: bus}
}

func TestOAuth_FirstTimeCreatesUserAndLinks(t *testing.T) {
	h := newOIDCHarness(ports.OIDCClaims{ProviderUID: "g-123", Email: "new@example.com", EmailVerified: true}, nil)
	tok, err := h.svc.OAuthLogin(context.Background(), "google", "code", "verifier", "https://app/cb", "nonce")
	if err != nil {
		t.Fatalf("oauth: %v", err)
	}
	if tok.AccessToken == "" {
		t.Error("expected tokens on first oauth login")
	}
	oid, err := h.stores.OAuth.ByProviderUID(context.Background(), "google", "g-123")
	if err != nil {
		t.Fatalf("identity not linked: %v", err)
	}
	u, _ := h.stores.Users.ByID(context.Background(), oid.UserID)
	if !u.IsVerified() {
		t.Error("provider-verified email should mark account verified")
	}
}

func TestOAuth_ExistingLinkReused(t *testing.T) {
	h := newOIDCHarness(ports.OIDCClaims{ProviderUID: "g-123", Email: "x@example.com", EmailVerified: true}, nil)
	first, _ := h.svc.OAuthLogin(context.Background(), "google", "c", "v", "cb", "n")
	_ = first
	second, err := h.svc.OAuthLogin(context.Background(), "google", "c2", "v2", "cb", "n2")
	if err != nil {
		t.Fatalf("second oauth: %v", err)
	}
	if second.AccessToken == "" {
		t.Error("relogin via linked identity should succeed")
	}
	// vẫn chỉ 1 user
	if _, err := h.stores.Users.ByEmail(context.Background(), "x@example.com"); err != nil {
		t.Errorf("user should persist: %v", err)
	}
}

func TestOAuth_NoMergeOnUnverifiedEmail(t *testing.T) {
	// user email/password đã tồn tại nhưng chưa verify; provider trả cùng email
	h := newOIDCHarness(ports.OIDCClaims{ProviderUID: "g-999", Email: "dup@example.com", EmailVerified: true}, nil)
	seedUser(t, h, "dup@example.com", "Tr0ub4dour!") // account CHƯA verify
	_, err := h.svc.OAuthLogin(context.Background(), "google", "c", "v", "cb", "n")
	if !errors.Is(err, domain.ErrOAuthNoMerge) {
		t.Errorf("must not auto-merge into unverified account, got %v", err)
	}
}

func TestOAuth_VerifierFailure(t *testing.T) {
	h := newOIDCHarness(ports.OIDCClaims{}, errors.New("bad signature"))
	if _, err := h.svc.OAuthLogin(context.Background(), "google", "c", "v", "cb", "n"); !errors.Is(err, domain.ErrOAuthFailed) {
		t.Errorf("expected ErrOAuthFailed, got %v", err)
	}
}
