// Package handler là adapter Gin cho read model Progress (bind → service → envelope).
package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/progress/service"
)

// Reader là cổng service mà handler cần (định nghĩa ở phía gọi — AD-1).
type Reader interface {
	Dashboard(ctx context.Context, userID string, now time.Time, loc *time.Location) (service.DashboardView, error)
	Stats(ctx context.Context, userID string, now time.Time, loc *time.Location) (service.StatsView, error)
}

// TZResolver phân giải TZ user cho "ngày học" (AD-12). Request-path wrap
// IdentityPort.UserTimezone (AD-9, wired Task 10); test dùng double.
type TZResolver interface {
	Location(ctx context.Context, userID string) *time.Location
}

type Handler struct {
	svc Reader
	tz  TZResolver // request-path: wrap IdentityPort.UserTimezone (AD-9); test dùng double
	now func() time.Time
}

func New(svc Reader, tz TZResolver) *Handler {
	return &Handler{svc: svc, tz: tz, now: time.Now}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	pg := g.Group("/progress")
	pg.GET("/dashboard", h.dashboard)
	pg.GET("/stats", h.stats)
}

// userLoc phân giải TZ user qua TZResolver (backed by IdentityPort ở prod),
// KHÔNG đọc từ gin context (TZ không nằm trong principal — Auth Contract).
func (h *Handler) userLoc(ctx context.Context, uid string) *time.Location {
	return h.tz.Location(ctx, uid)
}

func (h *Handler) dashboard(c *gin.Context) {
	uid, _ := authmw.UserID(c)                 // canonical reader (Sprint 1)
	loc := h.userLoc(c.Request.Context(), uid) // TZ qua IdentityPort, KHÔNG từ context
	v, err := h.svc.Dashboard(c.Request.Context(), uid, h.now(), loc)
	if err != nil {
		e := httpx.NewError(httpx.CodeInternal, "không tải được trang chủ")
		c.JSON(e.HTTPStatus(), e)
		return
	}
	c.JSON(200, gin.H{"data": v})
}

func (h *Handler) stats(c *gin.Context) {
	uid, _ := authmw.UserID(c)                 // canonical reader (Sprint 1)
	loc := h.userLoc(c.Request.Context(), uid) // TZ qua IdentityPort, KHÔNG từ context
	v, err := h.svc.Stats(c.Request.Context(), uid, h.now(), loc)
	if err != nil {
		e := httpx.NewError(httpx.CodeInternal, "không tải được thống kê")
		c.JSON(e.HTTPStatus(), e)
		return
	}
	c.JSON(200, gin.H{"data": v})
}
