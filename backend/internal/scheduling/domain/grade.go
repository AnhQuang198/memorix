package domain

// Grade map 1-1 với go-fsrs Rating (Again=1, Hard=2, Good=3, Easy=4). Adapter dựa vào.
type Grade int16

const (
	GradeAgain Grade = iota + 1
	GradeHard
	GradeGood
	GradeEasy
)

// Valid báo Grade nằm trong khoảng hợp lệ Again..Easy.
func (g Grade) Valid() bool { return g >= GradeAgain && g <= GradeEasy }
