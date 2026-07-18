package oauthx

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/memorix/memorix/internal/identity/ports"
	"golang.org/x/oauth2"
)

type provider struct {
	oauth2   *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// Verifier implements ports.OIDCVerifier cho nhiều provider (google, apple).
type Verifier struct {
	providers map[string]*provider
}

// ProviderConfig cấu hình 1 provider OIDC.
type ProviderConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// New khám phá OIDC discovery cho từng provider (cần mạng — gọi lúc bootstrap).
func New(ctx context.Context, cfgs map[string]ProviderConfig) (*Verifier, error) {
	v := &Verifier{providers: map[string]*provider{}}
	for name, c := range cfgs {
		op, err := oidc.NewProvider(ctx, c.IssuerURL)
		if err != nil {
			return nil, fmt.Errorf("oidc discovery %s: %w", name, err)
		}
		scopes := c.Scopes
		if len(scopes) == 0 {
			scopes = []string{oidc.ScopeOpenID, "email", "profile"}
		}
		v.providers[name] = &provider{
			oauth2: &oauth2.Config{
				ClientID:     c.ClientID,
				ClientSecret: c.ClientSecret,
				RedirectURL:  c.RedirectURL,
				Endpoint:     op.Endpoint(),
				Scopes:       scopes,
			},
			verifier: op.Verifier(&oidc.Config{ClientID: c.ClientID}),
		}
	}
	return v, nil
}

// AuthRequest là dữ liệu 1 lần server lưu (state/nonce/verifier) để verify callback.
type AuthRequest struct {
	URL      string
	State    string
	Nonce    string
	Verifier string
}

// BeginAuth sinh PKCE verifier + state + nonce và AuthURL (Authorization Code + PKCE).
func (v *Verifier) BeginAuth(provider string) (AuthRequest, error) {
	p, ok := v.providers[provider]
	if !ok {
		return AuthRequest{}, fmt.Errorf("oauthx: unknown provider %q", provider)
	}
	codeVerifier := oauth2.GenerateVerifier()
	state := randString()
	nonce := randString()
	url := p.oauth2.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(codeVerifier),
		oidc.Nonce(nonce),
	)
	return AuthRequest{URL: url, State: state, Nonce: nonce, Verifier: codeVerifier}, nil
}

// Verify đổi code (PKCE) lấy token, xác minh id_token (sig/aud/iss) + nonce (AD-11).
func (v *Verifier) Verify(ctx context.Context, provider, code, codeVerifier, redirectURI, nonce string) (ports.OIDCClaims, error) {
	p, ok := v.providers[provider]
	if !ok {
		return ports.OIDCClaims{}, fmt.Errorf("oauthx: unknown provider %q", provider)
	}
	oauthTok, err := p.oauth2.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return ports.OIDCClaims{}, fmt.Errorf("oauthx: code exchange: %w", err)
	}
	rawID, ok := oauthTok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return ports.OIDCClaims{}, fmt.Errorf("oauthx: missing id_token")
	}
	idTok, err := p.verifier.Verify(ctx, rawID)
	if err != nil {
		return ports.OIDCClaims{}, fmt.Errorf("oauthx: id_token verify: %w", err)
	}
	if idTok.Nonce != nonce {
		return ports.OIDCClaims{}, fmt.Errorf("oauthx: nonce mismatch")
	}
	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idTok.Claims(&claims); err != nil {
		return ports.OIDCClaims{}, fmt.Errorf("oauthx: claims: %w", err)
	}
	return ports.OIDCClaims{
		ProviderUID:   claims.Sub,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
	}, nil
}

func randString() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic("oauthx: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

var _ ports.OIDCVerifier = (*Verifier)(nil)
