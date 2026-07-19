package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/scheduling/domain"
)

// CardStore = driven adapter cho scheduling.cards. Nhận Querier để tham gia TX
// chấm span nhiều schema (AD-3). ErrCardNotFound khi không thuộc owner (AD-8 deny).
type CardStore interface {
	Load(ctx context.Context, q db.Querier, cardID, ownerID uuid.UUID) (domain.Card, error)
	ApplyResult(ctx context.Context, q db.Querier, cardID uuid.UUID, r domain.ScheduleResult) error
	DueCards(ctx context.Context, q db.Querier, ownerID uuid.UUID, now time.Time, limit int) ([]domain.Card, error)
}

// PrefsStore = driven adapter cho scheduling.user_scheduler_prefs.
type PrefsStore interface {
	Get(ctx context.Context, q db.Querier, userID uuid.UUID) (domain.SchedulerPrefs, error)
	Upsert(ctx context.Context, q db.Querier, p domain.SchedulerPrefs) error
}
