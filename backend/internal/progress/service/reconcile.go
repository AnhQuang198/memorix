package service

import (
	"context"

	"github.com/memorix/memorix/internal/progress/domain"
)

// ReconcileRepo — rebuild read model từ nguồn chân lý review_logs (AD-4).
type ReconcileRepo interface {
	DistinctOwners(ctx context.Context) ([]string, error)
	AllLogsForOwner(ctx context.Context, ownerID string) ([]domain.LogRow, error)
	ReplaceDailyStats(ctx context.Context, ownerID string, stats []domain.DailyStat) error
	UpsertStudyProfile(ctx context.Context, userID string, p domain.StudyProfile) error
}

// Reconciler chạy định kỳ (River) chữa drift daily_stats.
type Reconciler struct {
	repo ReconcileRepo
	tz   TZResolver
}

// NewReconciler tạo Reconciler trên một ReconcileRepo + TZResolver.
func NewReconciler(repo ReconcileRepo, tz TZResolver) *Reconciler {
	return &Reconciler{repo: repo, tz: tz}
}

// ReconcileAll rebuild daily_stats + study_profiles cho mọi user.
func (r *Reconciler) ReconcileAll(ctx context.Context) error {
	owners, err := r.repo.DistinctOwners(ctx)
	if err != nil {
		return err
	}
	for _, o := range owners {
		logs, err := r.repo.AllLogsForOwner(ctx, o)
		if err != nil {
			return err
		}
		loc := r.tz.Location(ctx, o)
		stats := domain.RebuildDailyStats(logs, loc)
		if err := r.repo.ReplaceDailyStats(ctx, o, stats); err != nil {
			return err
		}
		prof := domain.RebuildStudyProfile(stats)
		if prof.LastStudyDate == nil {
			continue // không có ngày recall thật → không ghi profile
		}
		if err := r.repo.UpsertStudyProfile(ctx, o, prof); err != nil {
			return err
		}
	}
	return nil
}
