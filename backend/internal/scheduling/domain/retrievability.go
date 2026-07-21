package domain

import (
	"math"
	"time"
)

// Tham số đường quên FSRS-4.5/5: R(t) = (1 + FACTOR * t/S)^DECAY.
const (
	fsrsFactor = 19.0 / 81.0
	fsrsDecay  = -0.5
)

// Retrievability trả xác suất nhớ lại R∈(0,1] tại now, với t = số ngày kể từ lần
// ôn gần nhất, S = stability. R thấp = càng sắp quên = càng cấp thiết. Thuần, dùng
// để XẾP ƯU TIÊN queue; toán reschedule vẫn qua SchedulerPort (AD-7), không ở đây.
func Retrievability(c Card, now time.Time) float64 {
	if c.Stability <= 0 || c.LastReviewAt == nil {
		return 0 // chưa có stability/chưa ôn ⇒ coi cấp thiết nhất
	}
	elapsedDays := now.Sub(*c.LastReviewAt).Hours() / 24.0
	if elapsedDays < 0 {
		elapsedDays = 0
	}
	return math.Pow(1+fsrsFactor*elapsedDays/c.Stability, fsrsDecay)
}
