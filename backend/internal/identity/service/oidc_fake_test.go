package service

import (
	"context"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/identity/ports"
)

type stubOIDC struct {
	claims ports.OIDCClaims
	err    error
}

func (s stubOIDC) Verify(context.Context, string, string, string, string, string) (ports.OIDCClaims, error) {
	if s.err != nil {
		return ports.OIDCClaims{}, s.err
	}
	return s.claims, nil
}

var _ ports.OIDCVerifier = stubOIDC{}
var _ = domain.ErrOAuthFailed
