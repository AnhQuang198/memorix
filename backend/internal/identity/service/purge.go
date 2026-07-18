package service

import (
	"context"
	"time"
)

// PurgeDeletedAccounts xóa cứng tài khoản đã soft-delete quá `retention` tính
// đến `now` (Story 1.8: soft-delete → purge theo lịch). CASCADE dọn
// sessions/email_tokens/oauth_identities (FK ON DELETE CASCADE trong schema).
func (s *Service) PurgeDeletedAccounts(ctx context.Context, retention time.Duration, now time.Time) (int, error) {
	cutoff := now.Add(-retention)
	return s.deps.Users.PurgeDeletedBefore(ctx, cutoff)
}
