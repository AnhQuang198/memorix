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
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/service"
)

type fakeQueue struct{ res domain.QueueResult }

func (f fakeQueue) BuildToday(_ context.Context, _ uuid.UUID, _ time.Time) (domain.QueueResult, error) {
	return f.res, nil
}

type fakeLearn struct{ acked bool }

func (f *fakeLearn) StartSession(_ context.Context, _ uuid.UUID, _ time.Time) (service.LearnSession, error) {
	return service.LearnSession{Cards: []domain.Card{{ID: uuid.New(), Status: domain.StatusNew}}, ShowCoach: true}, nil
}
func (f *fakeLearn) AckCoach(_ context.Context, _ uuid.UUID) error { f.acked = true; return nil }

type fakePrefsUpd struct{ p domain.SchedulerPrefs }

func (f fakePrefsUpd) UpdateLimits(_ context.Context, _ uuid.UUID, n, r int) (domain.SchedulerPrefs, error) {
	f.p.DailyNewLimit, f.p.DailyReviewLimit = n, r
	return f.p, nil
}

func setup(q QueueBuilder, l LearnProvider, p PrefsUpdater) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { authmw.SetPrincipal(c, authmw.Principal{UserID: uuid.NewString()}); c.Next() }) // fake authmw (Auth Contract)
	RegisterQueueRoutes(r.Group("/api/v1"), q, l, p)
	return r
}

func TestQueueEndpoint(t *testing.T) {
	now := time.Now()
	res := domain.QueueResult{Cards: []domain.Card{{ID: uuid.New(), Status: domain.StatusReview, DueAt: &now}}, NewCount: 0, ReviewCount: 1}
	r := setup(fakeQueue{res: res}, &fakeLearn{}, fakePrefsUpd{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/queue", nil))
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["review_count"].(float64) != 1 {
		t.Errorf("review_count = %v, want 1", body["review_count"])
	}
}

func TestLearnAndCoachAck(t *testing.T) {
	fl := &fakeLearn{}
	r := setup(fakeQueue{}, fl, fakePrefsUpd{})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/learn", nil))
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["show_coach"] != true {
		t.Errorf("show_coach = %v, want true", body["show_coach"])
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/api/v1/learn/coach/ack", nil))
	if w2.Code != 204 || !fl.acked {
		t.Errorf("ack status=%d acked=%v", w2.Code, fl.acked)
	}
}

func TestUpdatePrefs_Validation(t *testing.T) {
	r := setup(fakeQueue{}, &fakeLearn{}, fakePrefsUpd{})
	// hợp lệ
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/api/v1/scheduler/prefs",
		strings.NewReader(`{"daily_new_limit":30,"daily_review_limit":150}`)))
	if w.Code != 200 {
		t.Fatalf("valid update status = %d", w.Code)
	}
	// ngoài khoảng 1..9999
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPatch, "/api/v1/scheduler/prefs",
		strings.NewReader(`{"daily_new_limit":0,"daily_review_limit":150}`)))
	if w2.Code != 400 {
		t.Errorf("invalid limit status = %d, want 400", w2.Code)
	}
}
