package fsrsadapter_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/repo/fsrsadapter"
	"github.com/stretchr/testify/require"
)

func newCard() domain.Card {
	due := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	return domain.Card{
		ID: uuid.New(), OwnerID: uuid.New(), EntryID: uuid.New(),
		Direction: domain.DirectionFrontBack, Status: domain.StatusNew,
		DueAt: &due,
	}
}

func TestApply_GoodOnNewCard_BuildsPositiveStabilityAndFutureDue(t *testing.T) {
	a := fsrsadapter.New()
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	r := a.Apply(newCard(), domain.GradeGood, 0.90, now)
	require.Greater(t, r.Stability, 0.0)
	require.GreaterOrEqual(t, r.Difficulty, 1.0)
	require.LessOrEqual(t, r.Difficulty, 10.0)
	require.True(t, r.DueAt.After(now), "due phải ở tương lai")
	require.Equal(t, 1, r.Reps)
	require.Equal(t, now, r.LastReviewAt)
}

func TestApply_AgainCountsLapseFromReviewCard(t *testing.T) {
	a := fsrsadapter.New()
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	// card đang ở Review, quên → lapse tăng, chuyển Relearning.
	c := newCard()
	c.Status = domain.StatusReview
	c.Stability = 10
	c.Difficulty = 5
	last := now.AddDate(0, 0, -10)
	c.LastReviewAt = &last
	r := a.Apply(c, domain.GradeAgain, 0.90, now)
	require.Equal(t, 1, r.Lapses)
	require.Equal(t, domain.StatusRelearning, r.Status)
}

func TestApply_Deterministic(t *testing.T) {
	a := fsrsadapter.New()
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	c := newCard()
	r1 := a.Apply(c, domain.GradeGood, 0.90, now)
	r2 := a.Apply(c, domain.GradeGood, 0.90, now)
	require.Equal(t, r1.Stability, r2.Stability)
	require.Equal(t, r1.DueAt, r2.DueAt, "fuzz TẮT → Due lặp lại y hệt (replay AD-4)")
}

func TestPreview_FourIntervalsOrdered(t *testing.T) {
	a := fsrsadapter.New()
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	c := newCard()
	c.Status = domain.StatusReview
	c.Stability = 10
	c.Difficulty = 5
	last := now.AddDate(0, 0, -10)
	c.LastReviewAt = &last
	iv := a.Preview(c, 0.90, now)
	// Again ≤ Hard ≤ Good ≤ Easy (ngữ nghĩa Anki).
	require.LessOrEqual(t, iv.Again, iv.Hard)
	require.LessOrEqual(t, iv.Hard, iv.Good)
	require.LessOrEqual(t, iv.Good, iv.Easy)
}
