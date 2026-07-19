package ports

import (
	"time"

	"github.com/memorix/memorix/internal/scheduling/domain"
)

// SchedulerPort bọc toán FSRS (AD-7). domain KHÔNG import go-fsrs; adapter
// (scheduling/repo/fsrsadapter) implement port này bằng go-fsrs. Cho phép A/B
// nhiều impl trên cùng review_logs.
type SchedulerPort interface {
	// Apply tính trạng thái card sau khi chấm `grade` tại `now` với desired retention.
	Apply(card domain.Card, grade domain.Grade, retention float64, now time.Time) domain.ScheduleResult
	// Preview trả khoảng cách ôn kế cho cả 4 mức (FR-14), không thay đổi card.
	Preview(card domain.Card, retention float64, now time.Time) domain.NextIntervals
}
