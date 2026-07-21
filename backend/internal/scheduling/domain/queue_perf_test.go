package domain

import (
	"math/rand"
	"sort"
	"testing"
	"time"
)

// gen10k dựng 10k Card theo REAL type (Status CardStatus, DueAt/LastReviewAt *time.Time),
// phân bố due -15..+4 ngày quanh now để phủ overdue/đến-hạn/tương-lai. Seed cố định ⇒ tái lập.
func gen10k(now time.Time) []Card {
	r := rand.New(rand.NewSource(42)) //nolint:gosec // deterministic fixture, not security-sensitive
	cards := make([]Card, 0, 10000)
	statuses := []CardStatus{StatusNew, StatusLearning, StatusReview, StatusRelearning}
	for i := 0; i < 10000; i++ {
		st := statuses[r.Intn(len(statuses))]
		dueOffset := time.Duration(r.Intn(20)-15) * 24 * time.Hour // -15..+4 ngày
		due := now.Add(dueOffset)
		last := due.Add(-48 * time.Hour)
		cards = append(cards, Card{
			Status:       st,
			Stability:    1 + r.Float64()*200,
			DueAt:        &due,
			LastReviewAt: &last,
			CreatedAt:    now.Add(-time.Duration(r.Intn(1000)) * time.Hour),
		})
	}
	return cards
}

// TestBuildQueue_Performance10k chứng minh NFR-2: BuildQueue xử lý 10k thẻ với p95 < 500ms
// (thuần CPU, in-memory — không DB). Đo 30 lần rồi lấy p95.
func TestBuildQueue_Performance10k(t *testing.T) {
	if testing.Short() {
		t.Skip("skip perf test in -short mode")
	}
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	cards := gen10k(now)

	const iters = 30
	durs := make([]time.Duration, iters)
	for i := 0; i < iters; i++ {
		start := time.Now()
		_ = BuildQueue(cards, prefs, now)
		durs[i] = time.Since(start)
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	p95 := durs[iters*95/100-1] // p95 index (ceil của phân vị 95, 0-based)
	if p95 > 500*time.Millisecond {
		t.Errorf("NFR-2 vi phạm: p95 BuildQueue(10k) = %v, want <500ms", p95)
	}
	t.Logf("BuildQueue(10k) p95 = %v", p95)
}

func BenchmarkBuildQueue10k(b *testing.B) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	cards := gen10k(now)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildQueue(cards, prefs, now)
	}
}
