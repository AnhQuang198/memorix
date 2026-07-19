package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/ports"
)

var (
	ErrRetentionRange = errors.New("desired_retention must be within [0.80, 0.97]")
	ErrBadTimezone    = errors.New("invalid IANA timezone")
)

// PrefsUpdate = input cập nhật cấu hình lịch (FR-17, FR-26).
type PrefsUpdate struct {
	DesiredRetention float64
	DailyNewLimit    int
	DailyReviewLimit int
	Timezone         string
}

// PrefsService quản cấu hình lịch. pool có thể nil trong unit test (fake store bỏ qua Querier).
type PrefsService struct {
	pool  *pgxpool.Pool
	store ports.PrefsStore
}

func NewPrefsService(pool *pgxpool.Pool, store ports.PrefsStore) *PrefsService {
	return &PrefsService{pool: pool, store: store}
}

func (s *PrefsService) Get(ctx context.Context, userID uuid.UUID) (domain.SchedulerPrefs, error) {
	return s.store.Get(ctx, poolOrNil(s.pool), userID)
}

func (s *PrefsService) Update(ctx context.Context, userID uuid.UUID, in PrefsUpdate) (domain.SchedulerPrefs, error) {
	if !domain.RetentionInRange(in.DesiredRetention) {
		return domain.SchedulerPrefs{}, ErrRetentionRange
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return domain.SchedulerPrefs{}, ErrBadTimezone
	}
	p := domain.SchedulerPrefs{
		UserID:           userID,
		DesiredRetention: in.DesiredRetention,
		DailyNewLimit:    in.DailyNewLimit,
		DailyReviewLimit: in.DailyReviewLimit,
		Timezone:         in.Timezone,
	}
	if err := s.store.Upsert(ctx, poolOrNil(s.pool), p); err != nil {
		return domain.SchedulerPrefs{}, err
	}
	return p, nil
}
