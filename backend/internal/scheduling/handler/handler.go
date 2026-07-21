package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/service"
)

// QueueBuilder dựng queue học hôm nay (QueueService.BuildToday, Story 4.1–4.4).
type QueueBuilder interface {
	BuildToday(ctx context.Context, userID uuid.UUID, now time.Time) (domain.QueueResult, error)
}

// LearnProvider phục vụ luồng học thẻ New RIÊNG + cờ coach (LearnService, Story 4.5).
type LearnProvider interface {
	StartSession(ctx context.Context, userID uuid.UUID, now time.Time) (service.LearnSession, error)
	AckCoach(ctx context.Context, userID uuid.UUID) error
}

// PrefsUpdater cập nhật giới hạn new/review hằng ngày (PATCH prefs, FR-27).
type PrefsUpdater interface {
	UpdateLimits(ctx context.Context, userID uuid.UUID, newLimit, reviewLimit int) (domain.SchedulerPrefs, error)
}

// RegisterQueueRoutes gắn endpoint queue/learn/coach/prefs-limits vào group
// /api/v1. Tách tên khỏi prefs.go's RegisterRoutes (S3) để cùng tồn tại trong
// package handler mà KHÔNG clobber handler prefs cũ. Guard authmw.RequireAuth
// được wire ở cmd/api, KHÔNG ở đây.
func RegisterQueueRoutes(g *gin.RouterGroup, q QueueBuilder, l LearnProvider, p PrefsUpdater) {
	h := &handlers{q: q, l: l, p: p}
	g.GET("/queue", h.getQueue)
	g.GET("/learn", h.getLearn)
	g.POST("/learn/coach/ack", h.ackCoach)
	g.PATCH("/scheduler/prefs", h.updatePrefs)
}

type handlers struct {
	q QueueBuilder
	l LearnProvider
	p PrefsUpdater
}

// cardDTO là hình chiếu JSON của domain.Card cho client (id/entry/status/due).
type cardDTO struct {
	ID      uuid.UUID         `json:"id"`
	EntryID uuid.UUID         `json:"entry_id"`
	Status  domain.CardStatus `json:"status"`
	DueAt   *time.Time        `json:"due_at"`
}

func toDTOs(cards []domain.Card) []cardDTO {
	out := make([]cardDTO, len(cards))
	for i, c := range cards {
		out[i] = cardDTO{ID: c.ID, EntryID: c.EntryID, Status: c.Status, DueAt: c.DueAt}
	}
	return out
}

// principalID đọc principal theo Auth Contract Sprint 1: authmw.UserID trả
// (string, bool); UserID là uuid dạng string → parse ở ranh giới. KHÔNG đọc key
// thô "user_id".
func principalID(c *gin.Context) (uuid.UUID, bool) {
	uid, ok := authmw.UserID(c)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(uid)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func abort(c *gin.Context, e *httpx.APIError) {
	c.JSON(e.HTTPStatus(), e.WithTrace(c.GetHeader("X-Trace-Id")))
}

func (h *handlers) getQueue(c *gin.Context) {
	uid, ok := principalID(c)
	if !ok {
		abort(c, httpx.NewError(httpx.CodeUnauthenticated, "cần đăng nhập"))
		return
	}
	res, err := h.q.BuildToday(c.Request.Context(), uid, time.Now())
	if err != nil {
		abort(c, httpx.NewError(httpx.CodeInternal, "không dựng được queue"))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"cards":        toDTOs(res.Cards),
		"new_count":    res.NewCount,
		"review_count": res.ReviewCount,
	})
}

func (h *handlers) getLearn(c *gin.Context) {
	uid, ok := principalID(c)
	if !ok {
		abort(c, httpx.NewError(httpx.CodeUnauthenticated, "cần đăng nhập"))
		return
	}
	sess, err := h.l.StartSession(c.Request.Context(), uid, time.Now())
	if err != nil {
		abort(c, httpx.NewError(httpx.CodeInternal, "không mở được phiên học"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"cards": toDTOs(sess.Cards), "show_coach": sess.ShowCoach})
}

func (h *handlers) ackCoach(c *gin.Context) {
	uid, ok := principalID(c)
	if !ok {
		abort(c, httpx.NewError(httpx.CodeUnauthenticated, "cần đăng nhập"))
		return
	}
	if err := h.l.AckCoach(c.Request.Context(), uid); err != nil {
		abort(c, httpx.NewError(httpx.CodeInternal, "không lưu được"))
		return
	}
	c.Status(http.StatusNoContent)
}

type updateLimitsReq struct {
	DailyNewLimit    int `json:"daily_new_limit"`
	DailyReviewLimit int `json:"daily_review_limit"`
}

func (h *handlers) updatePrefs(c *gin.Context) {
	uid, ok := principalID(c)
	if !ok {
		abort(c, httpx.NewError(httpx.CodeUnauthenticated, "cần đăng nhập"))
		return
	}
	var req updateLimitsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		abort(c, httpx.NewError(httpx.CodeValidation, "body không hợp lệ"))
		return
	}
	if e := validateLimit("daily_new_limit", req.DailyNewLimit); e != nil {
		abort(c, e)
		return
	}
	if e := validateLimit("daily_review_limit", req.DailyReviewLimit); e != nil {
		abort(c, e)
		return
	}
	prefs, err := h.p.UpdateLimits(c.Request.Context(), uid, req.DailyNewLimit, req.DailyReviewLimit)
	if err != nil {
		abort(c, httpx.NewError(httpx.CodeInternal, "không cập nhật được"))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"daily_new_limit":    prefs.DailyNewLimit,
		"daily_review_limit": prefs.DailyReviewLimit,
	})
}

func validateLimit(field string, v int) *httpx.APIError {
	if v < 1 || v > 9999 {
		return httpx.NewError(httpx.CodeValidation, "giới hạn phải trong 1..9999").WithField(field, "1..9999")
	}
	return nil
}
