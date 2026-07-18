package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestNew_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, "info")
	l.Info("hello", "user", "linh")
	out := buf.String()
	if !strings.Contains(out, `"msg":"hello"`) || !strings.Contains(out, `"user":"linh"`) {
		t.Errorf("expected JSON log, got %q", out)
	}
}

func TestScrub_RedactsSensitive(t *testing.T) {
	for _, k := range []string{"password", "token", "refresh_token", "authorization"} {
		if Scrub(k, "secret") != "[REDACTED]" {
			t.Errorf("key %q not scrubbed", k)
		}
	}
	if Scrub("email", "a@b.com") != "a@b.com" {
		t.Error("non-sensitive key should pass through")
	}
}
