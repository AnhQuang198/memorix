package oauthx

import (
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestBeginAuth_BuildsPKCEAuthURL(t *testing.T) {
	v := &Verifier{providers: map[string]*provider{
		"google": {oauth2: &oauth2.Config{
			ClientID:    "client-x",
			Endpoint:    oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth"},
			RedirectURL: "https://app/cb",
			Scopes:      []string{"openid", "email"},
		}},
	}}
	req, err := v.BeginAuth("google")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if req.State == "" || req.Nonce == "" || req.Verifier == "" {
		t.Fatal("state/nonce/verifier must be generated")
	}
	u, err := url.Parse(req.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	q := u.Query()
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		t.Error("auth URL must carry S256 PKCE challenge")
	}
	if q.Get("state") != req.State || q.Get("nonce") != req.Nonce {
		t.Error("auth URL must carry state and nonce")
	}
	if !strings.Contains(req.URL, "client-x") {
		t.Error("auth URL must carry client id")
	}
}

func TestBeginAuth_UnknownProvider(t *testing.T) {
	v := &Verifier{providers: map[string]*provider{}}
	if _, err := v.BeginAuth("apple"); err == nil {
		t.Error("unknown provider must error")
	}
}
