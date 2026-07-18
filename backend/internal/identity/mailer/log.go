package mailer

import (
	"context"
	"log/slog"
)

// LogMailer ghi log sự kiện gửi mail (MVP). KHÔNG log raw token (NFR-14) — chỉ
// độ dài để chẩn đoán. Thay bằng SMTP/provider adapter ở prod.
type LogMailer struct{ log *slog.Logger }

func NewLogMailer(log *slog.Logger) *LogMailer { return &LogMailer{log: log} }

func (m *LogMailer) SendVerification(_ context.Context, email, rawToken string) error {
	m.log.Info("send verification email", "email", email, "token_len", len(rawToken))
	return nil
}

func (m *LogMailer) SendPasswordReset(_ context.Context, email, rawToken string) error {
	m.log.Info("send password reset email", "email", email, "token_len", len(rawToken))
	return nil
}
