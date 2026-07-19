package domain

import "time"

// ScheduleResult = kết quả FSRS sau một lần chấm (do SchedulerPort trả).
type ScheduleResult struct {
	Stability      float64
	Difficulty     float64
	Status         CardStatus
	Reps           int
	Lapses         int
	DueAt          time.Time
	LastReviewAt   time.Time
	ElapsedDays    int
	Retrievability float64
}

// NextIntervals = khoảng cách tới lần ôn kế cho từng mức (FR-14). Server tính, client hiển thị.
type NextIntervals struct {
	Again time.Duration
	Hard  time.Duration
	Good  time.Duration
	Easy  time.Duration
}
