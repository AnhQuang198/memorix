package service

import (
	"context"
	"log/slog"

	"github.com/memorix/memorix/internal/platform/eventbus"
	"github.com/memorix/memorix/internal/progress/domain"
	"github.com/memorix/memorix/internal/shared/events"
)

// Ingestor cập nhật read model khi có CardGraded (fire-and-forget, ngoài TX grade — AD-8).
type Ingestor struct {
	repo IngestRepo
	tz   TZResolver
	log  *slog.Logger
}

func NewIngestor(repo IngestRepo, tz TZResolver, log *slog.Logger) *Ingestor {
	if log == nil {
		log = slog.Default()
	}
	return &Ingestor{repo: repo, tz: tz, log: log}
}

// HandleCardGraded áp một event vào daily_stats + study_profiles.
func (i *Ingestor) HandleCardGraded(ctx context.Context, e events.CardGraded) error {
	loc := i.tz.Location(ctx, e.OwnerID)
	day := domain.DayOf(e.ReviewedAt, loc)
	retained := domain.IsRetained(e.Grade, e.ScheduledDays)

	if err := i.repo.BumpDailyStat(ctx, e.OwnerID, day, e.WasNew, e.Grade, retained); err != nil {
		return err
	}
	prof, _, err := i.repo.GetStudyProfile(ctx, e.OwnerID)
	if err != nil {
		return err
	}
	retainedDelta := 0
	if retained {
		retainedDelta = 1
	}
	prof = domain.ApplyStudyDay(prof, day, 1, retainedDelta)
	return i.repo.UpsertStudyProfile(ctx, e.OwnerID, prof)
}

// Subscribe gắn Ingestor vào bus. Bus in-process phát async (Sprint 0) → fire-and-forget.
// Lỗi read model KHÔNG làm hỏng grade: chỉ log (AD-8).
func (i *Ingestor) Subscribe(bus eventbus.Bus) {
	bus.Subscribe(events.CardGradedName, func(ctx context.Context, ev eventbus.Event) {
		e, ok := ev.Payload.(events.CardGraded)
		if !ok {
			i.log.Warn("progress: bỏ qua payload CardGraded sai kiểu")
			return
		}
		if err := i.HandleCardGraded(ctx, e); err != nil {
			i.log.Error("progress: cập nhật read model thất bại (sẽ do reconcile chữa)",
				slog.String("owner_id", e.OwnerID), slog.Any("err", err))
		}
	})
}
