package service

import (
	"time"

	"github.com/memorix/memorix/internal/identity/repo/memory"
	"github.com/memorix/memorix/internal/platform/eventbus"
)

type harness struct {
	svc     *Service
	stores  *memory.Stores
	clock   *fakeClock
	limiter *fakeLimiter
	bus     *eventbus.InProcess
}

func newHarness() *harness {
	clk := &fakeClock{t: time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)}
	lim := &fakeLimiter{allow: true}
	bus := eventbus.NewInProcess()
	st := memory.New()
	svc := New(Deps{
		Users:      st.Users,
		Sessions:   st.Sessions,
		Tokens:     st.Tokens,
		OAuth:      st.OAuth,
		Hasher:     fakeHasher{},
		Issuer:     fakeIssuer{now: clk.Now},
		Secrets:    &fakeSecrets{},
		Clock:      clk,
		Limiter:    lim,
		OIDC:       stubOIDC{},
		Bus:        bus,
		RefreshTTL: 30 * 24 * time.Hour,
		VerifyTTL:  24 * time.Hour,
		ResetTTL:   time.Hour,
	})
	return &harness{svc: svc, stores: st, clock: clk, limiter: lim, bus: bus}
}
