package domain

// StudyProfile — trạng thái động lực tích lũy của một user.
type StudyProfile struct {
	StreakCurrent int
	StreakBest    int
	LastStudyDate *Day // nil = chưa có ngày recall thật
	TotalReviews  int
	TotalRetained int
}

// ApplyStudyDay áp một "sự kiện học" trong ngày `today` vào profile.
//   - totals LUÔN cộng dồn (reviewsDelta/retainedDelta) — total_retained KHÔNG bao giờ reset (FR-32).
//   - streak/last_study_date CHỈ đổi khi có recall thật trong sự kiện này (retainedDelta > 0).
func ApplyStudyDay(p StudyProfile, today Day, reviewsDelta, retainedDelta int) StudyProfile {
	p.TotalReviews += reviewsDelta
	p.TotalRetained += retainedDelta

	if retainedDelta <= 0 {
		return p // ngày không có recall thật → không tính streak
	}

	switch {
	case p.LastStudyDate == nil:
		p.StreakCurrent = 1
	case *p.LastStudyDate == today:
		// đã tính streak cho hôm nay rồi
	case DaysBetween(*p.LastStudyDate, today) == 1:
		p.StreakCurrent++
	default:
		p.StreakCurrent = 1 // lỡ ≥1 ngày → reset streak (nhưng total_retained giữ nguyên)
	}
	if p.StreakCurrent > p.StreakBest {
		p.StreakBest = p.StreakCurrent
	}
	d := today
	p.LastStudyDate = &d
	return p
}

// RebuildStudyProfile dựng lại profile từ chuỗi daily stats (đã sort tăng theo Day).
// Nguồn chân lý = log → daily_stats → fold. Dùng bởi reconcile (AD-4).
func RebuildStudyProfile(stats []DailyStat) StudyProfile {
	var p StudyProfile
	for _, s := range stats {
		p = ApplyStudyDay(p, s.Day, s.ReviewsDone, s.Retained)
	}
	return p
}
