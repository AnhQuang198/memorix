-- Nguồn chân lý query cho read model Progress (sqlc gen về internal/progress/repo/gen).
-- Ghi progress.daily_stats + progress.study_profiles; đọc review.review_logs (nguồn chân
-- lý, AD-4/AD-8) và scheduling.cards.due_at (forecast). review_logs KHÔNG lưu cột
-- scheduled_days rời — "interval kế" suy từ (new_due_at::date - reviewed_at::date).

-- name: BumpDailyStat :exec
INSERT INTO progress.daily_stats (user_id, day, reviews_done, new_done, retained, again, hard, good, easy)
VALUES (@user_id, @day, 1, @new_done, @retained, @again, @hard, @good, @easy)
ON CONFLICT (user_id, day) DO UPDATE SET
    reviews_done = progress.daily_stats.reviews_done + 1,
    new_done     = progress.daily_stats.new_done + EXCLUDED.new_done,
    retained     = progress.daily_stats.retained + EXCLUDED.retained,
    again        = progress.daily_stats.again + EXCLUDED.again,
    hard         = progress.daily_stats.hard + EXCLUDED.hard,
    good         = progress.daily_stats.good + EXCLUDED.good,
    easy         = progress.daily_stats.easy + EXCLUDED.easy,
    updated_at   = now();

-- name: GetStudyProfile :one
SELECT streak_current, streak_best, last_study_date, total_reviews, total_retained
FROM progress.study_profiles
WHERE user_id = @user_id;

-- name: UpsertStudyProfile :exec
INSERT INTO progress.study_profiles
    (user_id, streak_current, streak_best, last_study_date, total_reviews, total_retained)
VALUES (@user_id, @streak_current, @streak_best, @last_study_date, @total_reviews, @total_retained)
ON CONFLICT (user_id) DO UPDATE SET
    streak_current  = EXCLUDED.streak_current,
    streak_best     = EXCLUDED.streak_best,
    last_study_date = EXCLUDED.last_study_date,
    total_reviews   = EXCLUDED.total_reviews,
    total_retained  = EXCLUDED.total_retained,
    updated_at      = now();

-- name: DeleteDailyStats :exec
DELETE FROM progress.daily_stats WHERE user_id = @user_id;

-- name: InsertDailyStat :exec
INSERT INTO progress.daily_stats (user_id, day, reviews_done, new_done, retained, again, hard, good, easy)
VALUES (@user_id, @day, @reviews_done, @new_done, @retained, @again, @hard, @good, @easy);

-- name: DistinctOwners :many
SELECT DISTINCT owner_id FROM review.review_logs;

-- name: AllLogsForOwner :many
SELECT card_id, grade, (new_due_at::date - reviewed_at::date)::int AS scheduled_days, reviewed_at
FROM review.review_logs
WHERE owner_id = @owner_id
ORDER BY reviewed_at;

-- name: WeekRetentionLogs :many
SELECT card_id, grade, (new_due_at::date - reviewed_at::date)::int AS scheduled_days
FROM review.review_logs
WHERE owner_id = @owner_id
  AND reviewed_at >= @from_ts AND reviewed_at < @to_ts;

-- name: DueCount :one
SELECT count(*) FROM scheduling.cards
WHERE owner_id = @owner_id AND deleted_at IS NULL AND status <> 'suspended' AND due_at <= @now;

-- name: ForecastDue :many
SELECT (due_at AT TIME ZONE @tz::text)::date AS day, count(*) AS due
FROM scheduling.cards
WHERE owner_id = @owner_id AND deleted_at IS NULL AND status <> 'suspended'
  AND due_at >= @from_ts AND due_at < @to_ts
GROUP BY 1 ORDER BY 1;

-- name: TodayStat :one
SELECT reviews_done, new_done
FROM progress.daily_stats
WHERE user_id = @user_id AND day = @day;

-- name: HeatmapRange :many
SELECT day, reviews_done, retained
FROM progress.daily_stats
WHERE user_id = @user_id AND day >= @from_day AND day <= @to_day
ORDER BY day;

-- name: GradeDistribution :one
SELECT COALESCE(sum(again),0)::bigint AS again, COALESCE(sum(hard),0)::bigint AS hard,
       COALESCE(sum(good),0)::bigint AS good, COALESCE(sum(easy),0)::bigint AS easy
FROM progress.daily_stats
WHERE user_id = @user_id AND day >= @from_day AND day <= @to_day;
