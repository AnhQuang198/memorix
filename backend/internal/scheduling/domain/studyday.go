package domain

import "time"

// StudyDayStart trả mốc bắt đầu "ngày học" mà `now` thuộc về, theo TZ user, có
// grace tới graceHour giờ sáng (AD-12): trước graceHour tính là ngày hôm trước.
func StudyDayStart(now time.Time, loc *time.Location, graceHour int) time.Time {
	local := now.In(loc)
	d := local
	if local.Hour() < graceHour {
		d = local.AddDate(0, 0, -1)
	}
	return time.Date(d.Year(), d.Month(), d.Day(), graceHour, 0, 0, 0, loc)
}
