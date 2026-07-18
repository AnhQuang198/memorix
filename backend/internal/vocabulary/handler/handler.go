// Package handler là adapter Gin của vocabulary (bind/validate → service).
package handler

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
	"github.com/memorix/memorix/internal/vocabulary/service"
)

// VocabService là mặt service handler cần (interface để test bằng fake).
type VocabService interface {
	Create(ctx context.Context, owner uuid.UUID, in service.CreateEntryInput) (domain.Entry, error)
	Get(ctx context.Context, owner, id uuid.UUID) (service.EntryView, error)
	Update(ctx context.Context, owner, id uuid.UUID, in service.UpdateEntryInput) (domain.Entry, error)
	Delete(ctx context.Context, owner, id uuid.UUID) error
	List(ctx context.Context, in service.ListInput) (service.ListResult, error)
	ListCuratedDecks(ctx context.Context) ([]domain.CuratedDeck, error)
	Enroll(ctx context.Context, owner, deckID uuid.UUID) (uuid.UUID, error)
}

type Handler struct {
	svc VocabService
}

func New(svc VocabService) *Handler {
	return &Handler{svc: svc}
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

func writeErr(c *gin.Context, e *httpx.APIError) {
	e = e.WithTrace(c.GetHeader("X-Trace-Id"))
	c.JSON(e.HTTPStatus(), e)
}

// mapErr chuyển lỗi service/domain sang envelope chuẩn (AD-14).
func mapErr(c *gin.Context, err error) {
	var dup service.DuplicateError
	switch {
	case errors.As(err, &dup):
		writeErr(c, httpx.NewError(httpx.CodeConflict, "từ đã tồn tại").
			WithField("existing_id", dup.ExistingID.String()))
	case errors.Is(err, domain.ErrTermRequired):
		writeErr(c, httpx.NewError(httpx.CodeValidation, "term bắt buộc").WithField("term", "bắt buộc"))
	case errors.Is(err, domain.ErrEntryNotFound):
		writeErr(c, httpx.NewError(httpx.CodeNotFound, "không tìm thấy từ"))
	case errors.Is(err, domain.ErrDeckNotFound):
		writeErr(c, httpx.NewError(httpx.CodeNotFound, "không tìm thấy bộ thẻ"))
	case errors.Is(err, domain.ErrAlreadyEnrolled):
		writeErr(c, httpx.NewError(httpx.CodeConflict, "đã enroll bộ này"))
	default:
		writeErr(c, httpx.NewError(httpx.CodeInternal, "lỗi hệ thống"))
	}
}

func parseID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		writeErr(c, httpx.NewError(httpx.CodeValidation, "id không hợp lệ").WithField(name, "phải là uuid"))
		return uuid.Nil, false
	}
	return id, true
}

var validStatuses = map[string]bool{
	"new": true, "learning": true, "review": true, "relearning": true, "suspended": true,
}
