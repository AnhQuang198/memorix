// Package worker gắn reconcile vào River (job runner Postgres-backed, ARCH-12).
package worker

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/memorix/memorix/internal/progress/service"
)

// ReconcileArgs là job rebuild daily_stats định kỳ.
type ReconcileArgs struct{}

// Kind định danh job cho River.
func (ReconcileArgs) Kind() string { return "progress_reconcile" }

// ReconcileWorker chạy Reconciler.ReconcileAll.
type ReconcileWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	Reconciler *service.Reconciler
}

// Work rebuild daily_stats + study_profiles từ review_logs (AD-4).
func (w *ReconcileWorker) Work(ctx context.Context, _ *river.Job[ReconcileArgs]) error {
	return w.Reconciler.ReconcileAll(ctx)
}

// PeriodicSpec trả periodic job chạy mỗi giờ (chữa drift AD-8), RunOnStart để chữa
// drift ngay khi worker khởi động.
func PeriodicSpec() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return ReconcileArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}
