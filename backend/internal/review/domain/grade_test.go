package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/review/domain"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/stretchr/testify/require"
)

func TestResultFromScheduleResult(t *testing.T) {
	cid := uuid.New()
	due := time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC)
	sr := scheddom.ScheduleResult{
		Stability: 12.5, Difficulty: 6, Status: scheddom.StatusReview,
		Reps: 1, Lapses: 0, DueAt: due,
	}
	got := domain.ResultFromSchedule(cid, sr)
	require.Equal(t, cid, got.CardID)
	require.InDelta(t, 12.5, got.Stability, 1e-9)
	require.Equal(t, scheddom.StatusReview, got.Status)
	require.Equal(t, due, got.DueAt)
}
