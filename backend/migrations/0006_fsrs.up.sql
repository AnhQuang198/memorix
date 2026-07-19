-- === scheduling.cards: thêm trường FSRS còn thiếu (expand; AD-13 expand-and-contract) ===
-- 0004_cards đã tạo status/due_at/stability/difficulty/reps/lapses; chỉ last_review_at
-- là mới. IF NOT EXISTS đảm bảo idempotent, không đụng cột đã tồn tại.
ALTER TABLE scheduling.cards
  ADD COLUMN IF NOT EXISTS stability      double precision NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS difficulty     double precision NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS reps           integer          NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS lapses         integer          NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS due_at         timestamptz,
  ADD COLUMN IF NOT EXISTS last_review_at timestamptz;

-- Index nóng cho queue (owner_id, due_at) — NFR-2. 0004_cards đã tạo
-- idx_cards_owner_due; IF NOT EXISTS nên đây là no-op an toàn nếu đã có.
CREATE INDEX IF NOT EXISTS idx_cards_owner_due
  ON scheduling.cards (owner_id, due_at)
  WHERE deleted_at IS NULL;

-- === scheduling.user_scheduler_prefs ===
CREATE TABLE IF NOT EXISTS scheduling.user_scheduler_prefs (
  user_id            uuid PRIMARY KEY,                              -- ref logic identity.users (AD-10)
  desired_retention  double precision NOT NULL DEFAULT 0.90
      CHECK (desired_retention >= 0.80 AND desired_retention <= 0.97),
  daily_new_limit    integer NOT NULL DEFAULT 20
      CHECK (daily_new_limit BETWEEN 0 AND 9999),
  daily_review_limit integer NOT NULL DEFAULT 200
      CHECK (daily_review_limit BETWEEN 0 AND 9999),
  timezone           text NOT NULL DEFAULT 'UTC',
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);

-- === review.review_logs: append-only, partition theo THÁNG trên reviewed_at (AD-4) ===
-- Postgres yêu cầu mọi unique/PK trên bảng partitioned phải chứa cột phân vùng,
-- nên PK và unique defensive đều gồm reviewed_at.
CREATE TABLE IF NOT EXISTS review.review_logs (
  id               uuid        NOT NULL DEFAULT gen_random_uuid(),
  card_id          uuid        NOT NULL,                            -- ref logic scheduling.cards
  owner_id         uuid        NOT NULL,                            -- ref logic identity.users
  client_review_id text        NOT NULL,
  grade            smallint    NOT NULL CHECK (grade BETWEEN 1 AND 4),
  -- snapshot trước khi chấm (để replay + kiểm toán)
  prev_stability   double precision NOT NULL,
  prev_difficulty  double precision NOT NULL,
  prev_status      smallint    NOT NULL,
  retrievability   double precision NOT NULL,
  -- kết quả sau khi chấm
  new_stability    double precision NOT NULL,
  new_difficulty   double precision NOT NULL,
  new_status       smallint    NOT NULL,
  new_reps         integer     NOT NULL,
  new_lapses       integer     NOT NULL,
  new_due_at       timestamptz NOT NULL,
  elapsed_days     integer     NOT NULL,
  reviewed_at      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, reviewed_at),
  UNIQUE (card_id, client_review_id, reviewed_at)
) PARTITION BY RANGE (reviewed_at);

-- Partition DEFAULT bắt mọi hàng (test + an toàn khi worker chưa tạo partition tháng).
CREATE TABLE IF NOT EXISTS review.review_logs_default
  PARTITION OF review.review_logs DEFAULT;

CREATE INDEX IF NOT EXISTS idx_review_logs_owner_ts
  ON review.review_logs (owner_id, reviewed_at);
CREATE INDEX IF NOT EXISTS idx_review_logs_card_ts
  ON review.review_logs (card_id, reviewed_at);

-- === review.grade_receipts: idempotency guard (AD-3) — KHÔNG partition ===
-- unique(card_id, client_review_id) không đặt được trên bảng partitioned, nên
-- guard nằm ở đây; giữ snapshot kết quả để trả lại y hệt khi client retry.
CREATE TABLE IF NOT EXISTS review.grade_receipts (
  card_id          uuid        NOT NULL,                            -- ref logic scheduling.cards
  client_review_id text        NOT NULL,
  review_log_id    uuid        NOT NULL,                            -- ref logic review.review_logs
  new_stability    double precision NOT NULL,
  new_difficulty   double precision NOT NULL,
  new_status       smallint    NOT NULL,
  new_reps         integer     NOT NULL,
  new_lapses       integer     NOT NULL,
  new_due_at       timestamptz NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (card_id, client_review_id)
);
