package service_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/platform/eventbus"
	revdom "github.com/memorix/memorix/internal/review/domain"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/stretchr/testify/require"
)

// TestGrade_P95Under150ms đo p95 của đường chấm thật (load card+prefs → FSRS →
// append log + update card + receipt, cùng 1 TX) trên Postgres thật — NFR-1 < 150ms.
// Gate bằng Docker: dbtest.RunPostgres skip khi -short. insertNewCard/realService
// tái dùng từ grade_integration_test.go (cùng package service_test).
func TestGrade_P95Under150ms(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	owner := uuid.New()
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)

	const n = 100
	durs := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		// entry mới mỗi vòng: né unique(owner,entry,direction); không ảnh hưởng đường chấm.
		cardID := insertNewCard(t, ctx, pool, owner, uuid.New(), now)
		svc := realService(pool, eventbus.NewInProcess(), now)
		start := time.Now()
		_, err := svc.Grade(ctx, owner, revdom.GradeCommand{
			CardID: cardID, Grade: scheddom.GradeGood, ClientReviewID: uuid.NewString(),
		})
		require.NoError(t, err)
		durs = append(durs, time.Since(start))
	}

	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	p95 := durs[int(float64(n)*0.95)-1]
	t.Logf("grade p95 = %v (target < 150ms)", p95)
	require.Less(t, p95, 150*time.Millisecond, "NFR-1: grade p95 < 150ms")
}
