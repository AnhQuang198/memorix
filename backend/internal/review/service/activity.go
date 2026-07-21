package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	schedports "github.com/memorix/memorix/internal/scheduling/ports"
)

// ActivityAdapter hiện thực scheduling ports.ReviewActivityPort đọc từ
// review.review_logs (AD-9: PORT ở scheduling/ports phía consumer, ADAPTER đọc
// review_logs vì review sở hữu bảng này). Đếm "new served" = log có prev_status=0
// (thẻ trước khi chấm còn New), "review served" = phần còn lại. Nguồn chân lý =
// review_logs (AD-4). Wire glue ở cmd/api.
type ActivityAdapter struct{ pool *pgxpool.Pool }

// NewActivityAdapter tạo adapter gắn pool.
func NewActivityAdapter(pool *pgxpool.Pool) *ActivityAdapter { return &ActivityAdapter{pool: pool} }

// compile-time port check (cross-module, AD-9).
var _ schedports.ReviewActivityPort = (*ActivityAdapter)(nil)

// CountServedSince đếm số thẻ new/review đã phục vụ user từ mốc since (đầu ngày học
// theo TZ user, tính ở caller). userID map sang cột owner_id của review_logs.
func (a *ActivityAdapter) CountServedSince(ctx context.Context, userID uuid.UUID, since time.Time) (scheddom.DayCounts, error) {
	var d scheddom.DayCounts
	err := a.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE prev_status = 0),
		  COUNT(*) FILTER (WHERE prev_status <> 0)
		FROM review.review_logs
		WHERE owner_id = $1 AND reviewed_at >= $2`, userID, since).
		Scan(&d.NewServed, &d.ReviewServed)
	if err != nil {
		return scheddom.DayCounts{}, err
	}
	return d, nil
}
