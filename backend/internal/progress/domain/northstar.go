package domain

// RetentionLog là dòng review_logs tối thiểu để tính North Star.
type RetentionLog struct {
	CardID        string
	Grade         int
	ScheduledDays int
}

// CountWordsRetained = số THẺ (distinct) đạt điều kiện retained trong tập log.
// Đọc thẳng từ review_logs của tuần hiện tại (AD-8) — không dùng daily_stats lag.
func CountWordsRetained(logs []RetentionLog) int {
	seen := make(map[string]struct{})
	for _, l := range logs {
		if IsRetained(l.Grade, l.ScheduledDays) {
			seen[l.CardID] = struct{}{}
		}
	}
	return len(seen)
}
