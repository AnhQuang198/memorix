package authmw

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := NewJWTManager([]byte("s3cret"), 15*time.Minute, "memorix")
	tok, _, _ := m.Issue("user-42", "user", "free")

	r := gin.New()
	r.GET("/me", RequireAuth(m), func(c *gin.Context) {
		p, ok := PrincipalFrom(c)
		if !ok {
			c.String(500, "no principal")
			return
		}
		c.String(200, p.UserID)
	})

	cases := []struct {
		name   string
		header string
		want   int
		body   string
	}{
		{"valid bearer", "Bearer " + tok, 200, "user-42"},
		{"missing header", "", 401, ""},
		{"malformed", "Basic xyz", 401, ""},
		{"garbage token", "Bearer not.a.jwt", 401, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
			if tc.body != "" && w.Body.String() != tc.body {
				t.Errorf("body = %q, want %q", w.Body.String(), tc.body)
			}
			if tc.want == 401 && w.Body.String() != "" {
				// envelope AD-14: {"error":{"code":"UNAUTHENTICATED",...}}
				if !containsCode(w.Body.String(), "UNAUTHENTICATED") {
					t.Errorf("401 body missing UNAUTHENTICATED envelope: %s", w.Body.String())
				}
			}
		})
	}
}

func containsCode(body, code string) bool {
	return len(body) > 0 && (indexOf(body, code) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
