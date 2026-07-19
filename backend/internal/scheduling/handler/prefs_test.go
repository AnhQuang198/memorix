package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/handler"
	"github.com/memorix/memorix/internal/scheduling/service"
)

// fakePrefs = fake ports.PrefsStore; PrefsService validation thật vẫn chạy.
type fakePrefs struct{ saved domain.SchedulerPrefs }

func (f *fakePrefs) Get(_ context.Context, _ db.Querier, uid uuid.UUID) (domain.SchedulerPrefs, error) {
	p := domain.DefaultPrefs()
	p.UserID = uid
	return p, nil
}

func (f *fakePrefs) Upsert(_ context.Context, _ db.Querier, p domain.SchedulerPrefs) error {
	f.saved = p
	return nil
}

// newRouter dựng engine với principal giả lập qua authmw.SetPrincipal (Auth
// Contract Sprint 1) — KHÔNG dùng key thô "user_id".
func newRouter(owner uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		authmw.SetPrincipal(c, authmw.Principal{UserID: owner.String()})
		c.Next()
	})
	ps := service.NewPrefsService(nil, &fakePrefs{})
	handler.RegisterRoutes(r.Group("/api/v1"), handler.New(ps))
	return r
}

// newRouterNoAuth dựng engine KHÔNG có principal (test deny-by-default).
func newRouterNoAuth() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ps := service.NewPrefsService(nil, &fakePrefs{})
	handler.RegisterRoutes(r.Group("/api/v1"), handler.New(ps))
	return r
}

func errCode(t *testing.T, body []byte) string {
	t.Helper()
	var resp map[string]map[string]any
	require.NoError(t, json.Unmarshal(body, &resp))
	code, _ := resp["error"]["code"].(string)
	return code
}

func TestPutPrefs_OK(t *testing.T) {
	owner := uuid.New()
	r := newRouter(owner)

	body := `{"desired_retention":0.85,"daily_new_limit":30,"daily_review_limit":150,"timezone":"Asia/Bangkok"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/scheduler/prefs", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, 0.85, got["desired_retention"])
	require.Equal(t, float64(30), got["daily_new_limit"])
	require.Equal(t, float64(150), got["daily_review_limit"])
	require.Equal(t, "Asia/Bangkok", got["timezone"])
}

func TestPutPrefs_RejectsBadRetention(t *testing.T) {
	r := newRouter(uuid.New())

	req := httptest.NewRequest(http.MethodPut, "/api/v1/scheduler/prefs",
		strings.NewReader(`{"desired_retention":0.5,"timezone":"UTC"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "VALIDATION_ERROR", errCode(t, w.Body.Bytes()))
}

func TestPutPrefs_RejectsBadTimezone(t *testing.T) {
	r := newRouter(uuid.New())

	req := httptest.NewRequest(http.MethodPut, "/api/v1/scheduler/prefs",
		strings.NewReader(`{"desired_retention":0.9,"timezone":"Not/AZone"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "VALIDATION_ERROR", errCode(t, w.Body.Bytes()))
}

func TestPutPrefs_RejectsBadBody(t *testing.T) {
	r := newRouter(uuid.New())

	req := httptest.NewRequest(http.MethodPut, "/api/v1/scheduler/prefs",
		strings.NewReader(`{`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "VALIDATION_ERROR", errCode(t, w.Body.Bytes()))
}

func TestGetPrefs_OK(t *testing.T) {
	owner := uuid.New()
	r := newRouter(owner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/prefs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, 0.90, got["desired_retention"])
	require.Equal(t, "UTC", got["timezone"])
}

func TestPrefs_Unauthenticated(t *testing.T) {
	r := newRouterNoAuth()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/prefs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Equal(t, "UNAUTHENTICATED", errCode(t, w.Body.Bytes()))
}
