// Package jobs bọc River (client + migrate). River tables migrate riêng khỏi
// golang-migrate để River tự quản version schema của nó (ARCH-12).
package jobs

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// Migrate áp schema River lên DB.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	m, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return err
	}
	_, err = m.Migrate(ctx, rivermigrate.DirectionUp, nil)
	return err
}

// NewClient tạo River client (insert-only ở API; worker truyền Workers + periodic jobs
// riêng). periodicJobs tùy chọn: worker đăng ký reconcile định kỳ (Task 9), API bỏ trống.
func NewClient(pool *pgxpool.Pool, workers *river.Workers, periodicJobs ...*river.PeriodicJob) (*river.Client[pgx.Tx], error) {
	cfg := &river.Config{}
	if workers != nil {
		cfg.Queues = map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 10}}
		cfg.Workers = workers
	}
	if len(periodicJobs) > 0 {
		cfg.PeriodicJobs = periodicJobs
	}
	return river.NewClient(riverpgxv5.New(pool), cfg)
}
