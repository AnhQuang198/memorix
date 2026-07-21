package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/ports"
)

// QueueRepo = adapter pgx cho smart-queue (Sprint 4). Hiện thực ports.CardRepo,
// ports.PrefsRepo và ports.StudyProfileRepo bằng pool trực tiếp (đọc/ghi ngoài TX
// chấm điểm — queue là read path + đổi limit + cờ coach). Tách khỏi CardStore/
// PrefsStore (Querier-based, tham gia TX chấm điểm) để không đụng chữ ký cũ.
type QueueRepo struct{ pool *pgxpool.Pool }

// NewQueueRepo tạo QueueRepo gắn pool.
func NewQueueRepo(pool *pgxpool.Pool) *QueueRepo { return &QueueRepo{pool: pool} }

// compile-time port checks (Task 7).
var (
	_ ports.CardRepo         = (*QueueRepo)(nil)
	_ ports.PrefsRepo        = (*QueueRepo)(nil)
	_ ports.StudyProfileRepo = (*QueueRepo)(nil)
)

// LoadCandidates nạp thẻ New + thẻ đến hạn (due_at<=now) của owner, bỏ suspended
// và deleted; dùng index cards(owner_id,due_at) (NFR-2). BuildQueue lo sắp xếp/hạn
// ngày ở domain, nên đây chỉ lọc ứng viên. NULLS LAST để thẻ new (due NULL) xuống cuối.
func (r *QueueRepo) LoadCandidates(ctx context.Context, ownerID uuid.UUID, now time.Time) ([]domain.Card, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+cardCols+`
		FROM scheduling.cards
		WHERE owner_id = $1 AND deleted_at IS NULL
		  AND status <> 'suspended'
		  AND (status = 'new' OR due_at <= $2)
		ORDER BY owner_id, due_at ASC NULLS LAST`, ownerID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// BulkDefer đẩy due_at nhiều thẻ overdue sang ngày mới (anti-flood) trong 1 batch.
func (r *QueueRepo) BulkDefer(ctx context.Context, deferred []domain.DeferredCard) error {
	if len(deferred) == 0 {
		return nil
	}
	b := &pgx.Batch{}
	for _, d := range deferred {
		b.Queue(`UPDATE scheduling.cards SET due_at = $2, updated_at = now() WHERE id = $1`,
			d.CardID, d.NewDueAt)
	}
	br := r.pool.SendBatch(ctx, b)
	defer func() { _ = br.Close() }()
	for range deferred {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

// Get đọc prefs của user (không rewrite quá khứ). ErrNoRows ⇒ default kèm UserID.
func (r *QueueRepo) Get(ctx context.Context, userID uuid.UUID) (domain.SchedulerPrefs, error) {
	p := domain.DefaultPrefs()
	p.UserID = userID
	err := r.pool.QueryRow(ctx, `
		SELECT desired_retention, daily_new_limit, daily_review_limit, timezone
		FROM scheduling.user_scheduler_prefs WHERE user_id = $1`, userID).
		Scan(&p.DesiredRetention, &p.DailyNewLimit, &p.DailyReviewLimit, &p.Timezone)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return domain.SchedulerPrefs{}, err
	}
	return p, nil
}

// UpdateLimits đổi hạn new/review ngày (Story 4.2). Upsert để không phụ thuộc row
// prefs đã tồn tại; desired_retention/timezone giữ default hoặc giá trị cũ.
func (r *QueueRepo) UpdateLimits(ctx context.Context, userID uuid.UUID, newLimit, reviewLimit int) (domain.SchedulerPrefs, error) {
	var p domain.SchedulerPrefs
	err := r.pool.QueryRow(ctx, `
		INSERT INTO scheduling.user_scheduler_prefs (user_id, daily_new_limit, daily_review_limit)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		  SET daily_new_limit = EXCLUDED.daily_new_limit,
		      daily_review_limit = EXCLUDED.daily_review_limit,
		      updated_at = now()
		RETURNING user_id, desired_retention, daily_new_limit, daily_review_limit, timezone`,
		userID, newLimit, reviewLimit).
		Scan(&p.UserID, &p.DesiredRetention, &p.DailyNewLimit, &p.DailyReviewLimit, &p.Timezone)
	if err != nil {
		return domain.SchedulerPrefs{}, err
	}
	return p, nil
}

// CoachSeen báo user đã xem hướng dẫn chấm điểm lần đầu chưa (Story 4.5).
func (r *QueueRepo) CoachSeen(ctx context.Context, userID uuid.UUID) (bool, error) {
	var seenAt *time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT learn_coach_seen_at FROM scheduling.study_profiles WHERE user_id = $1`, userID).
		Scan(&seenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return seenAt != nil, nil
}

// MarkCoachSeen đặt cờ đã-xem (upsert idempotent).
func (r *QueueRepo) MarkCoachSeen(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO scheduling.study_profiles (user_id, learn_coach_seen_at)
		VALUES ($1, now())
		ON CONFLICT (user_id) DO UPDATE
		  SET learn_coach_seen_at = now(), updated_at = now()`, userID)
	return err
}
