package domain

import (
	"time"

	"github.com/google/uuid"
)

type CardStatus string

const (
	StatusNew        CardStatus = "new"
	StatusLearning   CardStatus = "learning"
	StatusReview     CardStatus = "review"
	StatusRelearning CardStatus = "relearning"
	StatusSuspended  CardStatus = "suspended"
)

func (s CardStatus) Valid() bool {
	switch s {
	case StatusNew, StatusLearning, StatusReview, StatusRelearning, StatusSuspended:
		return true
	}
	return false
}

type Direction string

const (
	DirectionFrontBack Direction = "front_back"
	DirectionBackFront Direction = "back_front"
)

func (d Direction) Valid() bool {
	return d == DirectionFrontBack || d == DirectionBackFront
}

// Card giữ trạng thái học per-user/per-direction (AD-6). Sprint 2 chỉ tạo New.
type Card struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	EntryID    uuid.UUID
	Direction  Direction
	Status     CardStatus
	DueAt      *time.Time
	Stability  float64
	Difficulty float64
	Reps       int
	Lapses     int
	// LastReviewAt là thời điểm ôn gần nhất; nil khi card còn New. FSRS adapter
	// dùng để tính elapsed-days cho forgetting curve.
	LastReviewAt *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
