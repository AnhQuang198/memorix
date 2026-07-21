package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/memorix/memorix/internal/progress/domain"
	"github.com/memorix/memorix/internal/progress/repo/gen"
)

// WeekRetentionLogs đọc THẲNG review_logs cho North Star tức thì (AD-8) — không qua
// daily_stats (tránh lag). scheduled_days suy từ (new_due_at - reviewed_at) trong gen.
// reviewed_at là timestamptz NOT NULL nên params dùng time.Time trực tiếp.
func (r *Repo) WeekRetentionLogs(ctx context.Context, ownerID string, from, to time.Time) ([]domain.RetentionLog, error) {
	rows, err := r.q.WeekRetentionLogs(ctx, gen.WeekRetentionLogsParams{OwnerID: ownerID, FromTs: from, ToTs: to})
	if err != nil {
		return nil, err
	}
	out := make([]domain.RetentionLog, len(rows))
	for i, row := range rows {
		out[i] = domain.RetentionLog{CardID: row.CardID, Grade: int(row.Grade), ScheduledDays: int(row.ScheduledDays)}
	}
	return out, nil
}

// DueCount đếm card đến hạn (due_at <= now, chưa xóa, không suspended). cards.due_at
// nullable nên param Now sinh ra pgtype.Timestamptz — phải bọc {Time, Valid:true}.
func (r *Repo) DueCount(ctx context.Context, ownerID string, now time.Time) (int, error) {
	n, err := r.q.DueCount(ctx, gen.DueCountParams{
		OwnerID: ownerID,
		Now:     pgtype.Timestamptz{Time: now, Valid: true},
	})
	return int(n), err
}

// Forecast trả map "YYYY-MM-DD" → số card due (theo TZ user) trong [from, to).
// FromTs/ToTs sinh ra pgtype.Timestamptz (due_at nullable) — bọc {Time, Valid:true}.
func (r *Repo) Forecast(ctx context.Context, ownerID string, from, to time.Time, tz string) (map[string]int, error) {
	rows, err := r.q.ForecastDue(ctx, gen.ForecastDueParams{
		OwnerID: ownerID,
		Tz:      tz,
		FromTs:  pgtype.Timestamptz{Time: from, Valid: true},
		ToTs:    pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	m := make(map[string]int, len(rows))
	for _, row := range rows {
		m[fromPgDate(row.Day).String()] = int(row.Due)
	}
	return m, nil
}

// TodayStat trả (reviews_done, new_done) của ngày; user chưa học ngày đó → 0/0.
func (r *Repo) TodayStat(ctx context.Context, userID string, day domain.Day) (reviews, newDone int, err error) {
	row, err := r.q.TodayStat(ctx, gen.TodayStatParams{UserID: userID, Day: toPgDate(day)})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	return int(row.ReviewsDone), int(row.NewDone), nil
}

// Heatmap trả các hàng daily_stats trong [from, to] (sort tăng theo ngày) cho biểu đồ nhiệt.
func (r *Repo) Heatmap(ctx context.Context, userID string, from, to domain.Day) ([]domain.DailyStat, error) {
	rows, err := r.q.HeatmapRange(ctx, gen.HeatmapRangeParams{UserID: userID, FromDay: toPgDate(from), ToDay: toPgDate(to)})
	if err != nil {
		return nil, err
	}
	out := make([]domain.DailyStat, len(rows))
	for i, row := range rows {
		out[i] = domain.DailyStat{Day: fromPgDate(row.Day), ReviewsDone: int(row.ReviewsDone), Retained: int(row.Retained)}
	}
	return out, nil
}

// Distribution tổng phân bố mức chấm (again/hard/good/easy) trong khoảng [from, to].
func (r *Repo) Distribution(ctx context.Context, userID string, from, to domain.Day) (again, hard, good, easy int, err error) {
	row, err := r.q.GradeDistribution(ctx, gen.GradeDistributionParams{UserID: userID, FromDay: toPgDate(from), ToDay: toPgDate(to)})
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return int(row.Again), int(row.Hard), int(row.Good), int(row.Easy), nil
}
