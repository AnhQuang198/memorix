// Package repo là adapter Postgres cho read model Progress (sqlc/pgx). Đọc review_logs
// (nguồn chân lý, AD-4/AD-8) và cards.due_at (forecast) — read-only cho read model;
// ghi progress.daily_stats + progress.study_profiles.
package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/progress/domain"
	"github.com/memorix/memorix/internal/progress/repo/gen"
	"github.com/memorix/memorix/internal/progress/service"
)

// Repo bọc *gen.Queries, convert giữa domain và kiểu sqlc/pgx.
type Repo struct {
	q *gen.Queries
}

// New tạo Repo trên một pgxpool.Pool.
func New(pool *pgxpool.Pool) *Repo { return &Repo{q: gen.New(pool)} }

// Repo thỏa port write side service.IngestRepo (kiểm tra tại compile-time).
var _ service.IngestRepo = (*Repo)(nil)

func toPgDate(d domain.Day) pgtype.Date { return pgtype.Date{Time: d.At(), Valid: true} }

func toPgDatePtr(d *domain.Day) pgtype.Date {
	if d == nil {
		return pgtype.Date{}
	}
	return toPgDate(*d)
}

func fromPgDate(p pgtype.Date) domain.Day {
	t := p.Time
	return domain.Day{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}
}

// ---- write side (Task 6) ----

// BumpDailyStat cộng 1 review (và các counter phái sinh) vào hàng (user, day),
// tạo hàng nếu chưa có (upsert ON CONFLICT).
func (r *Repo) BumpDailyStat(ctx context.Context, ownerID string, day domain.Day, wasNew bool, grade int, retained bool) error {
	b2i := func(b bool) int32 {
		if b {
			return 1
		}
		return 0
	}
	return r.q.BumpDailyStat(ctx, gen.BumpDailyStatParams{
		UserID:   ownerID,
		Day:      toPgDate(day),
		NewDone:  b2i(wasNew),
		Retained: b2i(retained),
		Again:    b2i(grade == domain.GradeAgain),
		Hard:     b2i(grade == domain.GradeHard),
		Good:     b2i(grade == domain.GradeGood),
		Easy:     b2i(grade == domain.GradeEasy),
	})
}

// GetStudyProfile trả profile + found=false khi user chưa có hàng.
func (r *Repo) GetStudyProfile(ctx context.Context, userID string) (domain.StudyProfile, bool, error) {
	row, err := r.q.GetStudyProfile(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.StudyProfile{}, false, nil
	}
	if err != nil {
		return domain.StudyProfile{}, false, err
	}
	p := domain.StudyProfile{
		StreakCurrent: int(row.StreakCurrent),
		StreakBest:    int(row.StreakBest),
		TotalReviews:  int(row.TotalReviews),
		TotalRetained: int(row.TotalRetained),
	}
	if row.LastStudyDate.Valid {
		d := fromPgDate(row.LastStudyDate)
		p.LastStudyDate = &d
	}
	return p, true, nil
}

// UpsertStudyProfile ghi (insert hoặc ON CONFLICT update) profile động lực.
func (r *Repo) UpsertStudyProfile(ctx context.Context, userID string, p domain.StudyProfile) error {
	return r.q.UpsertStudyProfile(ctx, gen.UpsertStudyProfileParams{
		UserID:        userID,
		StreakCurrent: int32(p.StreakCurrent),
		StreakBest:    int32(p.StreakBest),
		LastStudyDate: toPgDatePtr(p.LastStudyDate),
		TotalReviews:  int32(p.TotalReviews),
		TotalRetained: int32(p.TotalRetained),
	})
}

// ---- rebuild side (dùng bởi reconcile, Task 9) ----

// DistinctOwners liệt kê mọi owner từng có review_log (để reconcile lặp qua).
func (r *Repo) DistinctOwners(ctx context.Context) ([]string, error) {
	return r.q.DistinctOwners(ctx)
}

// AllLogsForOwner trả toàn bộ review_logs của owner (sort tăng theo reviewed_at) để
// rebuild daily_stats. scheduled_days suy từ (new_due_at - reviewed_at) theo ngày.
func (r *Repo) AllLogsForOwner(ctx context.Context, ownerID string) ([]domain.LogRow, error) {
	rows, err := r.q.AllLogsForOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.LogRow, len(rows))
	for i, row := range rows {
		out[i] = domain.LogRow{
			CardID:        row.CardID,
			Grade:         int(row.Grade),
			ScheduledDays: int(row.ScheduledDays),
			ReviewedAt:    row.ReviewedAt,
		}
	}
	return out, nil
}

// ReplaceDailyStats thay toàn bộ daily_stats của owner bằng slice mới (delete + insert).
func (r *Repo) ReplaceDailyStats(ctx context.Context, ownerID string, stats []domain.DailyStat) error {
	if err := r.q.DeleteDailyStats(ctx, ownerID); err != nil {
		return err
	}
	for _, s := range stats {
		err := r.q.InsertDailyStat(ctx, gen.InsertDailyStatParams{
			UserID:      ownerID,
			Day:         toPgDate(s.Day),
			ReviewsDone: int32(s.ReviewsDone),
			NewDone:     int32(s.NewDone),
			Retained:    int32(s.Retained),
			Again:       int32(s.Again),
			Hard:        int32(s.Hard),
			Good:        int32(s.Good),
			Easy:        int32(s.Easy),
		})
		if err != nil {
			return err
		}
	}
	return nil
}
