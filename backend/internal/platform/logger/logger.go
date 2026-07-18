package logger

import (
	"io"
	"log/slog"
	"strings"
)

var sensitive = map[string]bool{
	"password": true, "token": true, "refresh_token": true,
	"access_token": true, "authorization": true, "secret": true,
}

// Scrub thay giá trị field nhạy cảm bằng [REDACTED] (NFR-14).
func Scrub(key, val string) string {
	if sensitive[strings.ToLower(key)] {
		return "[REDACTED]"
	}
	return val
}

func New(w io.Writer, level string) *slog.Logger {
	var lv slog.Level
	_ = lv.UnmarshalText([]byte(level))
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lv}))
}
