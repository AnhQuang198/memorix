DROP TABLE IF EXISTS review.grade_receipts;
DROP TABLE IF EXISTS review.review_logs_default;
DROP TABLE IF EXISTS review.review_logs;
DROP TABLE IF EXISTS scheduling.user_scheduler_prefs;
-- Chỉ gỡ cột 0006 thực sự thêm; status/due_at/stability/difficulty/reps/lapses và
-- index idx_cards_owner_due thuộc 0004_cards nên KHÔNG drop ở đây (expand-contract).
ALTER TABLE scheduling.cards
  DROP COLUMN IF EXISTS last_review_at;
