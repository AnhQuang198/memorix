// Package fsrsadapter implement ports.SchedulerPort bằng go-fsrs v3 (AD-7). Đây
// là NƠI DUY NHẤT trong codebase được phép import go-fsrs; domain/ports/service
// không đụng tới lib (depguard giữ ranh giới này). Fuzz TẮT để lịch replay-được
// tất định (AD-4).
package fsrsadapter

import (
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v3"

	"github.com/memorix/memorix/internal/scheduling/domain"
)

// Adapter bọc toán FSRS sau SchedulerPort.
type Adapter struct{}

// New tạo Adapter (stateless; Parameters dựng lại mỗi lần gọi theo retention của user).
func New() *Adapter { return &Adapter{} }

// params dựng Parameters với desired retention của user; fuzz TẮT cho determinism.
func (a *Adapter) params(retention float64) fsrs.Parameters {
	p := fsrs.DefaultParam()
	p.RequestRetention = retention
	p.EnableFuzz = false // replay-được (AD-4); fuzz load-balancing là future extension
	return p
}

// toFSRSState map domain CardStatus (string) → go-fsrs State (int enum). go-fsrs
// KHÔNG có khái niệm "suspended"; card suspended không được đưa vào chấm nên map
// an toàn về New. Mapping tường minh — KHÔNG giả định hai enum trùng giá trị int.
func toFSRSState(s domain.CardStatus) fsrs.State {
	switch s {
	case domain.StatusLearning:
		return fsrs.Learning
	case domain.StatusReview:
		return fsrs.Review
	case domain.StatusRelearning:
		return fsrs.Relearning
	case domain.StatusNew, domain.StatusSuspended:
		return fsrs.New
	default:
		return fsrs.New
	}
}

// fromFSRSState map go-fsrs State (int enum) → domain CardStatus (string). Tường minh.
func fromFSRSState(s fsrs.State) domain.CardStatus {
	switch s {
	case fsrs.Learning:
		return domain.StatusLearning
	case fsrs.Review:
		return domain.StatusReview
	case fsrs.Relearning:
		return domain.StatusRelearning
	case fsrs.New:
		return domain.StatusNew
	default:
		return domain.StatusNew
	}
}

// toRating map domain Grade → go-fsrs Rating tường minh (dù cùng dải 1..4, hai
// type khác nhau nên KHÔNG ép kiểu ngầm).
func toRating(g domain.Grade) fsrs.Rating {
	switch g {
	case domain.GradeAgain:
		return fsrs.Again
	case domain.GradeHard:
		return fsrs.Hard
	case domain.GradeGood:
		return fsrs.Good
	case domain.GradeEasy:
		return fsrs.Easy
	default:
		return fsrs.Good
	}
}

// toFSRS chuyển domain.Card sang fsrs.Card. Card New chưa có stability/difficulty
// nên để mặc định 0; card đã học mang giá trị hiện tại vào.
func toFSRS(c domain.Card) fsrs.Card {
	fc := fsrs.NewCard()
	fc.State = toFSRSState(c.Status)
	fc.Reps = uint64(c.Reps)
	fc.Lapses = uint64(c.Lapses)
	if c.DueAt != nil {
		fc.Due = *c.DueAt
	}
	if c.Status != domain.StatusNew {
		fc.Stability = c.Stability
		fc.Difficulty = c.Difficulty
	}
	if c.LastReviewAt != nil {
		fc.LastReview = *c.LastReviewAt
	}
	return fc
}

// Apply tính trạng thái card sau khi chấm grade tại now với desired retention.
func (a *Adapter) Apply(card domain.Card, grade domain.Grade, retention float64, now time.Time) domain.ScheduleResult {
	f := fsrs.NewFSRS(a.params(retention))
	fc := toFSRS(card)
	r := f.GetRetrievability(fc, now)
	info := f.Next(fc, now, toRating(grade))
	nc := info.Card
	return domain.ScheduleResult{
		Stability:      nc.Stability,
		Difficulty:     nc.Difficulty,
		Status:         fromFSRSState(nc.State),
		Reps:           int(nc.Reps),
		Lapses:         int(nc.Lapses),
		DueAt:          nc.Due,
		LastReviewAt:   now,
		ElapsedDays:    int(nc.ElapsedDays),
		Retrievability: r,
	}
}

// Preview trả khoảng cách ôn kế cho cả 4 mức (FR-14), không thay đổi card.
func (a *Adapter) Preview(card domain.Card, retention float64, now time.Time) domain.NextIntervals {
	f := fsrs.NewFSRS(a.params(retention))
	fc := toFSRS(card)
	m := f.Repeat(fc, now)
	return domain.NextIntervals{
		Again: m[fsrs.Again].Card.Due.Sub(now),
		Hard:  m[fsrs.Hard].Card.Due.Sub(now),
		Good:  m[fsrs.Good].Card.Due.Sub(now),
		Easy:  m[fsrs.Easy].Card.Due.Sub(now),
	}
}

// compile-time check: Adapter thỏa SchedulerPort (không import ports để tránh
// vòng phụ thuộc; chữ ký khớp 1-1 với ports.SchedulerPort).
var _ interface {
	Apply(domain.Card, domain.Grade, float64, time.Time) domain.ScheduleResult
	Preview(domain.Card, float64, time.Time) domain.NextIntervals
} = (*Adapter)(nil)
