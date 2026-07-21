package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/platform/eventbus"
	revdom "github.com/memorix/memorix/internal/review/domain"
	revports "github.com/memorix/memorix/internal/review/ports"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	schedports "github.com/memorix/memorix/internal/scheduling/ports"
	"github.com/memorix/memorix/internal/shared/events"
)

var ErrInvalidGrade = errors.New("grade must be 1..4")

// TxRunner chạy fn trong 1 transaction (db.WithinTx bọc pool ở cmd; fake trong test).
type TxRunner func(ctx context.Context, fn func(db.Querier) error) error

// GradeDeps gom phụ thuộc để wiring rõ ràng (S6).
type GradeDeps struct {
	Tx        TxRunner
	Scheduler schedports.SchedulerPort
	Cards     schedports.CardStore
	Prefs     schedports.PrefsStore
	Logs      revports.ReviewLogRepo
	Receipts  revports.ReceiptRepo
	Bus       eventbus.Bus
	Clock     func() time.Time
}

type GradeService struct{ d GradeDeps }

func NewGradeService(d GradeDeps) *GradeService {
	if d.Clock == nil {
		d.Clock = time.Now
	}
	return &GradeService{d: d}
}

// scheduledDaysOf suy "interval kế" (số ngày) từ lịch mới — khớp cách reconcile của
// progress tính (new_due_at::date - reviewed_at::date), so ở UTC để nhất quán N-day.
func scheduledDaysOf(reviewedAt, dueAt time.Time) int {
	r := reviewedAt.UTC()
	d := dueAt.UTC()
	rDay := time.Date(r.Year(), r.Month(), r.Day(), 0, 0, 0, 0, time.UTC)
	dDay := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	return int(dDay.Sub(rDay).Hours() / 24)
}

// Grade: server-authoritative (AD-5), nguyên tử (AD-3), idempotent (FR-15), append-only (AD-4).
func (s *GradeService) Grade(ctx context.Context, ownerID uuid.UUID, cmd revdom.GradeCommand) (revdom.GradeResult, error) {
	if !cmd.Grade.Valid() {
		return revdom.GradeResult{}, ErrInvalidGrade
	}
	now := s.d.Clock()
	var (
		result        revdom.GradeResult
		fresh         bool
		wasNew        bool
		scheduledDays int
	)

	err := s.d.Tx(ctx, func(q db.Querier) error {
		// 1. retry tuần tự: đã có receipt → trả kết quả cũ, không làm gì thêm.
		if prev, ok, err := s.d.Receipts.Get(ctx, q, cmd.CardID, cmd.ClientReviewID); err != nil {
			return err
		} else if ok {
			result = prev
			return nil
		}
		// 2. load card (ownership check) + prefs.
		card, err := s.d.Cards.Load(ctx, q, cmd.CardID, ownerID)
		if err != nil {
			return err
		}
		prefs, err := s.d.Prefs.Get(ctx, q, ownerID)
		if err != nil {
			return err
		}
		// 3. server tính S/D/Due (AD-5, AD-7).
		out := s.d.Scheduler.Apply(card, cmd.Grade, prefs.DesiredRetention, now)
		res := revdom.ResultFromSchedule(cmd.CardID, out)
		logID := uuid.New()
		// snapshot cho event CardGraded (phát ngoài TX): trạng thái New TRƯỚC khi chấm
		// + interval kế suy từ lịch mới.
		wasNew = card.Status == scheddom.StatusNew
		scheduledDays = scheduledDaysOf(now, out.DueAt)

		// 4. guard idempotency TRƯỚC khi append (chống race đa thiết bị).
		inserted, err := s.d.Receipts.Insert(ctx, q, res, logID, cmd.ClientReviewID)
		if err != nil {
			return err
		}
		if !inserted {
			prev, _, err := s.d.Receipts.Get(ctx, q, cmd.CardID, cmd.ClientReviewID)
			if err != nil {
				return err
			}
			result = prev
			return nil
		}
		// 5. append log (AD-4) + update card (AD-3) — cùng TX.
		if err := s.d.Logs.Append(ctx, q, revdom.ReviewLogRow{
			ID: logID, CardID: cmd.CardID, OwnerID: ownerID, ClientReviewID: cmd.ClientReviewID,
			Grade: cmd.Grade, PrevStability: card.Stability, PrevDifficulty: card.Difficulty,
			PrevStatus: card.Status, Retrievability: out.Retrievability,
			NewStability: out.Stability, NewDifficulty: out.Difficulty, NewStatus: out.Status,
			NewReps: out.Reps, NewLapses: out.Lapses, NewDueAt: out.DueAt,
			ElapsedDays: out.ElapsedDays, ReviewedAt: now,
		}); err != nil {
			return err
		}
		if err := s.d.Cards.ApplyResult(ctx, q, cmd.CardID, out); err != nil {
			return err
		}
		result = res
		fresh = true
		return nil
	})
	if err != nil {
		return revdom.GradeResult{}, err
	}

	// 6. phát event NGOÀI TX chấm (AD-8), chỉ khi chấm mới. Dùng hợp đồng chung
	// shared/events.CardGraded để progress subscribe (OwnerID/CardID dạng string).
	if fresh && s.d.Bus != nil {
		s.d.Bus.Publish(ctx, eventbus.Event{Name: events.CardGradedName, Payload: events.CardGraded{
			OwnerID:       ownerID.String(),
			CardID:        cmd.CardID.String(),
			Grade:         int(cmd.Grade),
			ScheduledDays: scheduledDays,
			WasNew:        wasNew,
			ReviewedAt:    now,
		}})
	}
	return result, nil
}
