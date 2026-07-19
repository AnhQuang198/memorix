package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/platform/db"
	revports "github.com/memorix/memorix/internal/review/ports"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	schedports "github.com/memorix/memorix/internal/scheduling/ports"
)

// SessionSummary = số liệu cuối phiên (FR-24). Đọc thẳng review_logs (AD-8, không lag).
type SessionSummary struct {
	Reviewed         int `json:"reviewed"`
	Remembered       int `json:"remembered"`
	ForecastTomorrow int `json:"forecast_tomorrow"`
}

type SummaryDeps struct {
	Pool     *pgxpool.Pool
	RunQuery QueryRunner
	Logs     revports.ReviewLogRepo
	Cards    schedports.CardStore
	Prefs    schedports.PrefsStore
	Clock    func() time.Time
}

type SummaryService struct{ d SummaryDeps }

func NewSummaryService(d SummaryDeps) *SummaryService {
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.RunQuery == nil && d.Pool != nil {
		p := d.Pool
		d.RunQuery = func(_ context.Context, fn func(db.Querier) error) error { return fn(p) }
	}
	return &SummaryService{d: d}
}

func (s *SummaryService) Summary(ctx context.Context, ownerID uuid.UUID) (SessionSummary, error) {
	now := s.d.Clock()
	var out SessionSummary
	err := s.d.RunQuery(ctx, func(q db.Querier) error {
		prefs, err := s.d.Prefs.Get(ctx, q, ownerID)
		if err != nil {
			return err
		}
		loc, err := time.LoadLocation(prefs.Timezone)
		if err != nil {
			loc = time.UTC
		}
		dayStart := scheddom.StudyDayStart(now, loc, 4) // grace 4h sáng (AD-12)

		rows, err := s.d.Logs.ListForOwnerSince(ctx, q, ownerID, dayStart.Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		out.Reviewed = len(rows)
		for _, r := range rows {
			if r.Grade >= scheddom.GradeGood {
				out.Remembered++
			}
		}

		// forecast mai: số thẻ due trước cuối ngày-học kế (dayStart + 48h là biên an toàn).
		tomorrowEnd := dayStart.AddDate(0, 0, 2)
		due, err := s.d.Cards.DueCards(ctx, q, ownerID, tomorrowEnd, 10000)
		if err != nil {
			return err
		}
		out.ForecastTomorrow = len(due)
		return nil
	})
	return out, err
}
