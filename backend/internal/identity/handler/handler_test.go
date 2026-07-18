package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/memorix/memorix/internal/identity/repo/memory"
	"github.com/memorix/memorix/internal/identity/service"
	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/platform/eventbus"
	"github.com/memorix/memorix/internal/platform/ratelimit"
	"github.com/memorix/memorix/internal/platform/security"
)

type testMailer struct{}

func (testMailer) SendVerification(context.Context, string, string) error  { return nil }
func (testMailer) SendPasswordReset(context.Context, string, string) error { return nil }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func newTestServer(t *testing.T) (*gin.Engine, *authmw.JWTManager) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := memory.New()
	jwt := authmw.NewJWTManager([]byte("test-secret"), 15*time.Minute, "memorix")
	svc := service.New(service.Deps{
		Users: st.Users, Sessions: st.Sessions, Tokens: st.Tokens, OAuth: st.OAuth,
		Hasher:  security.NewArgon2Hasher(),
		Issuer:  jwt,
		Secrets: security.TokenFactory{},
		Clock:   realClock{},
		Limiter: ratelimit.NewWindow(5, time.Minute),
		Bus:     eventbus.NewInProcess(),
		RefreshTTL: 30 * 24 * time.Hour, VerifyTTL: 24 * time.Hour, ResetTTL: time.Hour,
	})
	h := New(svc, testMailer{}, jwt, 30*24*time.Hour, false, nil)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	return r, jwt
}

func doJSON(r *gin.Engine, method, path, body, bearer string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandler_RegisterThenDuplicate(t *testing.T) {
	r, _ := newTestServer(t)
	body := `{"email":"h@example.com","password":"Tr0ub4dour!","display_name":"H"}`
	w := doJSON(r, http.MethodPost, "/api/v1/auth/register", body, "", nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Data.AccessToken == "" {
		t.Error("expected access_token in response")
	}
	if !hasCookie(w, refreshCookie) {
		t.Error("expected httpOnly refresh cookie set")
	}
	// duplicate → 409
	w2 := doJSON(r, http.MethodPost, "/api/v1/auth/register", body, "", nil)
	if w2.Code != http.StatusConflict {
		t.Errorf("duplicate register status = %d, want 409", w2.Code)
	}
}

func TestHandler_LoginWrongIs401Envelope(t *testing.T) {
	r, _ := newTestServer(t)
	doJSON(r, http.MethodPost, "/api/v1/auth/register",
		`{"email":"h@example.com","password":"Tr0ub4dour!"}`, "", nil)
	w := doJSON(r, http.MethodPost, "/api/v1/auth/login",
		`{"email":"h@example.com","password":"nope-nope9"}`, "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if !strings.Contains(w.Body.String(), "UNAUTHENTICATED") {
		t.Errorf("expected UNAUTHENTICATED envelope, got %s", w.Body.String())
	}
}

func TestHandler_MeRequiresAuth(t *testing.T) {
	r, jwt := newTestServer(t)
	// không token → 401
	if w := doJSON(r, http.MethodGet, "/api/v1/me", "", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("no-token /me = %d, want 401", w.Code)
	}
	// register lấy user id qua token
	reg := doJSON(r, http.MethodPost, "/api/v1/auth/register",
		`{"email":"me@example.com","password":"Tr0ub4dour!"}`, "", nil)
	var got struct {
		Data struct {
			UserID string `json:"user_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(reg.Body.Bytes(), &got)
	tok, _, _ := jwt.Issue(got.Data.UserID, "user", "free")
	w := doJSON(r, http.MethodGet, "/api/v1/me", "", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("authorized /me = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "me@example.com") {
		t.Errorf("me payload missing email: %s", w.Body.String())
	}
}

func hasCookie(w *httptest.ResponseRecorder, name string) bool {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return true
		}
	}
	return false
}
