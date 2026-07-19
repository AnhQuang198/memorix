package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/ports"
)

// PrefsStore = adapter pgx cho scheduling.user_scheduler_prefs
// (implements ports.PrefsStore).
type PrefsStore struct{}

// NewPrefsStore trả adapter stateless (Querier truyền theo call).
func NewPrefsStore() *PrefsStore { return &PrefsStore{} }

// compile-time port check.
var _ ports.PrefsStore = (*PrefsStore)(nil)

// Get trả prefs của user; khi chưa cấu hình → default (kèm UserID) chứ không
// tạo row (không rewrite quá khứ; đọc thuần).
func (s *PrefsStore) Get(ctx context.Context, q db.Querier, userID uuid.UUID) (domain.SchedulerPrefs, error) {
	p := domain.DefaultPrefs()
	p.UserID = userID
	row := q.QueryRow(ctx, `
		SELECT desired_retention, daily_new_limit, daily_review_limit, timezone
		FROM scheduling.user_scheduler_prefs WHERE user_id=$1`, userID)
	err := row.Scan(&p.DesiredRetention, &p.DailyNewLimit, &p.DailyReviewLimit, &p.Timezone)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, nil // p vẫn là DefaultPrefs() + UserID
	}
	if err != nil {
		return domain.SchedulerPrefs{}, err
	}
	return p, nil
}

// Upsert ghi/cập nhật prefs theo user_id.
func (s *PrefsStore) Upsert(ctx context.Context, q db.Querier, p domain.SchedulerPrefs) error {
	_, err := q.Exec(ctx, `
		INSERT INTO scheduling.user_scheduler_prefs
		  (user_id, desired_retention, daily_new_limit, daily_review_limit, timezone, updated_at)
		VALUES ($1,$2,$3,$4,$5,now())
		ON CONFLICT (user_id) DO UPDATE SET
		  desired_retention=EXCLUDED.desired_retention,
		  daily_new_limit=EXCLUDED.daily_new_limit,
		  daily_review_limit=EXCLUDED.daily_review_limit,
		  timezone=EXCLUDED.timezone,
		  updated_at=now()`,
		p.UserID, p.DesiredRetention, p.DailyNewLimit, p.DailyReviewLimit, p.Timezone)
	return err
}
