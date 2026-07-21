-- Nguồn chân lý query cho smart-queue (sqlc gen về repo/gen — S7). Schema thực:
-- scheduling.cards.status là text ('new'|'learning'|'review'|'relearning'|'suspended'),
-- không phải int. Các query dưới khớp adapter QueueRepo (internal/scheduling/repo/repo.go).

-- name: LoadCandidates :many
SELECT id, owner_id, entry_id, direction, stability, difficulty,
       status, reps, lapses, due_at, last_review_at, created_at, updated_at
FROM scheduling.cards
WHERE owner_id = $1
  AND deleted_at IS NULL
  AND status <> 'suspended'
  AND (status = 'new' OR due_at <= $2)
ORDER BY owner_id, due_at ASC NULLS LAST;

-- name: DeferCard :exec
UPDATE scheduling.cards SET due_at = $2, updated_at = now() WHERE id = $1;

-- name: GetPrefs :one
SELECT user_id, desired_retention, daily_new_limit, daily_review_limit, timezone
FROM scheduling.user_scheduler_prefs WHERE user_id = $1;

-- name: UpdateLimits :one
INSERT INTO scheduling.user_scheduler_prefs (user_id, daily_new_limit, daily_review_limit)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE
  SET daily_new_limit = EXCLUDED.daily_new_limit,
      daily_review_limit = EXCLUDED.daily_review_limit,
      updated_at = now()
RETURNING user_id, desired_retention, daily_new_limit, daily_review_limit, timezone;

-- name: CoachSeenAt :one
SELECT learn_coach_seen_at FROM scheduling.study_profiles WHERE user_id = $1;

-- name: MarkCoachSeen :exec
INSERT INTO scheduling.study_profiles (user_id, learn_coach_seen_at)
VALUES ($1, now())
ON CONFLICT (user_id) DO UPDATE
  SET learn_coach_seen_at = now(), updated_at = now();

-- name: CountServedSince :one
-- Cross-module (AD-9): adapter ở review đọc review_logs; giữ ở đây làm tài liệu query.
SELECT
  COUNT(*) FILTER (WHERE prev_status = 0)  AS new_served,
  COUNT(*) FILTER (WHERE prev_status <> 0) AS review_served
FROM review.review_logs
WHERE owner_id = $1 AND reviewed_at >= $2;
