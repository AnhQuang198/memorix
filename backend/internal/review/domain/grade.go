package domain

import (
	"time"

	"github.com/google/uuid"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
)

// GradeCommand = payload duy nhất client gửi (AD-5). KHÔNG có S/D/Due.
type GradeCommand struct {
	CardID         uuid.UUID
	Grade          scheddom.Grade
	ClientReviewID string
}

// GradeResult = trạng thái card sau chấm, trả cho client + lưu ở grade_receipts.
type GradeResult struct {
	CardID     uuid.UUID
	Stability  float64
	Difficulty float64
	Status     scheddom.CardStatus
	Reps       int
	Lapses     int
	DueAt      time.Time
}

// ResultFromSchedule chuyển ScheduleResult (do SchedulerPort trả) thành GradeResult
// gắn với card cụ thể để trả client + lưu receipt.
func ResultFromSchedule(cardID uuid.UUID, r scheddom.ScheduleResult) GradeResult {
	return GradeResult{
		CardID: cardID, Stability: r.Stability, Difficulty: r.Difficulty,
		Status: r.Status, Reps: r.Reps, Lapses: r.Lapses, DueAt: r.DueAt,
	}
}

// ReviewLogRow = một dòng append-only ở review.review_logs (AD-4 replay source).
type ReviewLogRow struct {
	ID             uuid.UUID
	CardID         uuid.UUID
	OwnerID        uuid.UUID
	ClientReviewID string
	Grade          scheddom.Grade
	PrevStability  float64
	PrevDifficulty float64
	PrevStatus     scheddom.CardStatus
	Retrievability float64
	NewStability   float64
	NewDifficulty  float64
	NewStatus      scheddom.CardStatus
	NewReps        int
	NewLapses      int
	NewDueAt       time.Time
	ElapsedDays    int
	ReviewedAt     time.Time
}
