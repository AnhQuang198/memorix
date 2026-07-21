package domain

import (
	"testing"
	"time"
)

func TestStartOfStudyDay_UserTZBoundary(t *testing.T) {
	// 02:00Z = 09:00 giờ VN cùng ngày ⇒ đầu ngày học = 2026-07-07 00:00+07 = 2026-07-06 17:00Z.
	now := time.Date(2026, 7, 7, 2, 0, 0, 0, time.UTC)
	got, err := StartOfStudyDay(now, "Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := time.Date(2026, 7, 6, 17, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("StartOfStudyDay = %v, want %v", got.UTC(), want)
	}
}

func TestStartOfStudyDay_LocalMidnightDSTZone(t *testing.T) {
	// TZ có DST: kết quả luôn là 00:00 giờ địa phương (không phải 00:00Z).
	now := time.Date(2026, 3, 8, 18, 0, 0, 0, time.UTC)
	got, err := StartOfStudyDay(now, "America/New_York")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Hour() != 0 || got.Minute() != 0 {
		t.Errorf("phải là nửa đêm giờ địa phương, got %v", got)
	}
}

func TestStartOfStudyDay_InvalidTZ(t *testing.T) {
	if _, err := StartOfStudyDay(time.Now(), "Not/AZone"); err == nil {
		t.Error("expected error cho TZ không hợp lệ")
	}
}
