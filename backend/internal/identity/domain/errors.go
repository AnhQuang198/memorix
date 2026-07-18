package domain

import "errors"

var (
	ErrNotFound           = errors.New("identity: not found")
	ErrEmailTaken         = errors.New("identity: email already registered")
	ErrWeakPassword       = errors.New("identity: password too weak")
	ErrInvalidCredentials = errors.New("identity: invalid credentials")
	ErrTokenInvalid       = errors.New("identity: token invalid or expired")
	ErrReuseDetected      = errors.New("identity: refresh token reuse detected")
	ErrRateLimited        = errors.New("identity: too many attempts")
	ErrInvalidProfile     = errors.New("identity: invalid profile field")
	ErrOAuthFailed        = errors.New("identity: oauth verification failed")
	ErrOAuthNoMerge       = errors.New("identity: cannot merge account on unverified email")
)
