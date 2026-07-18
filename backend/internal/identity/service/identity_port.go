package service

import (
	"context"
	"errors"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/identity/ports"
)

// Port implements ports.IdentityPort — mặt tiền để module khác (vd scheduling
// cần TZ user tính "ngày học" AD-12) hỏi identity mà không chạm bảng của nó.
type Port struct{ users ports.UserRepo }

func NewPort(users ports.UserRepo) *Port { return &Port{users: users} }

func (p *Port) UserExists(ctx context.Context, id string) (bool, error) {
	_, err := p.users.ByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (p *Port) UserTimezone(ctx context.Context, id string) (string, error) {
	u, err := p.users.ByID(ctx, id)
	if err != nil {
		return "", err
	}
	return u.Timezone, nil
}

var _ ports.IdentityPort = (*Port)(nil)
