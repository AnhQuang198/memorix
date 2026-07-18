package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/platform/httpx"
)

// writeErr map domain error → APIError envelope (AD-14). Mặc định 500.
func writeErr(c *gin.Context, err error) {
	var e *httpx.APIError
	switch {
	case errors.Is(err, domain.ErrWeakPassword):
		e = httpx.NewError(httpx.CodeValidation, "password is too weak").WithField("password", "choose a stronger password")
	case errors.Is(err, domain.ErrInvalidProfile):
		e = httpx.NewError(httpx.CodeValidation, "invalid profile value")
	case errors.Is(err, domain.ErrEmailTaken):
		e = httpx.NewError(httpx.CodeConflict, "email already registered")
	case errors.Is(err, domain.ErrInvalidCredentials):
		e = httpx.NewError(httpx.CodeUnauthenticated, "invalid email or password")
	case errors.Is(err, domain.ErrTokenInvalid):
		e = httpx.NewError(httpx.CodeUnauthenticated, "token invalid or expired")
	case errors.Is(err, domain.ErrReuseDetected):
		e = httpx.NewError(httpx.CodeUnauthenticated, "session revoked, please sign in again")
	case errors.Is(err, domain.ErrRateLimited):
		e = httpx.NewError(httpx.CodeRateLimited, "too many attempts, try again later")
	case errors.Is(err, domain.ErrOAuthNoMerge):
		e = httpx.NewError(httpx.CodeConflict, "an account with this email already exists")
	case errors.Is(err, domain.ErrOAuthFailed):
		e = httpx.NewError(httpx.CodeUnauthenticated, "oauth verification failed")
	case errors.Is(err, domain.ErrNotFound):
		e = httpx.NewError(httpx.CodeNotFound, "not found")
	default:
		e = httpx.NewError(httpx.CodeInternal, "internal error")
	}
	e = e.WithTrace(c.GetHeader("X-Request-Id"))
	c.JSON(e.HTTPStatus(), e)
}
