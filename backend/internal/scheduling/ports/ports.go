package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/scheduling/domain"
)

// CardRepo — driven adapter (Postgres) cho thẻ scheduling.
type CardRepo interface {
	// LoadCandidates trả thẻ New + thẻ due (due_at<=now) của user, dùng index cards(owner_id,due_at).
	LoadCandidates(ctx context.Context, ownerID uuid.UUID, now time.Time) ([]domain.Card, error)
	// BulkDefer đẩy due_at các thẻ (anti-flood) trong 1 batch.
	BulkDefer(ctx context.Context, deferred []domain.DeferredCard) error
}

// PrefsRepo — user_scheduler_prefs (đọc + đổi giới hạn ngày, Story 4.2).
type PrefsRepo interface {
	Get(ctx context.Context, userID uuid.UUID) (domain.SchedulerPrefs, error)
	UpdateLimits(ctx context.Context, userID uuid.UUID, newLimit, reviewLimit int) (domain.SchedulerPrefs, error)
}

// ReviewActivityPort — cross-module (AD-9): scheduling hỏi review đã phục vụ bao nhiêu
// thẻ new/review kể từ đầu ngày học. Review module implement, wire ở cmd/api.
type ReviewActivityPort interface {
	CountServedSince(ctx context.Context, userID uuid.UUID, since time.Time) (domain.DayCounts, error)
}

// StudyProfileRepo — cờ "đã xem coach lần đầu" (Story 4.5).
type StudyProfileRepo interface {
	CoachSeen(ctx context.Context, userID uuid.UUID) (bool, error)
	MarkCoachSeen(ctx context.Context, userID uuid.UUID) error
}
