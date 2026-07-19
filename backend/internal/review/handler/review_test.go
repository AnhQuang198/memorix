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
	revdom "github.com/memorix/memorix/internal/review/domain"
	"github.com/memorix/memorix/internal/review/handler"
	"github.com/memorix/memorix/internal/review/service"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
)

// fakeGrader = fake GraderPort; đếm số lần gọi để chứng minh passthrough.
type fakeGrader struct{ calls int }

func (f *fakeGrader) Grade(_ context.Context, _ uuid.UUID, cmd revdom.GradeCommand) (revdom.GradeResult, error) {
	f.calls++
	return revdom.GradeResult{CardID: cmd.CardID, Stability: 5, Difficulty: 5, Status: scheddom.StatusReview, Reps: 1}, nil
}

type fakeQueuer struct{}

func (fakeQueuer) Queue(context.Context, uuid.UUID, int) ([]service.QueueItem, error) {
	return []service.QueueItem{{CardID: uuid.New(), Term: "ephemeral",
		NextIntervals: service.QueueIntervals{AgainSeconds: 600, EasySeconds: 777600}}}, nil
}

type fakeSummary struct{}

func (fakeSummary) Summary(context.Context, uuid.UUID) (service.SessionSummary, error) {
	return service.SessionSummary{Reviewed: 3, Remembered: 2, ForecastTomorrow: 7}, nil
}

// router dựng engine với principal giả lập qua authmw.SetPrincipal (Auth Contract
// Sprint 1) — KHÔNG dùng key thô "user_id". Mirror scheduling/handler test.
func router(owner uuid.UUID, g handler.GraderPort) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		authmw.SetPrincipal(c, authmw.Principal{UserID: owner.String()})
		c.Next()
	})
	h := handler.NewReviewHandler(g, fakeQueuer{}, fakeSummary{})
	h.Register(r.Group("/api/v1"))
	return r
}

// routerNoAuth dựng engine KHÔNG principal (test deny-by-default, AD-14).
func routerNoAuth() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewReviewHandler(&fakeGrader{}, fakeQueuer{}, fakeSummary{})
	h.Register(r.Group("/api/v1"))
	return r
}

func TestGradeEndpoint_AcceptsOnlyCardGradeClientID(t *testing.T) {
	owner := uuid.New()
	fg := &fakeGrader{}
	r := router(owner, fg)

	cid := uuid.New()
	body := `{"card_id":"` + cid.String() + `","grade":3,"client_review_id":"cr-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/review/grade", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, cid.String(), got["card_id"])
	require.Equal(t, 5.0, got["stability"])
	require.Equal(t, 1, fg.calls)
}

func TestGradeEndpoint_RejectsBadGrade(t *testing.T) {
	fg := &fakeGrader{}
	r := router(uuid.New(), fg)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/review/grade",
		strings.NewReader(`{"card_id":"`+uuid.New().String()+`","grade":9,"client_review_id":"x"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "VALIDATION_ERROR", errCode(t, w.Body.Bytes()))
	require.Equal(t, 0, fg.calls) // handler validate range TRƯỚC khi gọi service
}

func TestGradeEndpoint_RejectsBadCardID(t *testing.T) {
	r := router(uuid.New(), &fakeGrader{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/review/grade",
		strings.NewReader(`{"card_id":"not-a-uuid","grade":3,"client_review_id":"x"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "VALIDATION_ERROR", errCode(t, w.Body.Bytes()))
}

func TestGradeEndpoint_RejectsMissingClientReviewID(t *testing.T) {
	r := router(uuid.New(), &fakeGrader{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/review/grade",
		strings.NewReader(`{"card_id":"`+uuid.New().String()+`","grade":3}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "VALIDATION_ERROR", errCode(t, w.Body.Bytes()))
}

func TestQueueEndpoint(t *testing.T) {
	r := router(uuid.New(), &fakeGrader{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/review/queue", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var got map[string][]map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got["data"], 1)
	require.Equal(t, "ephemeral", got["data"][0]["term"])
}

func TestSummaryEndpoint(t *testing.T) {
	r := router(uuid.New(), &fakeGrader{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/review/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, 2.0, got["remembered"])
}

func TestReview_Unauthenticated(t *testing.T) {
	r := routerNoAuth()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/review/queue", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Equal(t, "UNAUTHENTICATED", errCode(t, w.Body.Bytes()))
}

// errCode trích error.code từ envelope chuẩn (AD-14).
func errCode(t *testing.T, body []byte) string {
	t.Helper()
	var resp map[string]map[string]any
	require.NoError(t, json.Unmarshal(body, &resp))
	code, _ := resp["error"]["code"].(string)
	return code
}
