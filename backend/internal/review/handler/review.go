// Package handler là adapter Gin của review: POST /review/grade, GET /review/queue,
// GET /review/summary (AD-2, AD-14). Principal đọc qua authmw.UserID theo Auth
// Contract Sprint 1; guard authmw.RequireAuth wire ở cmd/api (Task 16), KHÔNG ở đây.
package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/platform/httpx"
	revdom "github.com/memorix/memorix/internal/review/domain"
	"github.com/memorix/memorix/internal/review/service"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
)

// Cổng service (interface để test inject fake, đảo phụ thuộc — S6).
type GraderPort interface {
	Grade(ctx context.Context, ownerID uuid.UUID, cmd revdom.GradeCommand) (revdom.GradeResult, error)
}
type QueuePort interface {
	Queue(ctx context.Context, ownerID uuid.UUID, limit int) ([]service.QueueItem, error)
}
type SummaryPort interface {
	Summary(ctx context.Context, ownerID uuid.UUID) (service.SessionSummary, error)
}

// ReviewHandler phục vụ 3 route review trên GradeService/QueueService/SummaryService.
type ReviewHandler struct {
	grader  GraderPort
	queuer  QueuePort
	summary SummaryPort
}

func NewReviewHandler(g GraderPort, q QueuePort, s SummaryPort) *ReviewHandler {
	return &ReviewHandler{grader: g, queuer: q, summary: s}
}

// Register gắn route review vào group /api/v1.
func (h *ReviewHandler) Register(g *gin.RouterGroup) {
	g.POST("/review/grade", h.grade)
	g.GET("/review/queue", h.queue)
	g.GET("/review/summary", h.summaryHandler)
}

// gradeReq = payload duy nhất client gửi (AD-5). KHÔNG có S/D/Due (server-authoritative).
// DurationMs tùy chọn (telemetry client); server không dùng để tính lịch.
type gradeReq struct {
	CardID         string `json:"card_id"`
	Grade          int16  `json:"grade"`
	ClientReviewID string `json:"client_review_id"`
	DurationMs     int    `json:"duration_ms"`
}

// gradeResp = trạng thái card sau chấm trả về client.
type gradeResp struct {
	CardID     string  `json:"card_id"`
	Stability  float64 `json:"stability"`
	Difficulty float64 `json:"difficulty"`
	Status     string  `json:"status"`
	Reps       int     `json:"reps"`
	Lapses     int     `json:"lapses"`
	DueAt      string  `json:"due_at"`
}

func (h *ReviewHandler) grade(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	var req gradeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErr(c, httpx.NewError(httpx.CodeValidation, "body không hợp lệ"))
		return
	}
	// Handler validate grade ∈ 1..4 TRƯỚC khi gọi service (fail fast, AD-14).
	if req.Grade < int16(scheddom.GradeAgain) || req.Grade > int16(scheddom.GradeEasy) {
		writeErr(c, httpx.NewError(httpx.CodeValidation, "grade phải trong 1..4").
			WithField("grade", "1..4"))
		return
	}
	cardID, err := uuid.Parse(req.CardID)
	if err != nil {
		writeErr(c, httpx.NewError(httpx.CodeValidation, "card_id không hợp lệ").
			WithField("card_id", "uuid"))
		return
	}
	if req.ClientReviewID == "" {
		writeErr(c, httpx.NewError(httpx.CodeValidation, "client_review_id bắt buộc").
			WithField("client_review_id", "required"))
		return
	}
	res, err := h.grader.Grade(c.Request.Context(), owner, revdom.GradeCommand{
		CardID: cardID, Grade: scheddom.Grade(req.Grade), ClientReviewID: req.ClientReviewID,
	})
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gradeResp{
		CardID: res.CardID.String(), Stability: res.Stability, Difficulty: res.Difficulty,
		Status: string(res.Status), Reps: res.Reps, Lapses: res.Lapses,
		DueAt: res.DueAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (h *ReviewHandler) queue(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	items, err := h.queuer.Queue(c.Request.Context(), owner, 50)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *ReviewHandler) summaryHandler(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	sum, err := h.summary.Summary(c.Request.Context(), owner)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, sum)
}

// owner đọc principal theo Auth Contract Sprint 1: authmw.UserID trả uuid dạng
// string; parse sang uuid.UUID ở ranh giới. KHÔNG dùng key thô "user_id".
func (h *ReviewHandler) owner(c *gin.Context) (uuid.UUID, bool) {
	uid, ok := authmw.UserID(c)
	if !ok {
		writeErr(c, httpx.NewError(httpx.CodeUnauthenticated, "authentication required"))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(uid)
	if err != nil {
		writeErr(c, httpx.NewError(httpx.CodeUnauthenticated, "authentication required"))
		return uuid.Nil, false
	}
	return id, true
}

// mapErr chuyển lỗi domain/service sang envelope chuẩn (AD-14): invalid grade →
// 400 VALIDATION_ERROR; not-found/ownership → 404; mặc định → 500.
func mapErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidGrade):
		writeErr(c, httpx.NewError(httpx.CodeValidation, "grade phải trong 1..4").
			WithField("grade", "1..4"))
	case errors.Is(err, scheddom.ErrCardNotFound):
		writeErr(c, httpx.NewError(httpx.CodeNotFound, "card không tồn tại"))
	default:
		writeErr(c, httpx.NewError(httpx.CodeInternal, "lỗi hệ thống"))
	}
}

func writeErr(c *gin.Context, e *httpx.APIError) {
	e = e.WithTrace(c.GetHeader("X-Trace-Id"))
	c.JSON(e.HTTPStatus(), e)
}
