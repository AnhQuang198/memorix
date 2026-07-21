package domain

// Mức chấm FSRS.
const (
	GradeAgain = 1
	GradeHard  = 2
	GradeGood  = 3
	GradeEasy  = 4
)

// MinRetainInterval — North Star N=7 ngày (PRD OQ-3).
const MinRetainInterval = 7

// IsRecalled: nhớ được (không Again).
func IsRecalled(grade int) bool { return grade >= GradeHard }

// IsRetained: đủ điều kiện North Star — recall đúng VÀ lịch kế ≥7 ngày.
func IsRetained(grade, scheduledDays int) bool {
	return IsRecalled(grade) && scheduledDays >= MinRetainInterval
}
