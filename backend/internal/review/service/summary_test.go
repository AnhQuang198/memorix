package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db"
	revdom "github.com/memorix/memorix/internal/review/domain"
	"github.com/memorix/memorix/internal/review/service"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/stretchr/testify/require"
)

type summaryLogs struct{ rows []revdom.ReviewLogRow }

func (s *summaryLogs) Append(context.Context, db.Querier, revdom.ReviewLogRow) error { return nil }
func (s *summaryLogs) ListForCard(context.Context, db.Querier, uuid.UUID) ([]revdom.ReviewLogRow, error) {
	return nil, nil
}
func (s *summaryLogs) ListForOwnerSince(_ context.Context, _ db.Querier, _ uuid.UUID, _ string) ([]revdom.ReviewLogRow, error) {
	return s.rows, nil
}

type summaryCards struct{ dueTomorrow int }

func (c *summaryCards) Load(context.Context, db.Querier, uuid.UUID, uuid.UUID) (scheddom.Card, error) {
	return scheddom.Card{}, nil
}
func (c *summaryCards) ApplyResult(context.Context, db.Querier, uuid.UUID, scheddom.ScheduleResult) error {
	return nil
}
func (c *summaryCards) DueCards(_ context.Context, _ db.Querier, _ uuid.UUID, until time.Time, _ int) ([]scheddom.Card, error) {
	out := make([]scheddom.Card, c.dueTomorrow)
	return out, nil
}

func TestSummary_CountsRememberedAndForecast(t *testing.T) {
	now := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	rows := []revdom.ReviewLogRow{
		{Grade: scheddom.GradeGood, ReviewedAt: now.Add(-2 * time.Hour)},
		{Grade: scheddom.GradeEasy, ReviewedAt: now.Add(-time.Hour)},
		{Grade: scheddom.GradeAgain, ReviewedAt: now.Add(-30 * time.Minute)}, // quên → không tính nhớ
	}
	svc := service.NewSummaryService(service.SummaryDeps{
		RunQuery: func(_ context.Context, fn func(db.Querier) error) error { return fn(nil) },
		Logs:     &summaryLogs{rows: rows}, Cards: &summaryCards{dueTomorrow: 7},
		Prefs: fakePrefs{}, Clock: func() time.Time { return now },
	})
	sum, err := svc.Summary(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Equal(t, 3, sum.Reviewed)
	require.Equal(t, 2, sum.Remembered, "chỉ grade ≥ Good tính là nhớ")
	require.Equal(t, 7, sum.ForecastTomorrow)
}
