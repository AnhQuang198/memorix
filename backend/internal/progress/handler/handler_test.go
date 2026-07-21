package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/progress/service"
)

// fakeTZ là TZResolver test double (thay cho IdentityPort ở prod).
type fakeTZ struct{ loc *time.Location }

func (f fakeTZ) Location(context.Context, string) *time.Location { return f.loc }

func mustLoc(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

type fakeReader struct{}

func (fakeReader) Dashboard(_ context.Context, userID string, _ time.Time, _ *time.Location) (service.DashboardView, error) {
	return service.DashboardView{DueCount: 24, NewToday: 5, StreakCurrent: 3, NorthStar: 12, TomorrowForecast: 8}, nil
}
func (fakeReader) Stats(context.Context, string, time.Time, *time.Location) (service.StatsView, error) {
	return service.StatsView{ReviewedToday: 20, Retention: 0.9}, nil
}

func setup() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// stub authmw: set principal đúng API Sprint 1 (không dùng key thô).
	r.Use(func(c *gin.Context) { authmw.SetPrincipal(c, authmw.Principal{UserID: "u1"}) })
	h := New(fakeReader{}, fakeTZ{loc: mustLoc("Asia/Ho_Chi_Minh")})
	h.Register(r.Group("/api/v1"))
	return r
}

func TestHandler_Dashboard(t *testing.T) {
	r := setup()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/progress/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	var body struct {
		Data service.DashboardView `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body: %v", err)
	}
	if body.Data.DueCount != 24 || body.Data.NorthStar != 12 {
		t.Errorf("data = %+v", body.Data)
	}
}

func TestHandler_Stats(t *testing.T) {
	r := setup()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/progress/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
}
