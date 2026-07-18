package handler

import "github.com/memorix/memorix/internal/identity/domain"

var (
	errBadJSON         = domain.ErrInvalidProfile // 400 VALIDATION_ERROR cho body sai
	domainTokenInvalid = domain.ErrTokenInvalid
	domainOAuthFailed  = domain.ErrOAuthFailed
)
