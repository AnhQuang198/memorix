package domain

import (
	"fmt"
	"time"
)

// Day là ngày lịch (civil date) — không giờ, không TZ. Tránh lệch DST khi đếm streak.
type Day struct {
	Year  int
	Month int
	Day   int
}

// DayOf quy đổi thời điểm sang "ngày học" theo TZ user (AD-12).
func DayOf(t time.Time, loc *time.Location) Day {
	t = t.In(loc)
	return Day{t.Year(), int(t.Month()), t.Day()}
}

// At neo Day về nửa đêm UTC (mốc so ngày ổn định, không DST).
func (d Day) At() time.Time {
	return time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC)
}

// DaysBetween = số ngày nguyên từ a tới b (b sau a → dương).
func DaysBetween(a, b Day) int {
	return int(b.At().Sub(a.At()).Hours()) / 24
}

func (d Day) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}
