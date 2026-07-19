// Package handler là adapter Gin của scheduling prefs: bind/validate → service
// (AD-2). Principal đọc qua authmw.UserID theo Auth Contract Sprint 1.
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/service"
)

// Handler phục vụ GET/PUT /api/v1/scheduler/prefs trên PrefsService.
type Handler struct {
	svc *service.PrefsService
}

func New(svc *service.PrefsService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes gắn route scheduler prefs vào group /api/v1. Guard
// authmw.RequireAuth được wire ở cmd/api (Task 16), KHÔNG ở đây.
func RegisterRoutes(g *gin.RouterGroup, h *Handler) {
	g.GET("/scheduler/prefs", h.get)
	g.PUT("/scheduler/prefs", h.put)
}

// prefsReq = body PUT cấu hình lịch (FR-17, FR-26).
type prefsReq struct {
	DesiredRetention float64 `json:"desired_retention"`
	DailyNewLimit    int     `json:"daily_new_limit"`
	DailyReviewLimit int     `json:"daily_review_limit"`
	Timezone         string  `json:"timezone"`
}

func (h *Handler) get(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	p, err := h.svc.Get(c.Request.Context(), owner)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, prefsToJSON(p))
}

func (h *Handler) put(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	var req prefsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErr(c, httpx.NewError(httpx.CodeValidation, "body không hợp lệ"))
		return
	}
	if req.DailyNewLimit == 0 {
		req.DailyNewLimit = domain.DefaultPrefs().DailyNewLimit
	}
	if req.DailyReviewLimit == 0 {
		req.DailyReviewLimit = domain.DefaultPrefs().DailyReviewLimit
	}
	p, err := h.svc.Update(c.Request.Context(), owner, service.PrefsUpdate{
		DesiredRetention: req.DesiredRetention,
		DailyNewLimit:    req.DailyNewLimit,
		DailyReviewLimit: req.DailyReviewLimit,
		Timezone:         req.Timezone,
	})
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, prefsToJSON(p))
}

// owner đọc principal theo Auth Contract Sprint 1: authmw.UserID trả uuid dạng
// string; parse sang uuid.UUID ở ranh giới. KHÔNG dùng key thô "user_id".
func (h *Handler) owner(c *gin.Context) (uuid.UUID, bool) {
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

func prefsToJSON(p domain.SchedulerPrefs) gin.H {
	return gin.H{
		"desired_retention":  p.DesiredRetention,
		"daily_new_limit":    p.DailyNewLimit,
		"daily_review_limit": p.DailyReviewLimit,
		"timezone":           p.Timezone,
	}
}

// mapErr chuyển lỗi service/domain sang envelope chuẩn (AD-14):
// retention-range/bad-tz → 400 VALIDATION_ERROR kèm field.
func mapErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrRetentionRange):
		writeErr(c, httpx.NewError(httpx.CodeValidation, err.Error()).
			WithField("desired_retention", "0.80–0.97"))
	case errors.Is(err, service.ErrBadTimezone):
		writeErr(c, httpx.NewError(httpx.CodeValidation, err.Error()).
			WithField("timezone", "IANA tz"))
	default:
		writeErr(c, httpx.NewError(httpx.CodeInternal, "lỗi hệ thống"))
	}
}

func writeErr(c *gin.Context, e *httpx.APIError) {
	e = e.WithTrace(c.GetHeader("X-Trace-Id"))
	c.JSON(e.HTTPStatus(), e)
}
