package ports_test

import (
	"testing"

	"github.com/memorix/memorix/internal/scheduling/ports"
)

// Ép compile: xác nhận interface tồn tại + chữ ký ổn định.
func TestPortsDeclared(t *testing.T) {
	var _ ports.SchedulerPort
	var _ ports.CardStore
	var _ ports.PrefsStore
}
