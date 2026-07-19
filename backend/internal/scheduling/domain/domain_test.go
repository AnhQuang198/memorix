package domain_test

import (
	"testing"
	"time"

	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/stretchr/testify/require"
)

func TestGrade_Valid(t *testing.T) {
	require.True(t, domain.GradeAgain.Valid())
	require.True(t, domain.GradeEasy.Valid())
	require.False(t, domain.Grade(0).Valid())
	require.False(t, domain.Grade(5).Valid())
}

func TestGrade_Values(t *testing.T) {
	// Grade map 1-1 với go-fsrs Rating (Again=1..Easy=4) — adapter dựa vào.
	require.EqualValues(t, 1, domain.GradeAgain)
	require.EqualValues(t, 2, domain.GradeHard)
	require.EqualValues(t, 3, domain.GradeGood)
	require.EqualValues(t, 4, domain.GradeEasy)
}

func TestStudyDayStart_GraceBeforeDawn(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Bangkok") // UTC+7
	require.NoError(t, err)
	// 02:00 giờ Bangkok, grace 4h → vẫn thuộc "ngày học" hôm trước.
	now := time.Date(2026, 7, 18, 2, 0, 0, 0, loc)
	start := domain.StudyDayStart(now, loc, 4)
	require.Equal(t, 2026, start.Year())
	require.Equal(t, time.July, start.Month())
	require.Equal(t, 17, start.Day(), "trước 4h sáng tính là ngày hôm trước")
	require.Equal(t, 4, start.Hour())
}

func TestStudyDayStart_AfterDawn(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 7, 18, 9, 0, 0, 0, loc)
	start := domain.StudyDayStart(now, loc, 4)
	require.Equal(t, 18, start.Day())
}

func TestPrefs_Default(t *testing.T) {
	p := domain.DefaultPrefs()
	require.InDelta(t, 0.90, p.DesiredRetention, 1e-9)
	require.Equal(t, "UTC", p.Timezone)
}

func TestRetentionInRange(t *testing.T) {
	require.True(t, domain.RetentionInRange(0.80))
	require.True(t, domain.RetentionInRange(0.90))
	require.True(t, domain.RetentionInRange(0.97))
	require.False(t, domain.RetentionInRange(0.79))
	require.False(t, domain.RetentionInRange(0.98))
}
