package domain

import "github.com/google/uuid"

// SchedulerPrefs = cấu hình lịch per-user (FR-17, FR-26).
type SchedulerPrefs struct {
	UserID           uuid.UUID
	DesiredRetention float64
	DailyNewLimit    int
	DailyReviewLimit int
	Timezone         string
}

// DefaultPrefs khi user chưa cấu hình.
func DefaultPrefs() SchedulerPrefs {
	return SchedulerPrefs{
		DesiredRetention: 0.90,
		DailyNewLimit:    20,
		DailyReviewLimit: 200,
		Timezone:         "UTC",
	}
}

// RetentionInRange kiểm tra desired retention hợp lệ (0.80–0.97, FR-17).
func RetentionInRange(r float64) bool { return r >= 0.80 && r <= 0.97 }
