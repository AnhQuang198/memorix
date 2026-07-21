package domain

import (
	"math"
	"testing"
	"time"
)

func TestRetrievability(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		card Card
		now  time.Time
		want float64
		tol  float64
	}{
		{"t=S ⇒ R≈0.9 (bất biến FSRS)", Card{Stability: 10, LastReviewAt: &base}, base.AddDate(0, 0, 10), 0.90, 0.005},
		{"t=0 ⇒ R=1", Card{Stability: 10, LastReviewAt: &base}, base, 1.0, 0.0001},
		{"stability=0 ⇒ cấp thiết nhất R=0", Card{Stability: 0, LastReviewAt: &base}, base.AddDate(0, 0, 1), 0.0, 0.0001},
		{"elapsed âm bị kẹp về 0 ⇒ R=1", Card{Stability: 5, LastReviewAt: &base}, base.Add(-2 * time.Hour), 1.0, 0.0001},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Retrievability(tc.card, tc.now)
			if math.Abs(got-tc.want) > tc.tol {
				t.Errorf("Retrievability = %v, want %v (±%v)", got, tc.want, tc.tol)
			}
		})
	}
}

func TestRetrievability_MonotoneDecreasing(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	c := Card{Stability: 5, LastReviewAt: &base}
	r1 := Retrievability(c, base.AddDate(0, 0, 2))
	r2 := Retrievability(c, base.AddDate(0, 0, 20))
	if !(r2 < r1) {
		t.Errorf("R phải giảm khi càng quá hạn: r@2d=%v r@20d=%v", r1, r2)
	}
}
