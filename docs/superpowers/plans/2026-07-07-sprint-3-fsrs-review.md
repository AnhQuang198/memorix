# Sprint 3 — FSRS Scheduling & Review Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Xây vòng học lõi Memorix — chấm thẻ FSRS (nguyên tử, idempotent, server-authoritative), queue thẻ đến hạn kèm khoảng cách ôn kế, replay-được từ `review_logs`, và màn ôn web (lật/chấm/phím/offline/tổng kết) — Epic 3, Stories 3.1–3.6.

**Architecture:** Hai module lõi `scheduling` + `review` theo **hexagonal đầy đủ** (addendum §Hexagonal có chọn lọc). Toán FSRS bọc sau `SchedulerPort` (AD-7) — `domain` không bao giờ import `go-fsrs` (depguard chặn); adapter trong `scheduling/repo/fsrsadapter` import lib. Chấm = **1 transaction** update `scheduling.cards` + append `review.review_logs` (AD-3, NFR-5), idempotency qua bảng guard `review.grade_receipts` `unique(card_id, client_review_id)` (partition buộc tách guard khỏi bảng log — xem Task 2). Client chỉ gửi `{card_id, grade, client_review_id}`; server tính S/D/Due (AD-5). `review_logs` append-only = nguồn chân lý, replay-được (AD-4). Đọc chéo module qua `VocabularyPort` batch-load (AD-9). Due theo server-time; "ngày học" theo TZ user (AD-12).

**Tech Stack:** Go 1.26, Gin v1.10, pgx v5 + pgxpool, `github.com/open-spaced-repetition/go-fsrs/v3` (v3.3.1 — bản stable mới nhất; v4.0.0-rc1 vẫn là release-candidate nên KHÔNG dùng), google/uuid, golang-migrate v4, testcontainers-go, testify, eventbus in-process (Sprint 0). Frontend: React 19 + Vite 7 + TS + vitest + @testing-library/react.

**Nguồn:** epics.md Epic 3 (3.1–3.6) · ARCHITECTURE-SPINE.md (AD-3,4,5,7,8,9,12) · memorix-spec/05-fsrs-analysis.md · addendum-structure.md · REUSE Sprint 0 (`internal/platform/{httpx,config,logger,eventbus,db}`, testcontainers) — `docs/superpowers/plans/2026-07-07-sprint-0-foundation.md`.

**Giả định đầu vào (từ Sprint 1 + 2):** `identity` auth + `authmw` cấp principal; `vocabulary.entries` (+ meanings/pronunciations/examples) và `vocabulary/ports` expose batch-load entry; `scheduling.cards` đã tồn tại ở trạng thái New (id, owner_id, entry_id, direction, created_at, updated_at, deleted_at). Migration Sprint 0=`0001`, Sprint 1≈`0002`, Sprint 2≈`0003` → sprint này dùng `0004` (đổi số nếu repo đã khác).

**Scope boundary:** CHỈ `scheduling` + `review`. Queue ở đây là **due cơ bản** (due_at ≤ now, sort due_at) — priority nâng cao + daily limit + chống-nổ = Epic 4/Sprint 4. Progress read model (daily_stats/streak/North Star) = Sprint 5; sprint này chỉ **phát event `CardGraded`** + đọc thẳng `review_logs` cho summary (AD-8). Fuzz interval TẮT (determinism cho replay AD-4; fuzz là future extension).

---

## Cross-Sprint Auth Contract (canonical — Sprint 1)

Sprint 1 sở hữu `internal/platform/authmw`. Downstream **phải** dùng đúng API này, không tự chế reader/context-key:
- `authmw.RequireAuth(jwtManager) gin.HandlerFunc` — guard route cần đăng nhập.
- `authmw.PrincipalFrom(c) (Principal, bool)` · `Principal{UserID string, Role string, Plan string}`.
- `authmw.UserID(c) (string, bool)` — reader tiện lợi; **UserID là uuid dạng string** (KHÔNG phải `(uuid.UUID, error)`). `uuid.Parse(uid)` ở ranh giới repo nếu cần `uuid.UUID`.
- **Timezone KHÔNG nằm trong principal/context** — lấy qua `IdentityPort.UserTimezone(ctx, userID) (string, error)` (AD-9) rồi `time.LoadLocation` (AD-12).
- Test: fake bằng middleware test gọi `c.Set` với đúng `authmw.Principal{...}`.

> Áp dụng: chỗ Task 16 giả định `authmw.UserID(c) (uuid.UUID, error)` → sửa thành `uid, ok := authmw.UserID(c)` (string) + `uuid.Parse(uid)` ở adapter; bỏ shim header `X-User-ID`.

---

### Task 1: platform/db — Querier + WithinTx + dbtest helper (TDD)

Unit-of-work để 1 transaction chấm span 2 schema (AD-3). `Querier` được cả `*pgxpool.Pool` lẫn `pgx.Tx` thỏa mãn → repo adapter nhận `Querier`, chạy trong hoặc ngoài tx tùy caller.

**Files:**
- Create: `internal/platform/db/querier.go`
- Create: `internal/platform/db/tx.go`
- Test: `internal/platform/db/tx_test.go`
- Create: `internal/platform/db/dbtest/dbtest.go` (helper container dùng chung mọi integration test sprint này)

- [ ] **Step 1: Add deps**

Run:
```bash
go get github.com/jackc/pgx/v5/pgxpool@latest
go get github.com/google/uuid@latest
go get github.com/open-spaced-repetition/go-fsrs/v3@v3.3.1
go get github.com/stretchr/testify@latest
```
Expected: các module thêm vào `go.mod`.

- [ ] **Step 2: Write Querier interface**

Create `internal/platform/db/querier.go`:
```go
package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier là tập method chung của *pgxpool.Pool và pgx.Tx. Repo adapter nhận
// Querier để cùng một code chạy được trong TX (unit-of-work) hoặc ngoài TX
// (read thuần) — nền cho grade nguyên tử span nhiều schema (AD-3).
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

- [ ] **Step 3: Write the failing test**

Create `internal/platform/db/tx_test.go`:
```go
package db_test

import (
	"context"
	"testing"

	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/stretchr/testify/require"
)

func TestWithinTx_CommitsOnNil(t *testing.T) {
	pool := dbtest.NewPostgres(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.tx_probe(n int)`)
	require.NoError(t, err)

	err = db.WithinTx(ctx, pool, func(q db.Querier) error {
		_, e := q.Exec(ctx, `INSERT INTO public.tx_probe(n) VALUES (1)`)
		return e
	})
	require.NoError(t, err)

	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM public.tx_probe`).Scan(&n))
	require.Equal(t, 1, n)
}

func TestWithinTx_RollsBackOnError(t *testing.T) {
	pool := dbtest.NewPostgres(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.tx_probe2(n int)`)
	require.NoError(t, err)

	wantErr := context.Canceled
	err = db.WithinTx(ctx, pool, func(q db.Querier) error {
		_, _ = q.Exec(ctx, `INSERT INTO public.tx_probe2(n) VALUES (1)`)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM public.tx_probe2`).Scan(&n))
	require.Equal(t, 0, n, "insert phải bị rollback")
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/platform/db/ -run TestWithinTx -v`
Expected: FAIL (build error — `db.WithinTx`, `dbtest.NewPostgres` chưa có).

- [ ] **Step 5: Write WithinTx**

Create `internal/platform/db/tx.go`:
```go
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WithinTx chạy fn trong 1 transaction: commit nếu fn trả nil, rollback nếu lỗi.
// pgx.Tx thỏa Querier nên fn dùng chung code với path ngoài-TX.
func WithinTx(ctx context.Context, pool *pgxpool.Pool, fn func(Querier) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
```

- [ ] **Step 6: Write dbtest helper**

Create `internal/platform/db/dbtest/dbtest.go`:
```go
package dbtest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NewPostgres khởi Postgres 18 qua testcontainers, áp toàn bộ migration, trả pool.
// Skip khi -short (CI đơn vị). Dùng chung cho mọi integration test của sprint.
func NewPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skip container test in -short mode")
	}
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "postgres:18",
		postgres.WithDatabase("memorix"),
		postgres.WithUsername("test"), postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start pg: %v", err)
	}
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	if err := db.Migrate("file://"+filepath.Join(repoRoot(t), "migrations"), dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		_ = pg.Terminate(ctx)
	})
	return pool
}

// repoRoot đi lên từ CWD của test tới thư mục chứa go.mod (root repo).
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from test dir")
		}
		dir = parent
	}
}
```

- [ ] **Step 7: Run test to verify it passes**

Run: `go test ./internal/platform/db/ -run TestWithinTx -v`
Expected: PASS (2 tests; lần đầu chậm do pull image). Migration `0004` chưa tồn tại nhưng `0001` đủ để test này (chỉ dùng bảng public tạm).

- [ ] **Step 8: Commit**

```bash
git add internal/platform/db go.mod go.sum
git commit -m "feat(platform): Querier unit-of-work, WithinTx, dbtest container helper"
```

---

### Task 2: Migration 0004 — FSRS fields, prefs, partitioned review_logs, idempotency guard (TDD)

**Files:**
- Create: `migrations/0004_scheduling_review_fsrs.up.sql`
- Create: `migrations/0004_scheduling_review_fsrs.down.sql`
- Test: `internal/platform/db/migrate0004_test.go`

> **Ghi chú thiết kế (quan trọng):** Postgres yêu cầu MỌI unique/PK trên bảng partitioned phải chứa cột phân vùng. Vì `review_logs` partition theo `reviewed_at`, KHÔNG thể đặt `unique(card_id, client_review_id)` trực tiếp trên nó. Idempotency guard (AD-3) do đó nằm ở bảng **không partition** `review.grade_receipts` `PRIMARY KEY (card_id, client_review_id)`. `review_logs` giữ vai trò append-only replay-source (AD-4) với `unique(card_id, client_review_id, reviewed_at)` phòng thủ. Cả hai ghi trong cùng 1 TX chấm.

- [ ] **Step 1: Write up migration**

Create `migrations/0004_scheduling_review_fsrs.up.sql`:
```sql
-- === scheduling.cards: thêm trường FSRS (expand; AD-13 expand-and-contract) ===
ALTER TABLE scheduling.cards
  ADD COLUMN IF NOT EXISTS stability      double precision NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS difficulty     double precision NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS status         smallint         NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS reps           integer          NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS lapses         integer          NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS due_at         timestamptz      NOT NULL DEFAULT now(),
  ADD COLUMN IF NOT EXISTS last_review_at timestamptz;

-- Index nóng cho queue (owner_id, due_at) — NFR-2, FSRS-analysis §Database.
CREATE INDEX IF NOT EXISTS idx_cards_owner_due
  ON scheduling.cards (owner_id, due_at)
  WHERE deleted_at IS NULL;

-- === scheduling.user_scheduler_prefs ===
CREATE TABLE IF NOT EXISTS scheduling.user_scheduler_prefs (
  user_id            uuid PRIMARY KEY,
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
CREATE TABLE IF NOT EXISTS review.review_logs (
  id               uuid        NOT NULL DEFAULT gen_random_uuid(),
  card_id          uuid        NOT NULL,
  owner_id         uuid        NOT NULL,
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
  card_id          uuid        NOT NULL,
  client_review_id text        NOT NULL,
  review_log_id    uuid        NOT NULL,
  new_stability    double precision NOT NULL,
  new_difficulty   double precision NOT NULL,
  new_status       smallint    NOT NULL,
  new_reps         integer     NOT NULL,
  new_lapses       integer     NOT NULL,
  new_due_at       timestamptz NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (card_id, client_review_id)
);
```

- [ ] **Step 2: Write down migration**

Create `migrations/0004_scheduling_review_fsrs.down.sql`:
```sql
DROP TABLE IF EXISTS review.grade_receipts;
DROP TABLE IF EXISTS review.review_logs_default;
DROP TABLE IF EXISTS review.review_logs;
DROP TABLE IF EXISTS scheduling.user_scheduler_prefs;
DROP INDEX IF EXISTS scheduling.idx_cards_owner_due;
ALTER TABLE scheduling.cards
  DROP COLUMN IF EXISTS last_review_at,
  DROP COLUMN IF EXISTS due_at,
  DROP COLUMN IF EXISTS lapses,
  DROP COLUMN IF EXISTS reps,
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS difficulty,
  DROP COLUMN IF EXISTS stability;
```

> **Giả định:** Sprint 2 đã tạo `scheduling.cards`. Nếu chưa, thêm block `CREATE TABLE IF NOT EXISTS scheduling.cards(...)` vào ĐẦU file up (id uuid PK, owner_id uuid, entry_id uuid, direction text, created_at/updated_at timestamptz, deleted_at timestamptz). Các test dưới tự tạo card qua INSERT nên không phụ thuộc cột ngoài FSRS.

- [ ] **Step 3: Write the failing test**

Create `internal/platform/db/migrate0004_test.go`:
```go
package db_test

import (
	"context"
	"testing"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/stretchr/testify/require"
)

func TestMigration0004_Schema(t *testing.T) {
	pool := dbtest.NewPostgres(t)
	ctx := context.Background()

	// cards có đủ cột FSRS
	var cols int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema='scheduling' AND table_name='cards'
		  AND column_name IN ('stability','difficulty','status','reps','lapses','due_at','last_review_at')
	`).Scan(&cols))
	require.Equal(t, 7, cols)

	// review_logs là bảng partitioned
	var isPartitioned bool
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM pg_partitioned_table pt
			JOIN pg_class c ON c.oid=pt.partrelid
			JOIN pg_namespace n ON n.oid=c.relnamespace
			WHERE n.nspname='review' AND c.relname='review_logs')
	`).Scan(&isPartitioned))
	require.True(t, isPartitioned, "review_logs phải partitioned")

	// CHECK desired_retention chặn ngoài [0.80, 0.97]
	_, err := pool.Exec(ctx, `
		INSERT INTO scheduling.user_scheduler_prefs(user_id, desired_retention)
		VALUES (gen_random_uuid(), 0.5)`)
	require.Error(t, err, "0.5 phải bị CHECK từ chối")

	// grade_receipts có unique(card_id, client_review_id)
	cid := "11111111-1111-1111-1111-111111111111"
	ins := func() error {
		_, e := pool.Exec(ctx, `
			INSERT INTO review.grade_receipts
			  (card_id, client_review_id, review_log_id, new_stability, new_difficulty,
			   new_status, new_reps, new_lapses, new_due_at)
			VALUES ($1,'cr-1',gen_random_uuid(),1,5,2,1,0,now())`, cid)
		return e
	}
	require.NoError(t, ins())
	require.Error(t, ins(), "trùng (card_id, client_review_id) phải bị chặn")
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/platform/db/ -run TestMigration0004 -v`
Expected: FAIL (migration `0004` chưa áp → cột/bảng thiếu → assert fail hoặc lỗi SQL).

- [ ] **Step 5: Verify it passes (impl = SQL đã viết ở Step 1-2)**

Run: `go test ./internal/platform/db/ -run TestMigration0004 -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add migrations internal/platform/db/migrate0004_test.go
git commit -m "feat(db): FSRS card fields, scheduler prefs, partitioned review_logs, idempotency guard"
```

---

### Task 3: scheduling/domain — Card, Grade, ScheduleResult, prefs, StudyDay (TDD, pure)

`domain` thuần: chỉ `time` + `google/uuid` (depguard chặn gin/pgx/net/http — AD-2). Không import go-fsrs.

**Files:**
- Create: `internal/scheduling/domain/card.go`
- Create: `internal/scheduling/domain/schedule.go`
- Create: `internal/scheduling/domain/prefs.go`
- Create: `internal/scheduling/domain/studyday.go`
- Test: `internal/scheduling/domain/domain_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/scheduling/domain/domain_test.go`:
```go
package domain_test

import (
	"testing"
	"time"

	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/stretchr/testify/require"
)

func TestGrade_Valid(t *testing.T) {
	require.True(t, domain.GradeAgain.Valid())
	require.True(t, domain.GradeEasy.Valid())
	require.False(t, domain.Grade(0).Valid())
	require.False(t, domain.Grade(5).Valid())
}

func TestStatusAlignsWithFSRS(t *testing.T) {
	// domain phải map 1-1 với go-fsrs State (New=0..Relearning=3) — adapter dựa vào.
	require.EqualValues(t, 0, domain.StatusNew)
	require.EqualValues(t, 3, domain.StatusRelearning)
}

func TestStudyDayStart_GraceBeforeDawn(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Bangkok") // UTC+7
	require.NoError(t, err)
	// 02:00 giờ Bangkok, grace 4h → vẫn thuộc "ngày học" hôm trước.
	now := time.Date(2026, 7, 18, 2, 0, 0, 0, loc)
	start := domain.StudyDayStart(now, loc, 4)
	require.Equal(t, 2026, start.Year())
	require.Equal(t, time.July, start.Month())
	require.Equal(t, 17, start.Day(), "trước 4h sáng tính là ngày hôm trước")
	require.Equal(t, 4, start.Hour())
}

func TestStudyDayStart_AfterDawn(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 7, 18, 9, 0, 0, 0, loc)
	start := domain.StudyDayStart(now, loc, 4)
	require.Equal(t, 18, start.Day())
}

func TestPrefs_Default(t *testing.T) {
	p := domain.DefaultPrefs()
	require.InDelta(t, 0.90, p.DesiredRetention, 1e-9)
	require.Equal(t, "UTC", p.Timezone)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scheduling/domain/ -v`
Expected: FAIL (package/types chưa có).

- [ ] **Step 3: Write card.go**

Create `internal/scheduling/domain/card.go`:
```go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// CardStatus map 1-1 với go-fsrs State (New=0, Learning=1, Review=2, Relearning=3).
type CardStatus int16

const (
	StatusNew CardStatus = iota
	StatusLearning
	StatusReview
	StatusRelearning
)

// Grade map 1-1 với go-fsrs Rating (Again=1..Easy=4).
type Grade int16

const (
	GradeAgain Grade = iota + 1
	GradeHard
	GradeGood
	GradeEasy
)

func (g Grade) Valid() bool { return g >= GradeAgain && g <= GradeEasy }

// Card = trạng thái học FSRS per-user, per-direction (AD-6). Không nhúng nội dung entry.
type Card struct {
	ID           uuid.UUID
	OwnerID      uuid.UUID
	EntryID      uuid.UUID
	Direction    string
	Stability    float64
	Difficulty   float64
	Status       CardStatus
	Reps         int
	Lapses       int
	DueAt        time.Time
	LastReviewAt *time.Time
	CreatedAt    time.Time
}
```

- [ ] **Step 4: Write schedule.go**

Create `internal/scheduling/domain/schedule.go`:
```go
package domain

import "time"

// ScheduleResult = kết quả FSRS sau một lần chấm (do SchedulerPort trả).
type ScheduleResult struct {
	Stability     float64
	Difficulty    float64
	Status        CardStatus
	Reps          int
	Lapses        int
	DueAt         time.Time
	LastReviewAt  time.Time
	ElapsedDays   int
	Retrievability float64
}

// NextIntervals = khoảng cách tới lần ôn kế cho từng mức (FR-14). Server tính, client hiển thị.
type NextIntervals struct {
	Again time.Duration
	Hard  time.Duration
	Good  time.Duration
	Easy  time.Duration
}
```

- [ ] **Step 5: Write prefs.go**

Create `internal/scheduling/domain/prefs.go`:
```go
package domain

import "github.com/google/uuid"

// SchedulerPrefs = cấu hình lịch per-user (FR-17, FR-26).
type SchedulerPrefs struct {
	UserID           uuid.UUID
	DesiredRetention float64
	DailyNewLimit    int
	DailyReviewLimit int
	Timezone         string
}

// DefaultPrefs khi user chưa cấu hình.
func DefaultPrefs() SchedulerPrefs {
	return SchedulerPrefs{
		DesiredRetention: 0.90,
		DailyNewLimit:    20,
		DailyReviewLimit: 200,
		Timezone:         "UTC",
	}
}

// RetentionInRange kiểm tra desired retention hợp lệ (0.80–0.97, FR-17).
func RetentionInRange(r float64) bool { return r >= 0.80 && r <= 0.97 }
```

- [ ] **Step 6: Write studyday.go**

Create `internal/scheduling/domain/studyday.go`:
```go
package domain

import "time"

// StudyDayStart trả mốc bắt đầu "ngày học" mà `now` thuộc về, theo TZ user, có
// grace tới graceHour giờ sáng (AD-12): trước graceHour tính là ngày hôm trước.
func StudyDayStart(now time.Time, loc *time.Location, graceHour int) time.Time {
	local := now.In(loc)
	d := local
	if local.Hour() < graceHour {
		d = local.AddDate(0, 0, -1)
	}
	return time.Date(d.Year(), d.Month(), d.Day(), graceHour, 0, 0, 0, loc)
}
```

- [ ] **Step 7: Run test to verify it passes**

Run: `go test ./internal/scheduling/domain/ -v`
Expected: PASS (5 tests).

- [ ] **Step 8: Verify depguard keeps domain pure**

Run: `golangci-lint run ./internal/scheduling/domain/`
Expected: no issues (chỉ import `time` + `google/uuid`).

- [ ] **Step 9: Commit**

```bash
git add internal/scheduling/domain
git commit -m "feat(scheduling): pure domain — Card, Grade, ScheduleResult, prefs, study-day (AD-12)"
```

---

### Task 4: scheduling/ports — SchedulerPort, CardStore, PrefsStore (compile-check)

Interfaces expose ra ngoài module (S2). `ports` được import pgx (depguard chỉ chặn `domain`).

**Files:**
- Create: `internal/scheduling/ports/scheduler.go`
- Create: `internal/scheduling/ports/stores.go`
- Test: `internal/scheduling/ports/ports_test.go`

- [ ] **Step 1: Write the failing test (interface assertion)**

Create `internal/scheduling/ports/ports_test.go`:
```go
package ports_test

import (
	"testing"

	"github.com/memorix/memorix/internal/scheduling/ports"
)

// Ép compile: xác nhận interface tồn tại + chữ ký ổn định.
func TestPortsDeclared(t *testing.T) {
	var _ ports.SchedulerPort
	var _ ports.CardStore
	var _ ports.PrefsStore
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scheduling/ports/ -v`
Expected: FAIL (interface chưa khai báo).

- [ ] **Step 3: Write scheduler.go**

Create `internal/scheduling/ports/scheduler.go`:
```go
package ports

import (
	"time"

	"github.com/memorix/memorix/internal/scheduling/domain"
)

// SchedulerPort bọc toán FSRS (AD-7). domain KHÔNG import go-fsrs; adapter
// (scheduling/repo/fsrsadapter) implement port này bằng go-fsrs. Cho phép A/B
// nhiều impl trên cùng review_logs.
type SchedulerPort interface {
	// Apply tính trạng thái card sau khi chấm `grade` tại `now` với desired retention.
	Apply(card domain.Card, grade domain.Grade, retention float64, now time.Time) domain.ScheduleResult
	// Preview trả khoảng cách ôn kế cho cả 4 mức (FR-14), không thay đổi card.
	Preview(card domain.Card, retention float64, now time.Time) domain.NextIntervals
}
```

- [ ] **Step 4: Write stores.go**

Create `internal/scheduling/ports/stores.go`:
```go
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/scheduling/domain"
)

// CardStore = driven adapter cho scheduling.cards. Nhận Querier để tham gia TX
// chấm span nhiều schema (AD-3). ErrCardNotFound khi không thuộc owner (AD-8 deny).
type CardStore interface {
	Load(ctx context.Context, q db.Querier, cardID, ownerID uuid.UUID) (domain.Card, error)
	ApplyResult(ctx context.Context, q db.Querier, cardID uuid.UUID, r domain.ScheduleResult) error
	DueCards(ctx context.Context, q db.Querier, ownerID uuid.UUID, now time.Time, limit int) ([]domain.Card, error)
}

// PrefsStore = driven adapter cho scheduling.user_scheduler_prefs.
type PrefsStore interface {
	Get(ctx context.Context, q db.Querier, userID uuid.UUID) (domain.SchedulerPrefs, error)
	Upsert(ctx context.Context, q db.Querier, p domain.SchedulerPrefs) error
}
```

- [ ] **Step 5: Add ErrCardNotFound to domain**

Create `internal/scheduling/domain/errors.go`:
```go
package domain

import "errors"

// ErrCardNotFound: card không tồn tại hoặc không thuộc owner (deny-by-default, NFR-8).
var ErrCardNotFound = errors.New("card not found")
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/scheduling/ports/ -v && go build ./internal/scheduling/...`
Expected: PASS + build sạch.

- [ ] **Step 7: Commit**

```bash
git add internal/scheduling/ports internal/scheduling/domain/errors.go
git commit -m "feat(scheduling): ports — SchedulerPort (FSRS), CardStore, PrefsStore (AD-7,AD-9)"
```

---

### Task 5: scheduling/repo/fsrsadapter — bọc go-fsrs (TDD, pure, không container)

Adapter DUY NHẤT được import `go-fsrs` (AD-7). Fuzz TẮT để determinism cho replay (AD-4).

**Files:**
- Create: `internal/scheduling/repo/fsrsadapter/adapter.go`
- Test: `internal/scheduling/repo/fsrsadapter/adapter_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/scheduling/repo/fsrsadapter/adapter_test.go`:
```go
package fsrsadapter_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/repo/fsrsadapter"
	"github.com/stretchr/testify/require"
)

func newCard() domain.Card {
	return domain.Card{
		ID: uuid.New(), OwnerID: uuid.New(), EntryID: uuid.New(),
		Direction: "front_back", Status: domain.StatusNew,
		DueAt: time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC),
	}
}

func TestApply_GoodOnNewCard_BuildsPositiveStabilityAndFutureDue(t *testing.T) {
	a := fsrsadapter.New()
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	r := a.Apply(newCard(), domain.GradeGood, 0.90, now)
	require.Greater(t, r.Stability, 0.0)
	require.GreaterOrEqual(t, r.Difficulty, 1.0)
	require.LessOrEqual(t, r.Difficulty, 10.0)
	require.True(t, r.DueAt.After(now), "due phải ở tương lai")
	require.Equal(t, 1, r.Reps)
	require.Equal(t, now, r.LastReviewAt)
}

func TestApply_AgainCountsLapseFromReviewCard(t *testing.T) {
	a := fsrsadapter.New()
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	// card đang ở Review, quên → lapse tăng, chuyển Relearning.
	c := newCard()
	c.Status = domain.StatusReview
	c.Stability = 10
	c.Difficulty = 5
	last := now.AddDate(0, 0, -10)
	c.LastReviewAt = &last
	r := a.Apply(c, domain.GradeAgain, 0.90, now)
	require.Equal(t, 1, r.Lapses)
	require.Equal(t, domain.StatusRelearning, r.Status)
}

func TestApply_Deterministic(t *testing.T) {
	a := fsrsadapter.New()
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	c := newCard()
	r1 := a.Apply(c, domain.GradeGood, 0.90, now)
	r2 := a.Apply(c, domain.GradeGood, 0.90, now)
	require.Equal(t, r1.Stability, r2.Stability)
	require.Equal(t, r1.DueAt, r2.DueAt, "fuzz TẮT → Due lặp lại y hệt (replay AD-4)")
}

func TestPreview_FourIntervalsOrdered(t *testing.T) {
	a := fsrsadapter.New()
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	c := newCard()
	c.Status = domain.StatusReview
	c.Stability = 10
	c.Difficulty = 5
	last := now.AddDate(0, 0, -10)
	c.LastReviewAt = &last
	iv := a.Preview(c, 0.90, now)
	// Again ≤ Hard ≤ Good ≤ Easy (ngữ nghĩa Anki).
	require.LessOrEqual(t, iv.Again, iv.Hard)
	require.LessOrEqual(t, iv.Hard, iv.Good)
	require.LessOrEqual(t, iv.Good, iv.Easy)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scheduling/repo/fsrsadapter/ -v`
Expected: FAIL (`fsrsadapter.New` chưa có).

- [ ] **Step 3: Write adapter**

Create `internal/scheduling/repo/fsrsadapter/adapter.go`:
```go
package fsrsadapter

import (
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v3"

	"github.com/memorix/memorix/internal/scheduling/domain"
)

// Adapter implement ports.SchedulerPort bằng go-fsrs v3 (AD-7). Đây là NƠI DUY
// NHẤT import go-fsrs; domain/ports/service không đụng lib.
type Adapter struct{}

func New() *Adapter { return &Adapter{} }

// params dựng Parameters với desired retention của user; fuzz TẮT cho determinism.
func (a *Adapter) params(retention float64) fsrs.Parameters {
	p := fsrs.DefaultParam()
	p.RequestRetention = retention
	p.EnableFuzz = false // replay-được (AD-4); fuzz load-balancing là future extension
	return p
}

func toFSRS(c domain.Card) fsrs.Card {
	fc := fsrs.NewCard()
	fc.State = fsrs.State(c.Status)
	fc.Reps = uint64(c.Reps)
	fc.Lapses = uint64(c.Lapses)
	fc.Due = c.DueAt
	if c.Status != domain.StatusNew {
		fc.Stability = c.Stability
		fc.Difficulty = c.Difficulty
	}
	if c.LastReviewAt != nil {
		fc.LastReview = *c.LastReviewAt
	}
	return fc
}

func (a *Adapter) Apply(card domain.Card, grade domain.Grade, retention float64, now time.Time) domain.ScheduleResult {
	f := fsrs.NewFSRS(a.params(retention))
	fc := toFSRS(card)
	r := f.GetRetrievability(fc, now)
	info := f.Next(fc, now, fsrs.Rating(grade))
	nc := info.Card
	return domain.ScheduleResult{
		Stability:      nc.Stability,
		Difficulty:     nc.Difficulty,
		Status:         domain.CardStatus(nc.State),
		Reps:           int(nc.Reps),
		Lapses:         int(nc.Lapses),
		DueAt:          nc.Due,
		LastReviewAt:   now,
		ElapsedDays:    int(nc.ElapsedDays),
		Retrievability: r,
	}
}

func (a *Adapter) Preview(card domain.Card, retention float64, now time.Time) domain.NextIntervals {
	f := fsrs.NewFSRS(a.params(retention))
	fc := toFSRS(card)
	m := f.Repeat(fc, now)
	return domain.NextIntervals{
		Again: m[fsrs.Again].Card.Due.Sub(now),
		Hard:  m[fsrs.Hard].Card.Due.Sub(now),
		Good:  m[fsrs.Good].Card.Due.Sub(now),
		Easy:  m[fsrs.Easy].Card.Due.Sub(now),
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scheduling/repo/fsrsadapter/ -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Assert adapter satisfies the port**

Add to end of `internal/scheduling/repo/fsrsadapter/adapter.go`:
```go
// compile-time check: Adapter thỏa SchedulerPort.
var _ interface {
	Apply(domain.Card, domain.Grade, float64, time.Time) domain.ScheduleResult
	Preview(domain.Card, float64, time.Time) domain.NextIntervals
} = (*Adapter)(nil)
```

Run: `go build ./internal/scheduling/...`
Expected: build sạch.

- [ ] **Step 6: Commit**

```bash
git add internal/scheduling/repo/fsrsadapter go.mod go.sum
git commit -m "feat(scheduling): FSRS adapter wrapping go-fsrs v3.3.1 behind SchedulerPort (AD-7)"
```

---

### Task 6: scheduling/repo — pgx CardStore + PrefsStore (TDD, testcontainers)

**Files:**
- Create: `internal/scheduling/repo/cardstore.go`
- Create: `internal/scheduling/repo/prefsstore.go`
- Test: `internal/scheduling/repo/repo_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/scheduling/repo/repo_test.go`:
```go
package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/repo"
	"github.com/stretchr/testify/require"
)

func seedCard(t *testing.T, ctx context.Context, q interface {
	Exec(context.Context, string, ...any) (pgconnTag, error)
}, owner, entry uuid.UUID, due time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := q.Exec(ctx, `
		INSERT INTO scheduling.cards (id, owner_id, entry_id, direction, status, due_at, created_at, updated_at)
		VALUES ($1,$2,$3,'front_back',0,$4,now(),now())`, id, owner, entry, due)
	require.NoError(t, err)
	return id
}

// alias để tránh import pgconn trong chữ ký helper (pool.Exec trả pgconn.CommandTag).
type pgconnTag = interface{ String() string }

func TestCardStore_LoadApplyResult(t *testing.T) {
	pool := dbtest.NewPostgres(t)
	ctx := context.Background()
	owner, entry := uuid.New(), uuid.New()
	due := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	id := seedCard(t, ctx, pool, owner, entry, due)

	cs := repo.NewCardStore()
	card, err := cs.Load(ctx, pool, id, owner)
	require.NoError(t, err)
	require.Equal(t, entry, card.EntryID)
	require.Equal(t, domain.StatusNew, card.Status)

	// ownership: owner khác → not found
	_, err = cs.Load(ctx, pool, id, uuid.New())
	require.ErrorIs(t, err, domain.ErrCardNotFound)

	res := domain.ScheduleResult{
		Stability: 12.5, Difficulty: 6.0, Status: domain.StatusReview,
		Reps: 1, Lapses: 0, DueAt: due.AddDate(0, 0, 12), LastReviewAt: due,
	}
	require.NoError(t, cs.ApplyResult(ctx, pool, id, res))

	got, err := cs.Load(ctx, pool, id, owner)
	require.NoError(t, err)
	require.InDelta(t, 12.5, got.Stability, 1e-9)
	require.Equal(t, domain.StatusReview, got.Status)
	require.NotNil(t, got.LastReviewAt)
}

func TestCardStore_DueCards(t *testing.T) {
	pool := dbtest.NewPostgres(t)
	ctx := context.Background()
	owner := uuid.New()
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	seedCard(t, ctx, pool, owner, uuid.New(), now.Add(-time.Hour)) // due
	seedCard(t, ctx, pool, owner, uuid.New(), now.Add(time.Hour))  // chưa due

	cs := repo.NewCardStore()
	cards, err := cs.DueCards(ctx, pool, owner, now, 50)
	require.NoError(t, err)
	require.Len(t, cards, 1)
}

func TestPrefsStore_UpsertGetDefault(t *testing.T) {
	pool := dbtest.NewPostgres(t)
	ctx := context.Background()
	ps := repo.NewPrefsStore()
	uid := uuid.New()

	// chưa cấu hình → default
	p, err := ps.Get(ctx, pool, uid)
	require.NoError(t, err)
	require.InDelta(t, 0.90, p.DesiredRetention, 1e-9)

	p.DesiredRetention = 0.85
	p.Timezone = "Asia/Bangkok"
	require.NoError(t, ps.Upsert(ctx, pool, p))

	got, err := ps.Get(ctx, pool, uid)
	require.NoError(t, err)
	require.InDelta(t, 0.85, got.DesiredRetention, 1e-9)
	require.Equal(t, "Asia/Bangkok", got.Timezone)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scheduling/repo/ -run 'CardStore|PrefsStore' -v`
Expected: FAIL (`repo.NewCardStore`/`NewPrefsStore` chưa có).

- [ ] **Step 3: Write cardstore.go**

Create `internal/scheduling/repo/cardstore.go`:
```go
package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/scheduling/domain"
)

// CardStore = adapter pgx cho scheduling.cards (S7 — repo implements port).
type CardStore struct{}

func NewCardStore() *CardStore { return &CardStore{} }

const cardCols = `id, owner_id, entry_id, direction, stability, difficulty,
	status, reps, lapses, due_at, last_review_at, created_at`

func scanCard(row pgx.Row) (domain.Card, error) {
	var c domain.Card
	err := row.Scan(&c.ID, &c.OwnerID, &c.EntryID, &c.Direction, &c.Stability,
		&c.Difficulty, &c.Status, &c.Reps, &c.Lapses, &c.DueAt, &c.LastReviewAt, &c.CreatedAt)
	return c, err
}

func (s *CardStore) Load(ctx context.Context, q db.Querier, cardID, ownerID uuid.UUID) (domain.Card, error) {
	row := q.QueryRow(ctx, `SELECT `+cardCols+`
		FROM scheduling.cards
		WHERE id=$1 AND owner_id=$2 AND deleted_at IS NULL`, cardID, ownerID)
	c, err := scanCard(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Card{}, domain.ErrCardNotFound
	}
	return c, err
}

func (s *CardStore) ApplyResult(ctx context.Context, q db.Querier, cardID uuid.UUID, r domain.ScheduleResult) error {
	_, err := q.Exec(ctx, `
		UPDATE scheduling.cards
		SET stability=$2, difficulty=$3, status=$4, reps=$5, lapses=$6,
		    due_at=$7, last_review_at=$8, updated_at=now()
		WHERE id=$1`,
		cardID, r.Stability, r.Difficulty, r.Status, r.Reps, r.Lapses, r.DueAt, r.LastReviewAt)
	return err
}

func (s *CardStore) DueCards(ctx context.Context, q db.Querier, ownerID uuid.UUID, now time.Time, limit int) ([]domain.Card, error) {
	rows, err := q.Query(ctx, `SELECT `+cardCols+`
		FROM scheduling.cards
		WHERE owner_id=$1 AND due_at<=$2 AND deleted_at IS NULL
		ORDER BY due_at ASC
		LIMIT $3`, ownerID, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Write prefsstore.go**

Create `internal/scheduling/repo/prefsstore.go`:
```go
package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/scheduling/domain"
)

// PrefsStore = adapter pgx cho scheduling.user_scheduler_prefs.
type PrefsStore struct{}

func NewPrefsStore() *PrefsStore { return &PrefsStore{} }

func (s *PrefsStore) Get(ctx context.Context, q db.Querier, userID uuid.UUID) (domain.SchedulerPrefs, error) {
	row := q.QueryRow(ctx, `
		SELECT desired_retention, daily_new_limit, daily_review_limit, timezone
		FROM scheduling.user_scheduler_prefs WHERE user_id=$1`, userID)
	p := domain.DefaultPrefs()
	p.UserID = userID
	err := row.Scan(&p.DesiredRetention, &p.DailyNewLimit, &p.DailyReviewLimit, &p.Timezone)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DefaultPrefs(), nil // chưa cấu hình → default (không rewrite quá khứ)
	}
	if err != nil {
		return domain.SchedulerPrefs{}, err
	}
	return p, nil
}

func (s *PrefsStore) Upsert(ctx context.Context, q db.Querier, p domain.SchedulerPrefs) error {
	_, err := q.Exec(ctx, `
		INSERT INTO scheduling.user_scheduler_prefs
		  (user_id, desired_retention, daily_new_limit, daily_review_limit, timezone, updated_at)
		VALUES ($1,$2,$3,$4,$5,now())
		ON CONFLICT (user_id) DO UPDATE SET
		  desired_retention=EXCLUDED.desired_retention,
		  daily_new_limit=EXCLUDED.daily_new_limit,
		  daily_review_limit=EXCLUDED.daily_review_limit,
		  timezone=EXCLUDED.timezone,
		  updated_at=now()`,
		p.UserID, p.DesiredRetention, p.DailyNewLimit, p.DailyReviewLimit, p.Timezone)
	return err
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/scheduling/repo/ -run 'CardStore|PrefsStore' -v`
Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/scheduling/repo
git commit -m "feat(scheduling): pgx CardStore + PrefsStore adapters (owner_id,due_at hot path)"
```

---

### Task 7: scheduling/service — prefs use case (validate retention) (TDD)

**Files:**
- Create: `internal/scheduling/service/prefs.go`
- Test: `internal/scheduling/service/prefs_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/scheduling/service/prefs_test.go`:
```go
package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/service"
	"github.com/stretchr/testify/require"
)

type fakePrefs struct{ saved domain.SchedulerPrefs }

func (f *fakePrefs) Get(_ context.Context, _ db.Querier, uid uuid.UUID) (domain.SchedulerPrefs, error) {
	if f.saved.UserID == uid {
		return f.saved, nil
	}
	p := domain.DefaultPrefs()
	p.UserID = uid
	return p, nil
}
func (f *fakePrefs) Upsert(_ context.Context, _ db.Querier, p domain.SchedulerPrefs) error {
	f.saved = p
	return nil
}

func TestPrefsService_UpdateRejectsOutOfRange(t *testing.T) {
	svc := service.NewPrefsService(nil, &fakePrefs{})
	_, err := svc.Update(context.Background(), uuid.New(), service.PrefsUpdate{DesiredRetention: 0.5, Timezone: "UTC"})
	require.ErrorIs(t, err, service.ErrRetentionRange)
}

func TestPrefsService_UpdateRejectsBadTimezone(t *testing.T) {
	svc := service.NewPrefsService(nil, &fakePrefs{})
	_, err := svc.Update(context.Background(), uuid.New(), service.PrefsUpdate{DesiredRetention: 0.9, Timezone: "Mars/Phobos"})
	require.ErrorIs(t, err, service.ErrBadTimezone)
}

func TestPrefsService_UpdatePersists(t *testing.T) {
	fp := &fakePrefs{}
	svc := service.NewPrefsService(nil, fp)
	uid := uuid.New()
	got, err := svc.Update(context.Background(), uid, service.PrefsUpdate{
		DesiredRetention: 0.85, DailyNewLimit: 30, DailyReviewLimit: 150, Timezone: "Asia/Bangkok",
	})
	require.NoError(t, err)
	require.InDelta(t, 0.85, got.DesiredRetention, 1e-9)
	require.Equal(t, "Asia/Bangkok", fp.saved.Timezone)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scheduling/service/ -v`
Expected: FAIL (`service.NewPrefsService` chưa có).

- [ ] **Step 3: Write prefs.go**

Create `internal/scheduling/service/prefs.go`:
```go
package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/ports"
)

var (
	ErrRetentionRange = errors.New("desired_retention must be within [0.80, 0.97]")
	ErrBadTimezone    = errors.New("invalid IANA timezone")
)

// PrefsUpdate = input cập nhật cấu hình lịch (FR-17, FR-26).
type PrefsUpdate struct {
	DesiredRetention float64
	DailyNewLimit    int
	DailyReviewLimit int
	Timezone         string
}

// PrefsService quản cấu hình lịch. pool có thể nil trong unit test (fake store bỏ qua Querier).
type PrefsService struct {
	pool  *pgxpool.Pool
	store ports.PrefsStore
}

func NewPrefsService(pool *pgxpool.Pool, store ports.PrefsStore) *PrefsService {
	return &PrefsService{pool: pool, store: store}
}

func (s *PrefsService) querier() (q interface{ any }) { return nil } // placeholder-free helper below

func (s *PrefsService) Get(ctx context.Context, userID uuid.UUID) (domain.SchedulerPrefs, error) {
	return s.store.Get(ctx, poolOrNil(s.pool), userID)
}

func (s *PrefsService) Update(ctx context.Context, userID uuid.UUID, in PrefsUpdate) (domain.SchedulerPrefs, error) {
	if !domain.RetentionInRange(in.DesiredRetention) {
		return domain.SchedulerPrefs{}, ErrRetentionRange
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return domain.SchedulerPrefs{}, ErrBadTimezone
	}
	p := domain.SchedulerPrefs{
		UserID:           userID,
		DesiredRetention: in.DesiredRetention,
		DailyNewLimit:    in.DailyNewLimit,
		DailyReviewLimit: in.DailyReviewLimit,
		Timezone:         in.Timezone,
	}
	if err := s.store.Upsert(ctx, poolOrNil(s.pool), p); err != nil {
		return domain.SchedulerPrefs{}, err
	}
	return p, nil
}
```

Create `internal/scheduling/service/querier.go`:
```go
package service

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/memorix/memorix/internal/platform/db"
)

// poolOrNil trả *pgxpool.Pool như db.Querier, hoặc nil khi pool nil (unit test).
func poolOrNil(p *pgxpool.Pool) db.Querier {
	if p == nil {
		return nil
	}
	return p
}
```

> Xóa method `querier()` thừa: sau khi tạo file trên, mở `prefs.go` và xóa dòng `func (s *PrefsService) querier() ...` — nó không được dùng (giữ code sạch, không placeholder).

- [ ] **Step 4: Remove the unused helper line**

Edit `internal/scheduling/service/prefs.go` — xóa nguyên dòng:
```go
func (s *PrefsService) querier() (q interface{ any }) { return nil } // placeholder-free helper below
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/scheduling/service/ -v`
Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/scheduling/service
git commit -m "feat(scheduling): prefs service with retention range + timezone validation (FR-17)"
```

---

### Task 8: scheduling/handler — GET/PUT /api/v1/scheduler/prefs (TDD httptest)

Handler chỉ bind/validate → service (AD-2). Principal lấy qua hàm inject (test dùng fake, cmd dùng authmw).

**Files:**
- Create: `internal/scheduling/handler/prefs.go`
- Test: `internal/scheduling/handler/prefs_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/scheduling/handler/prefs_test.go`:
```go
package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/handler"
	"github.com/memorix/memorix/internal/scheduling/service"
	"github.com/stretchr/testify/require"
)

type fakePrefs struct{ saved domain.SchedulerPrefs }

func (f *fakePrefs) Get(_ context.Context, _ db.Querier, uid uuid.UUID) (domain.SchedulerPrefs, error) {
	p := domain.DefaultPrefs()
	p.UserID = uid
	return p, nil
}
func (f *fakePrefs) Upsert(_ context.Context, _ db.Querier, p domain.SchedulerPrefs) error {
	f.saved = p
	return nil
}

func newRouter(owner uuid.UUID, ps *service.PrefsService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewPrefsHandler(ps, func(*gin.Context) (uuid.UUID, error) { return owner, nil })
	g := r.Group("/api/v1")
	h.Register(g)
	return r
}

func TestPutPrefs_OK(t *testing.T) {
	owner := uuid.New()
	ps := service.NewPrefsService(nil, &fakePrefs{})
	r := newRouter(owner, ps)

	body := `{"desired_retention":0.85,"daily_new_limit":30,"daily_review_limit":150,"timezone":"Asia/Bangkok"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/scheduler/prefs", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, 0.85, got["desired_retention"])
}

func TestPutPrefs_RejectsBadRetention(t *testing.T) {
	owner := uuid.New()
	ps := service.NewPrefsService(nil, &fakePrefs{})
	r := newRouter(owner, ps)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/scheduler/prefs",
		strings.NewReader(`{"desired_retention":0.5,"timezone":"UTC"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scheduling/handler/ -v`
Expected: FAIL (`handler.NewPrefsHandler` chưa có).

- [ ] **Step 3: Write handler**

Create `internal/scheduling/handler/prefs.go`:
```go
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/scheduling/service"
)

// PrincipalFunc trích owner id từ request (authmw ở cmd; fake trong test).
type PrincipalFunc func(*gin.Context) (uuid.UUID, error)

type PrefsHandler struct {
	svc   *service.PrefsService
	owner PrincipalFunc
}

func NewPrefsHandler(svc *service.PrefsService, owner PrincipalFunc) *PrefsHandler {
	return &PrefsHandler{svc: svc, owner: owner}
}

func (h *PrefsHandler) Register(g *gin.RouterGroup) {
	g.GET("/scheduler/prefs", h.get)
	g.PUT("/scheduler/prefs", h.put)
}

type prefsBody struct {
	DesiredRetention float64 `json:"desired_retention"`
	DailyNewLimit    int     `json:"daily_new_limit"`
	DailyReviewLimit int     `json:"daily_review_limit"`
	Timezone         string  `json:"timezone"`
}

type prefsResp struct {
	DesiredRetention float64 `json:"desired_retention"`
	DailyNewLimit    int     `json:"daily_new_limit"`
	DailyReviewLimit int     `json:"daily_review_limit"`
	Timezone         string  `json:"timezone"`
}

func (h *PrefsHandler) get(c *gin.Context) {
	owner, err := h.owner(c)
	if err != nil {
		writeErr(c, httpx.NewError(httpx.CodeUnauthenticated, "unauthenticated"))
		return
	}
	p, err := h.svc.Get(c.Request.Context(), owner)
	if err != nil {
		writeErr(c, httpx.NewError(httpx.CodeInternal, err.Error()))
		return
	}
	c.JSON(http.StatusOK, prefsResp(p2resp(p.DesiredRetention, p.DailyNewLimit, p.DailyReviewLimit, p.Timezone)))
}

func (h *PrefsHandler) put(c *gin.Context) {
	owner, err := h.owner(c)
	if err != nil {
		writeErr(c, httpx.NewError(httpx.CodeUnauthenticated, "unauthenticated"))
		return
	}
	var b prefsBody
	if err := c.ShouldBindJSON(&b); err != nil {
		writeErr(c, httpx.NewError(httpx.CodeValidation, "invalid body"))
		return
	}
	if b.DailyNewLimit == 0 {
		b.DailyNewLimit = 20
	}
	if b.DailyReviewLimit == 0 {
		b.DailyReviewLimit = 200
	}
	p, err := h.svc.Update(c.Request.Context(), owner, service.PrefsUpdate{
		DesiredRetention: b.DesiredRetention,
		DailyNewLimit:    b.DailyNewLimit,
		DailyReviewLimit: b.DailyReviewLimit,
		Timezone:         b.Timezone,
	})
	switch {
	case errors.Is(err, service.ErrRetentionRange):
		writeErr(c, httpx.NewError(httpx.CodeValidation, err.Error()).WithField("desired_retention", "0.80–0.97"))
		return
	case errors.Is(err, service.ErrBadTimezone):
		writeErr(c, httpx.NewError(httpx.CodeValidation, err.Error()).WithField("timezone", "IANA tz"))
		return
	case err != nil:
		writeErr(c, httpx.NewError(httpx.CodeInternal, err.Error()))
		return
	}
	c.JSON(http.StatusOK, prefsResp(p2resp(p.DesiredRetention, p.DailyNewLimit, p.DailyReviewLimit, p.Timezone)))
}

func p2resp(r float64, dn, dr int, tz string) prefsResp {
	return prefsResp{DesiredRetention: r, DailyNewLimit: dn, DailyReviewLimit: dr, Timezone: tz}
}

func writeErr(c *gin.Context, e *httpx.APIError) {
	c.JSON(e.HTTPStatus(), e)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scheduling/handler/ -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/scheduling/handler
git commit -m "feat(scheduling): scheduler prefs HTTP endpoints (GET/PUT /api/v1/scheduler/prefs)"
```

---

### Task 9: review/domain + review/ports — grade command, log row, receipt, VocabularyPort (TDD)

`review` phụ thuộc `scheduling/ports` + `scheduling/domain` (cross-module qua PUBLIC ports — AD-1 cho phép; cấm chỉ là `internal/`).

**Files:**
- Create: `internal/review/domain/grade.go`
- Create: `internal/review/ports/ports.go`
- Test: `internal/review/domain/grade_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/review/domain/grade_test.go`:
```go
package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/review/domain"
	"github.com/stretchr/testify/require"
)

func TestResultFromScheduleResult(t *testing.T) {
	cid := uuid.New()
	due := time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC)
	sr := scheddom.ScheduleResult{
		Stability: 12.5, Difficulty: 6, Status: scheddom.StatusReview,
		Reps: 1, Lapses: 0, DueAt: due,
	}
	got := domain.ResultFromSchedule(cid, sr)
	require.Equal(t, cid, got.CardID)
	require.InDelta(t, 12.5, got.Stability, 1e-9)
	require.Equal(t, scheddom.StatusReview, got.Status)
	require.Equal(t, due, got.DueAt)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/review/domain/ -v`
Expected: FAIL (package chưa có).

- [ ] **Step 3: Write grade.go**

Create `internal/review/domain/grade.go`:
```go
package domain

import (
	"time"

	"github.com/google/uuid"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
)

// GradeCommand = payload duy nhất client gửi (AD-5). KHÔNG có S/D/Due.
type GradeCommand struct {
	CardID         uuid.UUID
	Grade          scheddom.Grade
	ClientReviewID string
}

// GradeResult = trạng thái card sau chấm, trả cho client + lưu ở grade_receipts.
type GradeResult struct {
	CardID     uuid.UUID
	Stability  float64
	Difficulty float64
	Status     scheddom.CardStatus
	Reps       int
	Lapses     int
	DueAt      time.Time
}

func ResultFromSchedule(cardID uuid.UUID, r scheddom.ScheduleResult) GradeResult {
	return GradeResult{
		CardID: cardID, Stability: r.Stability, Difficulty: r.Difficulty,
		Status: r.Status, Reps: r.Reps, Lapses: r.Lapses, DueAt: r.DueAt,
	}
}

// ReviewLogRow = một dòng append-only ở review.review_logs (AD-4 replay source).
type ReviewLogRow struct {
	ID             uuid.UUID
	CardID         uuid.UUID
	OwnerID        uuid.UUID
	ClientReviewID string
	Grade          scheddom.Grade
	PrevStability  float64
	PrevDifficulty float64
	PrevStatus     scheddom.CardStatus
	Retrievability float64
	NewStability   float64
	NewDifficulty  float64
	NewStatus      scheddom.CardStatus
	NewReps        int
	NewLapses      int
	NewDueAt       time.Time
	ElapsedDays    int
	ReviewedAt     time.Time
}
```

- [ ] **Step 4: Write review/ports**

Create `internal/review/ports/ports.go`:
```go
package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/db"
	revdom "github.com/memorix/memorix/internal/review/domain"
)

// ReviewLogRepo append + đọc log (AD-4).
type ReviewLogRepo interface {
	Append(ctx context.Context, q db.Querier, row revdom.ReviewLogRow) error
	// ListForOwnerSince trả log của owner từ mốc `sinceRFC3339` (dùng cho summary + replay).
	ListForOwnerSince(ctx context.Context, q db.Querier, ownerID uuid.UUID, sinceRFC3339 string) ([]revdom.ReviewLogRow, error)
	// ListForCard trả log 1 card theo thứ tự reviewed_at tăng dần (replay AD-4).
	ListForCard(ctx context.Context, q db.Querier, cardID uuid.UUID) ([]revdom.ReviewLogRow, error)
}

// ReceiptRepo = idempotency guard (AD-3): unique(card_id, client_review_id).
type ReceiptRepo interface {
	// Insert trả (true) nếu chèn mới, (false) nếu đã tồn tại (ON CONFLICT DO NOTHING).
	Insert(ctx context.Context, q db.Querier, r revdom.GradeResult, reviewLogID uuid.UUID, clientReviewID string) (bool, error)
	// Get trả kết quả cũ để idempotent-return; ok=false nếu chưa có.
	Get(ctx context.Context, q db.Querier, cardID uuid.UUID, clientReviewID string) (revdom.GradeResult, bool, error)
}

// EntryContent = nội dung entry batch-load qua VocabularyPort (AD-9). Chỉ field
// cần cho mặt sau thẻ; review KHÔNG join bảng vocabulary.
type EntryContent struct {
	EntryID uuid.UUID
	Term    string
	IPA     string
	Meaning string
	Example string
}

// VocabularyPort = port chéo module (định nghĩa ở caller, addendum §chống import cycle).
type VocabularyPort interface {
	BatchGet(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]EntryContent, error)
}

// VocabularyFunc adapter hàm → VocabularyPort (cmd wiring bọc port thật của vocabulary).
type VocabularyFunc func(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]EntryContent, error)

func (f VocabularyFunc) BatchGet(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]EntryContent, error) {
	return f(ctx, ownerID, entryIDs)
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/review/domain/ -v && go build ./internal/review/...`
Expected: PASS + build sạch.

- [ ] **Step 6: Commit**

```bash
git add internal/review/domain internal/review/ports
git commit -m "feat(review): grade command/result, append-only log row, receipt + VocabularyPort (AD-3,4,9)"
```

---

### Task 10: review/repo — pgx ReviewLogRepo + ReceiptRepo (TDD, testcontainers)

**Files:**
- Create: `internal/review/repo/reviewlog.go`
- Create: `internal/review/repo/receipt.go`
- Test: `internal/review/repo/repo_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/review/repo/repo_test.go`:
```go
package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db/dbtest"
	revdom "github.com/memorix/memorix/internal/review/domain"
	"github.com/memorix/memorix/internal/review/repo"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/stretchr/testify/require"
)

func sampleRow(owner, card uuid.UUID, at time.Time) revdom.ReviewLogRow {
	return revdom.ReviewLogRow{
		ID: uuid.New(), CardID: card, OwnerID: owner, ClientReviewID: "cr-" + at.String(),
		Grade: scheddom.GradeGood, PrevStability: 0, PrevDifficulty: 0, PrevStatus: scheddom.StatusNew,
		Retrievability: 0.9, NewStability: 5, NewDifficulty: 5, NewStatus: scheddom.StatusReview,
		NewReps: 1, NewLapses: 0, NewDueAt: at.AddDate(0, 0, 5), ElapsedDays: 0, ReviewedAt: at,
	}
}

func TestReviewLog_AppendAndList(t *testing.T) {
	pool := dbtest.NewPostgres(t)
	ctx := context.Background()
	owner, card := uuid.New(), uuid.New()
	lr := repo.NewReviewLogRepo()

	t0 := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	require.NoError(t, lr.Append(ctx, pool, sampleRow(owner, card, t0)))
	require.NoError(t, lr.Append(ctx, pool, sampleRow(owner, card, t0.Add(time.Minute))))

	byCard, err := lr.ListForCard(ctx, pool, card)
	require.NoError(t, err)
	require.Len(t, byCard, 2)
	require.True(t, !byCard[1].ReviewedAt.Before(byCard[0].ReviewedAt), "phải sort tăng dần")

	sinceStart := t0.Add(-time.Hour).Format(time.RFC3339Nano)
	byOwner, err := lr.ListForOwnerSince(ctx, pool, owner, sinceStart)
	require.NoError(t, err)
	require.Len(t, byOwner, 2)
}

func TestReceipt_InsertIdempotentAndGet(t *testing.T) {
	pool := dbtest.NewPostgres(t)
	ctx := context.Background()
	card := uuid.New()
	rr := repo.NewReceiptRepo()

	res := revdom.GradeResult{
		CardID: card, Stability: 7, Difficulty: 5, Status: scheddom.StatusReview,
		Reps: 1, Lapses: 0, DueAt: time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC),
	}
	inserted, err := rr.Insert(ctx, pool, res, uuid.New(), "cr-1")
	require.NoError(t, err)
	require.True(t, inserted)

	// trùng → không chèn
	inserted2, err := rr.Insert(ctx, pool, res, uuid.New(), "cr-1")
	require.NoError(t, err)
	require.False(t, inserted2)

	got, ok, err := rr.Get(ctx, pool, card, "cr-1")
	require.NoError(t, err)
	require.True(t, ok)
	require.InDelta(t, 7, got.Stability, 1e-9)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/review/repo/ -v`
Expected: FAIL (`repo.NewReviewLogRepo`/`NewReceiptRepo` chưa có).

- [ ] **Step 3: Write reviewlog.go**

Create `internal/review/repo/reviewlog.go`:
```go
package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/memorix/memorix/internal/platform/db"
	revdom "github.com/memorix/memorix/internal/review/domain"
)

type ReviewLogRepo struct{}

func NewReviewLogRepo() *ReviewLogRepo { return &ReviewLogRepo{} }

const logCols = `id, card_id, owner_id, client_review_id, grade,
	prev_stability, prev_difficulty, prev_status, retrievability,
	new_stability, new_difficulty, new_status, new_reps, new_lapses, new_due_at,
	elapsed_days, reviewed_at`

func (r *ReviewLogRepo) Append(ctx context.Context, q db.Querier, row revdom.ReviewLogRow) error {
	_, err := q.Exec(ctx, `
		INSERT INTO review.review_logs (`+logCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		row.ID, row.CardID, row.OwnerID, row.ClientReviewID, row.Grade,
		row.PrevStability, row.PrevDifficulty, row.PrevStatus, row.Retrievability,
		row.NewStability, row.NewDifficulty, row.NewStatus, row.NewReps, row.NewLapses, row.NewDueAt,
		row.ElapsedDays, row.ReviewedAt)
	return err
}

func scanLog(rows pgx.Rows) (revdom.ReviewLogRow, error) {
	var x revdom.ReviewLogRow
	err := rows.Scan(&x.ID, &x.CardID, &x.OwnerID, &x.ClientReviewID, &x.Grade,
		&x.PrevStability, &x.PrevDifficulty, &x.PrevStatus, &x.Retrievability,
		&x.NewStability, &x.NewDifficulty, &x.NewStatus, &x.NewReps, &x.NewLapses, &x.NewDueAt,
		&x.ElapsedDays, &x.ReviewedAt)
	return x, err
}

func collect(rows pgx.Rows) ([]revdom.ReviewLogRow, error) {
	defer rows.Close()
	var out []revdom.ReviewLogRow
	for rows.Next() {
		x, err := scanLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *ReviewLogRepo) ListForCard(ctx context.Context, q db.Querier, cardID uuid.UUID) ([]revdom.ReviewLogRow, error) {
	rows, err := q.Query(ctx, `SELECT `+logCols+`
		FROM review.review_logs WHERE card_id=$1 ORDER BY reviewed_at ASC`, cardID)
	if err != nil {
		return nil, err
	}
	return collect(rows)
}

func (r *ReviewLogRepo) ListForOwnerSince(ctx context.Context, q db.Querier, ownerID uuid.UUID, sinceRFC3339 string) ([]revdom.ReviewLogRow, error) {
	rows, err := q.Query(ctx, `SELECT `+logCols+`
		FROM review.review_logs
		WHERE owner_id=$1 AND reviewed_at >= $2::timestamptz
		ORDER BY reviewed_at ASC`, ownerID, sinceRFC3339)
	if err != nil {
		return nil, err
	}
	return collect(rows)
}
```

- [ ] **Step 4: Write receipt.go**

Create `internal/review/repo/receipt.go`:
```go
package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/memorix/memorix/internal/platform/db"
	revdom "github.com/memorix/memorix/internal/review/domain"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
)

type ReceiptRepo struct{}

func NewReceiptRepo() *ReceiptRepo { return &ReceiptRepo{} }

func (r *ReceiptRepo) Insert(ctx context.Context, q db.Querier, res revdom.GradeResult, reviewLogID uuid.UUID, clientReviewID string) (bool, error) {
	var out uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO review.grade_receipts
		  (card_id, client_review_id, review_log_id, new_stability, new_difficulty,
		   new_status, new_reps, new_lapses, new_due_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (card_id, client_review_id) DO NOTHING
		RETURNING card_id`,
		res.CardID, clientReviewID, reviewLogID, res.Stability, res.Difficulty,
		res.Status, res.Reps, res.Lapses, res.DueAt).Scan(&out)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // đã tồn tại
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *ReceiptRepo) Get(ctx context.Context, q db.Querier, cardID uuid.UUID, clientReviewID string) (revdom.GradeResult, bool, error) {
	var res revdom.GradeResult
	res.CardID = cardID
	var status int16
	err := q.QueryRow(ctx, `
		SELECT new_stability, new_difficulty, new_status, new_reps, new_lapses, new_due_at
		FROM review.grade_receipts WHERE card_id=$1 AND client_review_id=$2`,
		cardID, clientReviewID).Scan(&res.Stability, &res.Difficulty, &status, &res.Reps, &res.Lapses, &res.DueAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return revdom.GradeResult{}, false, nil
	}
	if err != nil {
		return revdom.GradeResult{}, false, err
	}
	res.Status = scheddom.CardStatus(status)
	return res, true, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/review/repo/ -v`
Expected: PASS (2 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/review/repo
git commit -m "feat(review): pgx ReviewLogRepo (append-only) + ReceiptRepo (idempotency guard)"
```

---

### Task 11: review/service — GradeService (atomic + idempotent + server-authoritative + event) (TDD)

Trái tim Story 3.1. 1 TX: guard receipt → append log → update card. Idempotent qua receipt. Phát `CardGraded` chỉ khi chấm mới (không double-count khi retry).

**Files:**
- Create: `internal/review/service/grade.go`
- Test: `internal/review/service/grade_test.go` (unit, fakes)

- [ ] **Step 1: Write the failing test**

Create `internal/review/service/grade_test.go`:
```go
package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/platform/eventbus"
	revdom "github.com/memorix/memorix/internal/review/domain"
	"github.com/memorix/memorix/internal/review/service"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeSched struct{}

func (fakeSched) Apply(c scheddom.Card, g scheddom.Grade, r float64, now time.Time) scheddom.ScheduleResult {
	return scheddom.ScheduleResult{Stability: 5, Difficulty: 5, Status: scheddom.StatusReview,
		Reps: c.Reps + 1, Lapses: c.Lapses, DueAt: now.AddDate(0, 0, 5), LastReviewAt: now, Retrievability: 0.9}
}
func (fakeSched) Preview(scheddom.Card, float64, time.Time) scheddom.NextIntervals {
	return scheddom.NextIntervals{}
}

type fakeCards struct{ applied int }

func (c *fakeCards) Load(_ context.Context, _ db.Querier, cardID, owner uuid.UUID) (scheddom.Card, error) {
	return scheddom.Card{ID: cardID, OwnerID: owner, Status: scheddom.StatusNew}, nil
}
func (c *fakeCards) ApplyResult(_ context.Context, _ db.Querier, _ uuid.UUID, _ scheddom.ScheduleResult) error {
	c.applied++
	return nil
}
func (c *fakeCards) DueCards(context.Context, db.Querier, uuid.UUID, time.Time, int) ([]scheddom.Card, error) {
	return nil, nil
}

type fakePrefs struct{}

func (fakePrefs) Get(context.Context, db.Querier, uuid.UUID) (scheddom.SchedulerPrefs, error) {
	return scheddom.DefaultPrefs(), nil
}
func (fakePrefs) Upsert(context.Context, db.Querier, scheddom.SchedulerPrefs) error { return nil }

type fakeLogs struct {
	mu   sync.Mutex
	rows []revdom.ReviewLogRow
}

func (l *fakeLogs) Append(_ context.Context, _ db.Querier, row revdom.ReviewLogRow) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rows = append(l.rows, row)
	return nil
}
func (l *fakeLogs) ListForOwnerSince(context.Context, db.Querier, uuid.UUID, string) ([]revdom.ReviewLogRow, error) {
	return nil, nil
}
func (l *fakeLogs) ListForCard(context.Context, db.Querier, uuid.UUID) ([]revdom.ReviewLogRow, error) {
	return nil, nil
}

type fakeReceipts struct {
	mu    sync.Mutex
	store map[string]revdom.GradeResult
}

func newFakeReceipts() *fakeReceipts { return &fakeReceipts{store: map[string]revdom.GradeResult{}} }
func key(card uuid.UUID, cr string) string { return card.String() + "|" + cr }

func (r *fakeReceipts) Insert(_ context.Context, _ db.Querier, res revdom.GradeResult, _ uuid.UUID, cr string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(res.CardID, cr)
	if _, ok := r.store[k]; ok {
		return false, nil
	}
	r.store[k] = res
	return true, nil
}
func (r *fakeReceipts) Get(_ context.Context, _ db.Querier, card uuid.UUID, cr string) (revdom.GradeResult, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	res, ok := r.store[key(card, cr)]
	return res, ok, nil
}

// txRunner fake: chạy fn ngay với Querier=nil (không cần DB thật cho unit test).
func directTx(_ context.Context, fn func(db.Querier) error) error { return fn(nil) }

func newSvc(bus *eventbus.InProcess) (*service.GradeService, *fakeCards, *fakeLogs) {
	cards := &fakeCards{}
	logs := &fakeLogs{}
	svc := service.NewGradeService(service.GradeDeps{
		Tx: directTx, Scheduler: fakeSched{}, Cards: cards, Prefs: fakePrefs{},
		Logs: logs, Receipts: newFakeReceipts(), Bus: bus,
		Clock: func() time.Time { return time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC) },
	})
	return svc, cards, logs
}

func TestGrade_ServerComputesAndPersistsOnce(t *testing.T) {
	bus := eventbus.NewInProcess()
	var mu sync.Mutex
	events := 0
	bus.Subscribe("CardGraded", func(context.Context, eventbus.Event) { mu.Lock(); events++; mu.Unlock() })

	svc, cards, logs := newSvc(bus)
	cmd := revdom.GradeCommand{CardID: uuid.New(), Grade: scheddom.GradeGood, ClientReviewID: "cr-1"}
	res, err := svc.Grade(context.Background(), uuid.New(), cmd)
	require.NoError(t, err)
	require.InDelta(t, 5, res.Stability, 1e-9) // server tính, không nhận từ client
	require.Equal(t, 1, cards.applied)
	require.Len(t, logs.rows, 1)
	bus.Wait()
	require.Equal(t, 1, events)
}

func TestGrade_IdempotentOnDuplicateClientReviewID(t *testing.T) {
	bus := eventbus.NewInProcess()
	var mu sync.Mutex
	events := 0
	bus.Subscribe("CardGraded", func(context.Context, eventbus.Event) { mu.Lock(); events++; mu.Unlock() })

	svc, cards, logs := newSvc(bus)
	owner := uuid.New()
	cmd := revdom.GradeCommand{CardID: uuid.New(), Grade: scheddom.GradeGood, ClientReviewID: "cr-dup"}

	r1, err := svc.Grade(context.Background(), owner, cmd)
	require.NoError(t, err)
	r2, err := svc.Grade(context.Background(), owner, cmd) // gửi lại y hệt
	require.NoError(t, err)

	require.Equal(t, r1, r2, "trả kết quả cũ")
	require.Equal(t, 1, cards.applied, "KHÔNG chấm lại card")
	require.Len(t, logs.rows, 1, "KHÔNG tạo log trùng (AD-4)")
	bus.Wait()
	require.Equal(t, 1, events, "KHÔNG phát event lần hai")
}

func TestGrade_RejectsInvalidGrade(t *testing.T) {
	svc, _, _ := newSvc(eventbus.NewInProcess())
	_, err := svc.Grade(context.Background(), uuid.New(),
		revdom.GradeCommand{CardID: uuid.New(), Grade: scheddom.Grade(9), ClientReviewID: "x"})
	require.ErrorIs(t, err, service.ErrInvalidGrade)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/review/service/ -run TestGrade -v`
Expected: FAIL (`service.NewGradeService` chưa có).

- [ ] **Step 3: Write grade.go**

Create `internal/review/service/grade.go`:
```go
package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/platform/eventbus"
	revdom "github.com/memorix/memorix/internal/review/domain"
	revports "github.com/memorix/memorix/internal/review/ports"
	schedports "github.com/memorix/memorix/internal/scheduling/ports"
)

var ErrInvalidGrade = errors.New("grade must be 1..4")

// TxRunner chạy fn trong 1 transaction (db.WithinTx bọc pool ở cmd; fake trong test).
type TxRunner func(ctx context.Context, fn func(db.Querier) error) error

// GradeDeps gom phụ thuộc để wiring rõ ràng (S6).
type GradeDeps struct {
	Tx        TxRunner
	Scheduler schedports.SchedulerPort
	Cards     schedports.CardStore
	Prefs     schedports.PrefsStore
	Logs      revports.ReviewLogRepo
	Receipts  revports.ReceiptRepo
	Bus       eventbus.Bus
	Clock     func() time.Time
}

type GradeService struct{ d GradeDeps }

func NewGradeService(d GradeDeps) *GradeService {
	if d.Clock == nil {
		d.Clock = time.Now
	}
	return &GradeService{d: d}
}

// CardGradedPayload đi kèm event CardGraded (progress read model đọc — AD-8).
type CardGradedPayload struct {
	CardID     uuid.UUID
	OwnerID    uuid.UUID
	Grade      int
	ReviewedAt time.Time
}

// Grade: server-authoritative (AD-5), nguyên tử (AD-3), idempotent (FR-15), append-only (AD-4).
func (s *GradeService) Grade(ctx context.Context, ownerID uuid.UUID, cmd revdom.GradeCommand) (revdom.GradeResult, error) {
	if !cmd.Grade.Valid() {
		return revdom.GradeResult{}, ErrInvalidGrade
	}
	now := s.d.Clock()
	var (
		result revdom.GradeResult
		fresh  bool
	)

	err := s.d.Tx(ctx, func(q db.Querier) error {
		// 1. retry tuần tự: đã có receipt → trả kết quả cũ, không làm gì thêm.
		if prev, ok, err := s.d.Receipts.Get(ctx, q, cmd.CardID, cmd.ClientReviewID); err != nil {
			return err
		} else if ok {
			result = prev
			return nil
		}
		// 2. load card (ownership check) + prefs.
		card, err := s.d.Cards.Load(ctx, q, cmd.CardID, ownerID)
		if err != nil {
			return err
		}
		prefs, err := s.d.Prefs.Get(ctx, q, ownerID)
		if err != nil {
			return err
		}
		// 3. server tính S/D/Due (AD-5, AD-7).
		out := s.d.Scheduler.Apply(card, cmd.Grade, prefs.DesiredRetention, now)
		res := revdom.ResultFromSchedule(cmd.CardID, out)
		logID := uuid.New()

		// 4. guard idempotency TRƯỚC khi append (chống race đa thiết bị).
		inserted, err := s.d.Receipts.Insert(ctx, q, res, logID, cmd.ClientReviewID)
		if err != nil {
			return err
		}
		if !inserted {
			prev, _, err := s.d.Receipts.Get(ctx, q, cmd.CardID, cmd.ClientReviewID)
			if err != nil {
				return err
			}
			result = prev
			return nil
		}
		// 5. append log (AD-4) + update card (AD-3) — cùng TX.
		if err := s.d.Logs.Append(ctx, q, revdom.ReviewLogRow{
			ID: logID, CardID: cmd.CardID, OwnerID: ownerID, ClientReviewID: cmd.ClientReviewID,
			Grade: cmd.Grade, PrevStability: card.Stability, PrevDifficulty: card.Difficulty,
			PrevStatus: card.Status, Retrievability: out.Retrievability,
			NewStability: out.Stability, NewDifficulty: out.Difficulty, NewStatus: out.Status,
			NewReps: out.Reps, NewLapses: out.Lapses, NewDueAt: out.DueAt,
			ElapsedDays: out.ElapsedDays, ReviewedAt: now,
		}); err != nil {
			return err
		}
		if err := s.d.Cards.ApplyResult(ctx, q, cmd.CardID, out); err != nil {
			return err
		}
		result = res
		fresh = true
		return nil
	})
	if err != nil {
		return revdom.GradeResult{}, err
	}

	// 6. phát event NGOÀI TX chấm (AD-8), chỉ khi chấm mới.
	if fresh && s.d.Bus != nil {
		s.d.Bus.Publish(ctx, eventbus.Event{Name: "CardGraded", Payload: CardGradedPayload{
			CardID: cmd.CardID, OwnerID: ownerID, Grade: int(cmd.Grade), ReviewedAt: now,
		}})
	}
	return result, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/review/service/ -run TestGrade -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/review/service/grade.go internal/review/service/grade_test.go
git commit -m "feat(review): atomic idempotent server-authoritative GradeService + CardGraded event (AD-3,4,5,8)"
```

---

### Task 12: review/service — Grade integration + replay test (AD-4) (TDD, testcontainers)

Chứng minh nguyên tử thật trên Postgres + **replay `review_logs` tái tạo S/D/Due của card** (AD-4, NFR-6) — bài test vương miện.

**Files:**
- Test: `internal/review/service/grade_integration_test.go`

- [ ] **Step 1: Write the integration + replay test**

Create `internal/review/service/grade_integration_test.go`:
```go
package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/platform/eventbus"
	revdom "github.com/memorix/memorix/internal/review/domain"
	revrepo "github.com/memorix/memorix/internal/review/repo"
	"github.com/memorix/memorix/internal/review/service"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/repo"
	"github.com/memorix/memorix/internal/scheduling/repo/fsrsadapter"
	"github.com/stretchr/testify/require"
)

func insertNewCard(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner, entry uuid.UUID, due time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO scheduling.cards (id, owner_id, entry_id, direction, status, due_at, created_at, updated_at)
		VALUES ($1,$2,$3,'front_back',0,$4,now(),now())`, id, owner, entry, due)
	require.NoError(t, err)
	return id
}

func realService(pool *pgxpool.Pool, bus *eventbus.InProcess, now time.Time) *service.GradeService {
	return service.NewGradeService(service.GradeDeps{
		Tx:        func(ctx context.Context, fn func(db.Querier) error) error { return db.WithinTx(ctx, pool, fn) },
		Scheduler: fsrsadapter.New(),
		Cards:     repo.NewCardStore(),
		Prefs:     repo.NewPrefsStore(),
		Logs:      revrepo.NewReviewLogRepo(),
		Receipts:  revrepo.NewReceiptRepo(),
		Bus:       bus,
		Clock:     func() time.Time { return now },
	})
}

func TestGrade_AtomicIdempotentOnPostgres(t *testing.T) {
	pool := dbtest.NewPostgres(t)
	ctx := context.Background()
	owner, entry := uuid.New(), uuid.New()
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	cardID := insertNewCard(t, ctx, pool, owner, entry, now)

	svc := realService(pool, eventbus.NewInProcess(), now)
	cmd := revdom.GradeCommand{CardID: cardID, Grade: scheddom.GradeGood, ClientReviewID: "cr-1"}

	r1, err := svc.Grade(ctx, owner, cmd)
	require.NoError(t, err)
	require.Greater(t, r1.Stability, 0.0)

	r2, err := svc.Grade(ctx, owner, cmd) // idempotent
	require.NoError(t, err)
	require.Equal(t, r1, r2)

	var logCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM review.review_logs WHERE card_id=$1`, cardID).Scan(&logCount))
	require.Equal(t, 1, logCount, "chỉ 1 log dù chấm 2 lần")

	// card đã cập nhật đúng theo kết quả server-authoritative
	var dbStab float64
	require.NoError(t, pool.QueryRow(ctx, `SELECT stability FROM scheduling.cards WHERE id=$1`, cardID).Scan(&dbStab))
	require.InDelta(t, r1.Stability, dbStab, 1e-9)
}

// TestReplay_ReproducesCardState: replay chuỗi review_logs từ NewCard qua SchedulerPort
// phải tái tạo đúng S/D/Due/status/reps/lapses của card hiện tại (AD-4, NFR-6).
func TestReplay_ReproducesCardState(t *testing.T) {
	pool := dbtest.NewPostgres(t)
	ctx := context.Background()
	owner, entry := uuid.New(), uuid.New()
	start := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	cardID := insertNewCard(t, ctx, pool, owner, entry, start)

	// chấm nhiều lần ở các mốc thời gian khác nhau (mô phỏng ôn thật).
	grades := []struct {
		g  scheddom.Grade
		at time.Time
	}{
		{scheddom.GradeGood, start},
		{scheddom.GradeGood, start.AddDate(0, 0, 3)},
		{scheddom.GradeAgain, start.AddDate(0, 0, 10)},
		{scheddom.GradeHard, start.AddDate(0, 0, 11)},
		{scheddom.GradeEasy, start.AddDate(0, 0, 20)},
	}
	for i, step := range grades {
		svc := realService(pool, eventbus.NewInProcess(), step.at)
		_, err := svc.Grade(ctx, owner, revdom.GradeCommand{
			CardID: cardID, Grade: step.g, ClientReviewID: uuidStr(i),
		})
		require.NoError(t, err)
	}

	// đọc trạng thái card hiện tại
	cs := repo.NewCardStore()
	final, err := cs.Load(ctx, pool, cardID, owner)
	require.NoError(t, err)

	// REPLAY: dựng lại từ NewCard, áp từng grade theo reviewed_at.
	logs, err := revrepo.NewReviewLogRepo().ListForCard(ctx, pool, cardID)
	require.NoError(t, err)
	require.Len(t, logs, len(grades))

	sched := fsrsadapter.New()
	replayed := scheddom.Card{ID: cardID, OwnerID: owner, EntryID: entry, Status: scheddom.StatusNew, DueAt: start}
	for _, lg := range logs {
		out := sched.Apply(replayed, lg.Grade, 0.90, lg.ReviewedAt)
		last := out.LastReviewAt
		replayed.Stability = out.Stability
		replayed.Difficulty = out.Difficulty
		replayed.Status = out.Status
		replayed.Reps = out.Reps
		replayed.Lapses = out.Lapses
		replayed.DueAt = out.DueAt
		replayed.LastReviewAt = &last
	}

	require.InDelta(t, final.Stability, replayed.Stability, 1e-9, "replay S phải khớp")
	require.InDelta(t, final.Difficulty, replayed.Difficulty, 1e-9, "replay D phải khớp")
	require.Equal(t, final.Status, replayed.Status)
	require.Equal(t, final.Reps, replayed.Reps)
	require.Equal(t, final.Lapses, replayed.Lapses)
	require.WithinDuration(t, final.DueAt, replayed.DueAt, time.Second, "replay Due phải khớp (fuzz TẮT)")
}

func uuidStr(i int) string {
	return "replay-cr-" + time.Unix(int64(i), 0).UTC().Format("150405")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/review/service/ -run 'Atomic|Replay' -v`
Expected: FAIL nếu chạy trước khi mọi impl sẵn — nhưng ở điểm này mọi impl đã có, nên chạy để xác nhận PASS. Nếu cần thấy đỏ trước: tạm sửa replay loop bỏ `replayed.LastReviewAt = &last` → Due lệch → FAIL, rồi khôi phục.

- [ ] **Step 3: Run test to verify it passes**

Run: `go test ./internal/review/service/ -run 'Atomic|Replay' -v`
Expected: PASS (2 tests). Replay khớp chính xác nhờ fuzz TẮT (AD-4).

- [ ] **Step 4: Commit**

```bash
git add internal/review/service/grade_integration_test.go
git commit -m "test(review): atomic+idempotent on Postgres and review_logs replay reproduces card state (AD-4)"
```

---

### Task 13: review/service — QueueService (due + entry content + next_intervals) (TDD)

Story 3.3: due ≤ now, batch-load entry qua VocabularyPort (AD-9, không join hot path), mỗi thẻ kèm `next_intervals` 4 mức server tính (FR-14, AD-5).

**Files:**
- Create: `internal/review/service/queue.go`
- Test: `internal/review/service/queue_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/review/service/queue_test.go`:
```go
package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db"
	revports "github.com/memorix/memorix/internal/review/ports"
	"github.com/memorix/memorix/internal/review/service"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/stretchr/testify/require"
)

type queueCards struct{ cards []scheddom.Card }

func (q *queueCards) Load(context.Context, db.Querier, uuid.UUID, uuid.UUID) (scheddom.Card, error) {
	return scheddom.Card{}, nil
}
func (q *queueCards) ApplyResult(context.Context, db.Querier, uuid.UUID, scheddom.ScheduleResult) error {
	return nil
}
func (q *queueCards) DueCards(_ context.Context, _ db.Querier, _ uuid.UUID, _ time.Time, _ int) ([]scheddom.Card, error) {
	return q.cards, nil
}

type prevSched struct{}

func (prevSched) Apply(scheddom.Card, scheddom.Grade, float64, time.Time) scheddom.ScheduleResult {
	return scheddom.ScheduleResult{}
}
func (prevSched) Preview(_ scheddom.Card, _ float64, _ time.Time) scheddom.NextIntervals {
	return scheddom.NextIntervals{
		Again: 10 * time.Minute, Hard: 24 * time.Hour, Good: 4 * 24 * time.Hour, Easy: 9 * 24 * time.Hour,
	}
}

func TestQueue_BuildsItemsWithContentAndIntervals(t *testing.T) {
	owner := uuid.New()
	entry := uuid.New()
	card := scheddom.Card{ID: uuid.New(), OwnerID: owner, EntryID: entry, Direction: "front_back"}

	vocab := revports.VocabularyFunc(func(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]revports.EntryContent, error) {
		require.Equal(t, []uuid.UUID{entry}, ids, "batch-load đúng entry ids")
		return map[uuid.UUID]revports.EntryContent{
			entry: {EntryID: entry, Term: "ephemeral", IPA: "/ɪ'fem(ə)rəl/", Meaning: "chóng tàn", Example: "an ephemeral trend"},
		}, nil
	})

	svc := service.NewQueueService(service.QueueDeps{
		Pool: nil, RunQuery: func(_ context.Context, fn func(db.Querier) error) error { return fn(nil) },
		Cards: &queueCards{cards: []scheddom.Card{card}}, Prefs: fakePrefs{}, Scheduler: prevSched{}, Vocab: vocab,
		Clock: func() time.Time { return time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC) },
	})

	items, err := svc.Queue(context.Background(), owner, 50)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "ephemeral", items[0].Term)
	require.Equal(t, int64(600), items[0].NextIntervals.AgainSeconds) // 10 phút
	require.Equal(t, int64(9*86400), items[0].NextIntervals.EasySeconds)
}

func TestQueue_SkipsCardsWithMissingContent(t *testing.T) {
	owner := uuid.New()
	card := scheddom.Card{ID: uuid.New(), OwnerID: owner, EntryID: uuid.New()}
	vocab := revports.VocabularyFunc(func(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]revports.EntryContent, error) {
		return map[uuid.UUID]revports.EntryContent{}, nil // không có content
	})
	svc := service.NewQueueService(service.QueueDeps{
		RunQuery: func(_ context.Context, fn func(db.Querier) error) error { return fn(nil) },
		Cards:    &queueCards{cards: []scheddom.Card{card}}, Prefs: fakePrefs{}, Scheduler: prevSched{}, Vocab: vocab,
		Clock: func() time.Time { return time.Now() },
	})
	items, err := svc.Queue(context.Background(), owner, 50)
	require.NoError(t, err)
	require.Empty(t, items, "thẻ thiếu nội dung bị bỏ khỏi queue")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/review/service/ -run TestQueue -v`
Expected: FAIL (`service.NewQueueService` chưa có).

- [ ] **Step 3: Write queue.go**

Create `internal/review/service/queue.go`:
```go
package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/platform/db"
	revports "github.com/memorix/memorix/internal/review/ports"
	schedports "github.com/memorix/memorix/internal/scheduling/ports"
)

// QueueIntervals = next_intervals dạng giây (client format thành "10 phút"/"4 ngày").
type QueueIntervals struct {
	AgainSeconds int64 `json:"again_seconds"`
	HardSeconds  int64 `json:"hard_seconds"`
	GoodSeconds  int64 `json:"good_seconds"`
	EasySeconds  int64 `json:"easy_seconds"`
}

// QueueItem = 1 thẻ đến hạn kèm nội dung mặt sau + khoảng cách ôn kế.
type QueueItem struct {
	CardID        uuid.UUID      `json:"card_id"`
	EntryID       uuid.UUID      `json:"entry_id"`
	Direction     string         `json:"direction"`
	Term          string         `json:"term"`
	IPA           string         `json:"ipa"`
	Meaning       string         `json:"meaning"`
	Example       string         `json:"example"`
	NextIntervals QueueIntervals `json:"next_intervals"`
}

// QueryRunner chạy fn với 1 Querier read-only.
type QueryRunner func(ctx context.Context, fn func(db.Querier) error) error

type QueueDeps struct {
	Pool      *pgxpool.Pool
	RunQuery  QueryRunner
	Cards     schedports.CardStore
	Prefs     schedports.PrefsStore
	Scheduler schedports.SchedulerPort
	Vocab     revports.VocabularyPort
	Clock     func() time.Time
}

type QueueService struct{ d QueueDeps }

func NewQueueService(d QueueDeps) *QueueService {
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.RunQuery == nil && d.Pool != nil {
		p := d.Pool
		d.RunQuery = func(_ context.Context, fn func(db.Querier) error) error { return fn(p) }
	}
	return &QueueService{d: d}
}

func (s *QueueService) Queue(ctx context.Context, ownerID uuid.UUID, limit int) ([]QueueItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	now := s.d.Clock()

	var (
		cards     []cardsnapshot
		retention float64
	)
	err := s.d.RunQuery(ctx, func(q db.Querier) error {
		prefs, err := s.d.Prefs.Get(ctx, q, ownerID)
		if err != nil {
			return err
		}
		retention = prefs.DesiredRetention
		due, err := s.d.Cards.DueCards(ctx, q, ownerID, now, limit)
		if err != nil {
			return err
		}
		for _, c := range due {
			iv := s.d.Scheduler.Preview(c, retention, now)
			cards = append(cards, cardsnapshot{card: c, iv: iv})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(cards) == 0 {
		return []QueueItem{}, nil
	}

	ids := make([]uuid.UUID, 0, len(cards))
	for _, c := range cards {
		ids = append(ids, c.card.EntryID)
	}
	content, err := s.d.Vocab.BatchGet(ctx, ownerID, ids)
	if err != nil {
		return nil, err
	}

	items := make([]QueueItem, 0, len(cards))
	for _, c := range cards {
		ec, ok := content[c.card.EntryID]
		if !ok {
			continue // thiếu nội dung → bỏ khỏi queue (không hiển thị thẻ rỗng)
		}
		items = append(items, QueueItem{
			CardID: c.card.ID, EntryID: c.card.EntryID, Direction: c.card.Direction,
			Term: ec.Term, IPA: ec.IPA, Meaning: ec.Meaning, Example: ec.Example,
			NextIntervals: QueueIntervals{
				AgainSeconds: secs(c.iv.Again), HardSeconds: secs(c.iv.Hard),
				GoodSeconds: secs(c.iv.Good), EasySeconds: secs(c.iv.Easy),
			},
		})
	}
	return items, nil
}

type cardsnapshot struct {
	card cardType
	iv   intervalType
}

func secs(d time.Duration) int64 {
	if d < 0 {
		return 0
	}
	return int64(d / time.Second)
}
```

Create `internal/review/service/aliases.go`:
```go
package service

import scheddom "github.com/memorix/memorix/internal/scheduling/domain"

// alias nội bộ để queue.go gọn (tránh lặp import path dài).
type cardType = scheddom.Card
type intervalType = scheddom.NextIntervals
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/review/service/ -run TestQueue -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/review/service/queue.go internal/review/service/aliases.go internal/review/service/queue_test.go
git commit -m "feat(review): due queue with batch-loaded entry content + server next_intervals (AD-5,AD-9,FR-14)"
```

---

### Task 14: review/service — SummaryService (đọc thẳng review_logs, TZ study-day) (TDD)

Story 3.6: tổng kết cuối phiên — số từ nhớ được (grade ≥ Good) trong "ngày học" (TZ user, AD-12) đọc THẲNG `review_logs` (authoritative, AD-8), + forecast mai.

**Files:**
- Create: `internal/review/service/summary.go`
- Test: `internal/review/service/summary_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/review/service/summary_test.go`:
```go
package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db"
	revdom "github.com/memorix/memorix/internal/review/domain"
	"github.com/memorix/memorix/internal/review/service"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/stretchr/testify/require"
)

type summaryLogs struct{ rows []revdom.ReviewLogRow }

func (s *summaryLogs) Append(context.Context, db.Querier, revdom.ReviewLogRow) error { return nil }
func (s *summaryLogs) ListForCard(context.Context, db.Querier, uuid.UUID) ([]revdom.ReviewLogRow, error) {
	return nil, nil
}
func (s *summaryLogs) ListForOwnerSince(_ context.Context, _ db.Querier, _ uuid.UUID, _ string) ([]revdom.ReviewLogRow, error) {
	return s.rows, nil
}

type summaryCards struct{ dueTomorrow int }

func (c *summaryCards) Load(context.Context, db.Querier, uuid.UUID, uuid.UUID) (scheddom.Card, error) {
	return scheddom.Card{}, nil
}
func (c *summaryCards) ApplyResult(context.Context, db.Querier, uuid.UUID, scheddom.ScheduleResult) error {
	return nil
}
func (c *summaryCards) DueCards(_ context.Context, _ db.Querier, _ uuid.UUID, until time.Time, _ int) ([]scheddom.Card, error) {
	out := make([]scheddom.Card, c.dueTomorrow)
	return out, nil
}

func TestSummary_CountsRememberedAndForecast(t *testing.T) {
	now := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	rows := []revdom.ReviewLogRow{
		{Grade: scheddom.GradeGood, ReviewedAt: now.Add(-2 * time.Hour)},
		{Grade: scheddom.GradeEasy, ReviewedAt: now.Add(-time.Hour)},
		{Grade: scheddom.GradeAgain, ReviewedAt: now.Add(-30 * time.Minute)}, // quên → không tính nhớ
	}
	svc := service.NewSummaryService(service.SummaryDeps{
		RunQuery: func(_ context.Context, fn func(db.Querier) error) error { return fn(nil) },
		Logs:     &summaryLogs{rows: rows}, Cards: &summaryCards{dueTomorrow: 7},
		Prefs: fakePrefs{}, Clock: func() time.Time { return now },
	})
	sum, err := svc.Summary(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Equal(t, 3, sum.Reviewed)
	require.Equal(t, 2, sum.Remembered, "chỉ grade ≥ Good tính là nhớ")
	require.Equal(t, 7, sum.ForecastTomorrow)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/review/service/ -run TestSummary -v`
Expected: FAIL (`service.NewSummaryService` chưa có).

- [ ] **Step 3: Write summary.go**

Create `internal/review/service/summary.go`:
```go
package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/platform/db"
	revports "github.com/memorix/memorix/internal/review/ports"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	schedports "github.com/memorix/memorix/internal/scheduling/ports"
)

// SessionSummary = số liệu cuối phiên (FR-24). Đọc thẳng review_logs (AD-8, không lag).
type SessionSummary struct {
	Reviewed         int `json:"reviewed"`
	Remembered       int `json:"remembered"`
	ForecastTomorrow int `json:"forecast_tomorrow"`
}

type SummaryDeps struct {
	Pool     *pgxpool.Pool
	RunQuery QueryRunner
	Logs     revports.ReviewLogRepo
	Cards    schedports.CardStore
	Prefs    schedports.PrefsStore
	Clock    func() time.Time
}

type SummaryService struct{ d SummaryDeps }

func NewSummaryService(d SummaryDeps) *SummaryService {
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.RunQuery == nil && d.Pool != nil {
		p := d.Pool
		d.RunQuery = func(_ context.Context, fn func(db.Querier) error) error { return fn(p) }
	}
	return &SummaryService{d: d}
}

func (s *SummaryService) Summary(ctx context.Context, ownerID uuid.UUID) (SessionSummary, error) {
	now := s.d.Clock()
	var out SessionSummary
	err := s.d.RunQuery(ctx, func(q db.Querier) error {
		prefs, err := s.d.Prefs.Get(ctx, q, ownerID)
		if err != nil {
			return err
		}
		loc, err := time.LoadLocation(prefs.Timezone)
		if err != nil {
			loc = time.UTC
		}
		dayStart := scheddom.StudyDayStart(now, loc, 4) // grace 4h sáng (AD-12)

		rows, err := s.d.Logs.ListForOwnerSince(ctx, q, ownerID, dayStart.Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		out.Reviewed = len(rows)
		for _, r := range rows {
			if r.Grade >= scheddom.GradeGood {
				out.Remembered++
			}
		}

		// forecast mai: số thẻ due trước cuối ngày-học kế (dayStart + 48h là biên an toàn).
		tomorrowEnd := dayStart.AddDate(0, 0, 2)
		due, err := s.d.Cards.DueCards(ctx, q, ownerID, tomorrowEnd, 10000)
		if err != nil {
			return err
		}
		out.ForecastTomorrow = len(due)
		return nil
	})
	return out, err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/review/service/ -run TestSummary -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/review/service/summary.go internal/review/service/summary_test.go
git commit -m "feat(review): session summary reads review_logs directly, study-day by user TZ (AD-8,AD-12)"
```

---

### Task 15: review/handler — POST /grade, GET /queue, GET /summary (TDD httptest)

**Files:**
- Create: `internal/review/handler/review.go`
- Test: `internal/review/handler/review_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/review/handler/review_test.go`:
```go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/review/handler"
	"github.com/memorix/memorix/internal/review/service"
	"github.com/stretchr/testify/require"
)

// gradeFn/queueFn/summaryFn: chèn hành vi service qua interface handler yêu cầu.
type stubGrader struct{ calls int }

func (s *stubGrader) Grade(_ ginCtx, owner uuid.UUID, cmd service.GradeInput) (service.GradeOutput, error) {
	s.calls++
	return service.GradeOutput{CardID: cmd.CardID, Stability: 5, DueAt: cmd.Now}, nil
}

// ... xem impl handler cho các interface GraderPort/QueuePort/SummaryPort.
```

> Test đầy đủ (bind + status + idempotent passthrough) nằm ở Step 3 sau khi handler khai báo interface. Tạm để test tối thiểu trên chạy đỏ vì `service.GradeInput` chưa có; ta bổ sung DTO service + handler ở bước sau. **Bỏ file test tạm này**, thay bằng bản đầy đủ:

Ghi đè `internal/review/handler/review_test.go`:
```go
package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	revdom "github.com/memorix/memorix/internal/review/domain"
	"github.com/memorix/memorix/internal/review/handler"
	"github.com/memorix/memorix/internal/review/service"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/stretchr/testify/require"
)

type fakeGrader struct{ calls int }

func (f *fakeGrader) Grade(_ context.Context, owner uuid.UUID, cmd revdom.GradeCommand) (revdom.GradeResult, error) {
	f.calls++
	return revdom.GradeResult{CardID: cmd.CardID, Stability: 5, Difficulty: 5, Status: scheddom.StatusReview, Reps: 1}, nil
}

type fakeQueuer struct{}

func (fakeQueuer) Queue(context.Context, uuid.UUID, int) ([]service.QueueItem, error) {
	return []service.QueueItem{{CardID: uuid.New(), Term: "ephemeral",
		NextIntervals: service.QueueIntervals{AgainSeconds: 600, EasySeconds: 777600}}}, nil
}

type fakeSummary struct{}

func (fakeSummary) Summary(context.Context, uuid.UUID) (service.SessionSummary, error) {
	return service.SessionSummary{Reviewed: 3, Remembered: 2, ForecastTomorrow: 7}, nil
}

func router(owner uuid.UUID, g handler.GraderPort) (*gin.Engine, *gin.RouterGroup) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewReviewHandler(g, fakeQueuer{}, fakeSummary{},
		func(*gin.Context) (uuid.UUID, error) { return owner, nil })
	grp := r.Group("/api/v1")
	h.Register(grp)
	return r, grp
}

func TestGradeEndpoint_AcceptsOnlyCardGradeClientID(t *testing.T) {
	owner := uuid.New()
	fg := &fakeGrader{}
	r, _ := router(owner, fg)

	cid := uuid.New()
	body := `{"card_id":"` + cid.String() + `","grade":3,"client_review_id":"cr-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/review/grade", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, cid.String(), got["card_id"])
	require.Equal(t, 5.0, got["stability"])
	require.Equal(t, 1, fg.calls)
}

func TestGradeEndpoint_RejectsBadGrade(t *testing.T) {
	r, _ := router(uuid.New(), &fakeGrader{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/review/grade",
		strings.NewReader(`{"card_id":"`+uuid.New().String()+`","grade":9,"client_review_id":"x"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestQueueEndpoint(t *testing.T) {
	r, _ := router(uuid.New(), &fakeGrader{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/review/queue", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var got map[string][]map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got["data"], 1)
	require.Equal(t, "ephemeral", got["data"][0]["term"])
}

func TestSummaryEndpoint(t *testing.T) {
	r, _ := router(uuid.New(), &fakeGrader{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/review/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, 2.0, got["remembered"])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/review/handler/ -v`
Expected: FAIL (`handler.NewReviewHandler`, `handler.GraderPort` chưa có).

- [ ] **Step 3: Write handler**

Create `internal/review/handler/review.go`:
```go
package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/httpx"
	revdom "github.com/memorix/memorix/internal/review/domain"
	"github.com/memorix/memorix/internal/review/service"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
)

// PrincipalFunc trích owner id (authmw ở cmd; fake trong test).
type PrincipalFunc func(*gin.Context) (uuid.UUID, error)

// Cổng service (interface để test inject fake).
type GraderPort interface {
	Grade(ctx context.Context, ownerID uuid.UUID, cmd revdom.GradeCommand) (revdom.GradeResult, error)
}
type QueuePort interface {
	Queue(ctx context.Context, ownerID uuid.UUID, limit int) ([]service.QueueItem, error)
}
type SummaryPort interface {
	Summary(ctx context.Context, ownerID uuid.UUID) (service.SessionSummary, error)
}

type ReviewHandler struct {
	grader  GraderPort
	queuer  QueuePort
	summary SummaryPort
	owner   PrincipalFunc
}

func NewReviewHandler(g GraderPort, q QueuePort, s SummaryPort, owner PrincipalFunc) *ReviewHandler {
	return &ReviewHandler{grader: g, queuer: q, summary: s, owner: owner}
}

func (h *ReviewHandler) Register(g *gin.RouterGroup) {
	g.POST("/review/grade", h.grade)
	g.GET("/review/queue", h.queue)
	g.GET("/review/summary", h.summaryHandler)
}

type gradeBody struct {
	CardID         string `json:"card_id"`
	Grade          int16  `json:"grade"`
	ClientReviewID string `json:"client_review_id"`
}

type gradeResp struct {
	CardID     string  `json:"card_id"`
	Stability  float64 `json:"stability"`
	Difficulty float64 `json:"difficulty"`
	Status     int16   `json:"status"`
	Reps       int     `json:"reps"`
	Lapses     int     `json:"lapses"`
	DueAt      string  `json:"due_at"`
}

func (h *ReviewHandler) grade(c *gin.Context) {
	owner, err := h.owner(c)
	if err != nil {
		fail(c, httpx.NewError(httpx.CodeUnauthenticated, "unauthenticated"))
		return
	}
	var b gradeBody
	if err := c.ShouldBindJSON(&b); err != nil {
		fail(c, httpx.NewError(httpx.CodeValidation, "invalid body"))
		return
	}
	cardID, err := uuid.Parse(b.CardID)
	if err != nil {
		fail(c, httpx.NewError(httpx.CodeValidation, "invalid card_id"))
		return
	}
	if b.ClientReviewID == "" {
		fail(c, httpx.NewError(httpx.CodeValidation, "client_review_id required"))
		return
	}
	res, err := h.grader.Grade(c.Request.Context(), owner, revdom.GradeCommand{
		CardID: cardID, Grade: scheddom.Grade(b.Grade), ClientReviewID: b.ClientReviewID,
	})
	switch {
	case errors.Is(err, service.ErrInvalidGrade):
		fail(c, httpx.NewError(httpx.CodeValidation, "grade must be 1..4").WithField("grade", "1..4"))
		return
	case errors.Is(err, scheddom.ErrCardNotFound):
		fail(c, httpx.NewError(httpx.CodeNotFound, "card not found"))
		return
	case err != nil:
		fail(c, httpx.NewError(httpx.CodeInternal, err.Error()))
		return
	}
	c.JSON(http.StatusOK, gradeResp{
		CardID: res.CardID.String(), Stability: res.Stability, Difficulty: res.Difficulty,
		Status: int16(res.Status), Reps: res.Reps, Lapses: res.Lapses, DueAt: res.DueAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (h *ReviewHandler) queue(c *gin.Context) {
	owner, err := h.owner(c)
	if err != nil {
		fail(c, httpx.NewError(httpx.CodeUnauthenticated, "unauthenticated"))
		return
	}
	items, err := h.queuer.Queue(c.Request.Context(), owner, 50)
	if err != nil {
		fail(c, httpx.NewError(httpx.CodeInternal, err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *ReviewHandler) summaryHandler(c *gin.Context) {
	owner, err := h.owner(c)
	if err != nil {
		fail(c, httpx.NewError(httpx.CodeUnauthenticated, "unauthenticated"))
		return
	}
	sum, err := h.summary.Summary(c.Request.Context(), owner)
	if err != nil {
		fail(c, httpx.NewError(httpx.CodeInternal, err.Error()))
		return
	}
	c.JSON(http.StatusOK, sum)
}

func fail(c *gin.Context, e *httpx.APIError) { c.JSON(e.HTTPStatus(), e) }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/review/handler/ -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/review/handler
git commit -m "feat(review): HTTP endpoints POST /review/grade, GET /review/queue, GET /review/summary"
```

---

### Task 16: Grade p95 benchmark (NFR-1 < 150ms) + wire cmd/api

**Files:**
- Test: `internal/review/service/grade_bench_test.go`
- Modify: `cmd/api/main.go` (đăng ký routes scheduling + review)

- [ ] **Step 1: Write the grade latency benchmark**

Create `internal/review/service/grade_bench_test.go`:
```go
package service_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/platform/eventbus"
	revdom "github.com/memorix/memorix/internal/review/domain"
	scheddom "github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/stretchr/testify/require"
)

// Đo p95 chấm (tính lịch + ghi log) — NFR-1 < 150ms. Chạy khi có Docker (bỏ -short).
func TestGrade_P95Under150ms(t *testing.T) {
	pool := dbtest.NewPostgres(t)
	ctx := context.Background()
	owner, entry := uuid.New(), uuid.New()
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)

	const n = 100
	durs := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		cardID := insertNewCard(t, ctx, pool, owner, entry, now)
		svc := realService(pool, eventbus.NewInProcess(), now)
		start := time.Now()
		_, err := svc.Grade(ctx, owner, revdom.GradeCommand{
			CardID: cardID, Grade: scheddom.GradeGood, ClientReviewID: uuid.NewString(),
		})
		durs = append(durs, time.Since(start))
		_ = db.Querier(nil)
		require.NoError(t, err)
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	p95 := durs[int(float64(n)*0.95)-1]
	t.Logf("grade p95 = %v (target < 150ms)", p95)
	require.Less(t, p95, 150*time.Millisecond, "NFR-1: chấm p95 < 150ms")
}
```

- [ ] **Step 2: Run benchmark test**

Run: `go test ./internal/review/service/ -run TestGrade_P95 -v`
Expected: PASS, log `grade p95 = ...` < 150ms (local Postgres; nếu CI chậm, giữ test sau cờ `-short` skip đã có trong dbtest).

- [ ] **Step 3: Wire routes in cmd/api/main.go**

Sửa `cmd/api/main.go` — thêm import `"errors"` (nếu chưa có) cùng `gin`/`uuid`/`authmw` đã có từ Sprint 1-2; sau khi có `pool *pgxpool.Pool`, `bus`, `jwtManager`, thêm khối đăng ký (giữ nguyên phần Sprint 0 dựng router/health):
```go
	// --- Sprint 3: scheduling + review wiring (S6 — ráp adapter→port ở cmd) ---
	ownerFn := func(c *gin.Context) (uuid.UUID, error) {
		// Auth Contract (Sprint 1): authmw.UserID trả (string, bool); UserID là
		// uuid dạng string → parse ở ranh giới. KHÔNG dùng shim header X-User-ID.
		uid, ok := authmw.UserID(c)
		if !ok {
			return uuid.UUID{}, errors.New("unauthenticated")
		}
		return uuid.Parse(uid)
	}

	cardStore := schedrepo.NewCardStore()
	prefsStore := schedrepo.NewPrefsStore()
	sched := fsrsadapter.New()

	prefsSvc := schedsvc.NewPrefsService(pool, prefsStore)
	schedhandler.NewPrefsHandler(prefsSvc, ownerFn).Register(v1)

	gradeSvc := revsvc.NewGradeService(revsvc.GradeDeps{
		Tx:        func(ctx context.Context, fn func(db.Querier) error) error { return db.WithinTx(ctx, pool, fn) },
		Scheduler: sched, Cards: cardStore, Prefs: prefsStore,
		Logs: revrepo.NewReviewLogRepo(), Receipts: revrepo.NewReceiptRepo(), Bus: bus, Clock: time.Now,
	})
	// vocabAdapter bọc port batch-load của vocabulary (Sprint 2). Chữ ký khớp VocabularyFunc.
	vocab := revports.VocabularyFunc(vocabAdapter.BatchGetForReview)
	queueSvc := revsvc.NewQueueService(revsvc.QueueDeps{
		Pool: pool, Cards: cardStore, Prefs: prefsStore, Scheduler: sched, Vocab: vocab, Clock: time.Now,
	})
	summarySvc := revsvc.NewSummaryService(revsvc.SummaryDeps{
		Pool: pool, Logs: revrepo.NewReviewLogRepo(), Cards: cardStore, Prefs: prefsStore, Clock: time.Now,
	})
	revhandler.NewReviewHandler(gradeSvc, queueSvc, summarySvc, ownerFn).Register(v1)
```

Thêm import tương ứng (điều chỉnh alias cho khớp repo):
```go
	"github.com/memorix/memorix/internal/platform/db"
	schedrepo "github.com/memorix/memorix/internal/scheduling/repo"
	"github.com/memorix/memorix/internal/scheduling/repo/fsrsadapter"
	schedsvc "github.com/memorix/memorix/internal/scheduling/service"
	schedhandler "github.com/memorix/memorix/internal/scheduling/handler"
	revports "github.com/memorix/memorix/internal/review/ports"
	revrepo "github.com/memorix/memorix/internal/review/repo"
	revsvc "github.com/memorix/memorix/internal/review/service"
	revhandler "github.com/memorix/memorix/internal/review/handler"
```

> **Giả định wiring:** `authmw.UserID(c) (string, bool)` (Auth Contract — Sprint 1) và `vocabAdapter.BatchGetForReview` (bọc `vocabulary/ports`, Sprint 2). `v1 := r.Group("/api/v1")` đã có từ Sprint 0; `pool`/`bus` khởi tạo ở main (thêm `pool, _ := pgxpool.New(ctx, cfg.DatabaseURL)` + `bus := eventbus.NewInProcess()` nếu chưa có).

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: build sạch. `authmw.UserID` là API chuẩn Sprint 1 (Auth Contract); `vocabAdapter.BatchGetForReview` từ Sprint 2 — không cần shim.

- [ ] **Step 5: Full backend verification sweep**

Run:
```bash
go build ./... && go test ./... -short && golangci-lint run ./...
go test ./internal/scheduling/... ./internal/review/... ./internal/platform/db/... -v   # gồm testcontainers
```
Expected: tất cả xanh; depguard xác nhận `scheduling/domain` + `review/domain` KHÔNG import go-fsrs/pgx/gin.

- [ ] **Step 6: Commit**

```bash
git add internal/review/service/grade_bench_test.go cmd/api/main.go
git commit -m "feat(api): wire scheduling+review routes; grade p95<150ms benchmark (NFR-1)"
```

---

### Task 17: web — review API client + interval formatting (TDD vitest)

**Files:**
- Create: `web/src/review/api.ts`
- Create: `web/src/review/format.ts`
- Test: `web/src/review/format.test.ts`
- Test: `web/src/review/api.test.ts`

- [ ] **Step 1: Write the failing tests**

Create `web/src/review/format.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { formatInterval } from "./format";

describe("formatInterval", () => {
  it("phút cho < 1 giờ", () => expect(formatInterval(600)).toBe("10 phút"));
  it("giờ cho < 1 ngày", () => expect(formatInterval(3600)).toBe("1 giờ"));
  it("ngày cho >= 1 ngày", () => expect(formatInterval(4 * 86400)).toBe("4 ngày"));
  it("tháng khi lớn", () => expect(formatInterval(60 * 86400)).toBe("2 tháng"));
});
```

Create `web/src/review/api.test.ts`:
```ts
import { describe, it, expect, vi, beforeEach } from "vitest";
import { fetchQueue, submitGrade } from "./api";

beforeEach(() => vi.restoreAllMocks());

describe("review api", () => {
  it("fetchQueue trả data[]", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(
      JSON.stringify({ data: [{ card_id: "c1", term: "x", next_intervals: { again_seconds: 600, hard_seconds: 3600, good_seconds: 86400, easy_seconds: 777600 } }] }),
      { status: 200 })));
    const items = await fetchQueue();
    expect(items).toHaveLength(1);
    expect(items[0].term).toBe("x");
  });

  it("submitGrade gửi đúng payload {card_id,grade,client_review_id}", async () => {
    const spy = vi.fn(async () => new Response(JSON.stringify({ card_id: "c1", stability: 5 }), { status: 200 }));
    vi.stubGlobal("fetch", spy);
    await submitGrade({ card_id: "c1", grade: 3, client_review_id: "cr-1" });
    const [, init] = spy.mock.calls[0];
    expect(JSON.parse(init.body)).toEqual({ card_id: "c1", grade: 3, client_review_id: "cr-1" });
    expect(init.method).toBe("POST");
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && npx vitest run src/review/`
Expected: FAIL (module chưa có).

- [ ] **Step 3: Write format.ts**

Create `web/src/review/format.ts`:
```ts
// formatInterval đổi giây → nhãn tiếng Việt ngắn cho nút chấm (FR-14).
export function formatInterval(seconds: number): string {
  if (seconds < 3600) return `${Math.max(1, Math.round(seconds / 60))} phút`;
  if (seconds < 86400) return `${Math.round(seconds / 3600)} giờ`;
  const days = Math.round(seconds / 86400);
  if (days < 30) return `${days} ngày`;
  if (days < 365) return `${Math.round(days / 30)} tháng`;
  return `${Math.round(days / 365)} năm`;
}
```

- [ ] **Step 4: Write api.ts**

Create `web/src/review/api.ts`:
```ts
export interface NextIntervals {
  again_seconds: number;
  hard_seconds: number;
  good_seconds: number;
  easy_seconds: number;
}

export interface QueueItem {
  card_id: string;
  entry_id?: string;
  direction?: string;
  term: string;
  ipa?: string;
  meaning?: string;
  example?: string;
  next_intervals: NextIntervals;
}

export interface GradePayload {
  card_id: string;
  grade: number; // 1..4
  client_review_id: string;
}

export interface GradeResult {
  card_id: string;
  stability: number;
  due_at?: string;
}

export interface SessionSummary {
  reviewed: number;
  remembered: number;
  forecast_tomorrow: number;
}

const BASE = "/api/v1";

export async function fetchQueue(): Promise<QueueItem[]> {
  const res = await fetch(`${BASE}/review/queue`, { credentials: "include" });
  if (!res.ok) throw new Error(`queue ${res.status}`);
  const body = await res.json();
  return body.data ?? [];
}

export async function submitGrade(p: GradePayload): Promise<GradeResult> {
  const res = await fetch(`${BASE}/review/grade`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(p),
  });
  if (!res.ok) throw new Error(`grade ${res.status}`);
  return res.json();
}

export async function fetchSummary(): Promise<SessionSummary> {
  const res = await fetch(`${BASE}/review/summary`, { credentials: "include" });
  if (!res.ok) throw new Error(`summary ${res.status}`);
  return res.json();
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd web && npx vitest run src/review/`
Expected: PASS (6 tests).

- [ ] **Step 6: Commit**

```bash
cd .. && git add web/src/review/api.ts web/src/review/format.ts web/src/review/format.test.ts web/src/review/api.test.ts
git commit -m "feat(web): review API client (queue/grade/summary) + Vietnamese interval formatting"
```

---

### Task 18: web — offline grade queue (no lost grade) (TDD vitest)

Story 3.5: mất mạng → điểm ghi cục bộ (localStorage), flush idempotent khi có mạng; **không bao giờ mất điểm** (FR-22).

**Files:**
- Create: `web/src/review/offlineQueue.ts`
- Test: `web/src/review/offlineQueue.test.ts`

- [ ] **Step 1: Write the failing test**

Create `web/src/review/offlineQueue.test.ts`:
```ts
import { describe, it, expect, vi, beforeEach } from "vitest";
import { OfflineGradeQueue } from "./offlineQueue";
import type { GradePayload } from "./api";

const p = (id: string): GradePayload => ({ card_id: id, grade: 3, client_review_id: `cr-${id}` });

beforeEach(() => localStorage.clear());

describe("OfflineGradeQueue", () => {
  it("giữ điểm khi submit lỗi và không mất khi khởi tạo lại", async () => {
    const failing = vi.fn(async () => { throw new Error("offline"); });
    const q = new OfflineGradeQueue(failing);
    await q.enqueue(p("c1"));
    expect(q.pending()).toHaveLength(1);

    // reload: hàng đợi đọc lại từ localStorage → không mất điểm
    const q2 = new OfflineGradeQueue(failing);
    expect(q2.pending()).toHaveLength(1);
  });

  it("flush gửi hết và xóa khỏi hàng đợi khi thành công", async () => {
    const sent: string[] = [];
    const ok = vi.fn(async (g: GradePayload) => { sent.push(g.client_review_id); });
    const q = new OfflineGradeQueue(ok);
    // enqueue khi đang lỗi
    const flaky = new OfflineGradeQueue(async () => { throw new Error("offline"); });
    await flaky.enqueue(p("c1"));
    await flaky.enqueue(p("c2"));

    // cùng storage key → q thấy 2 mục, flush thành công
    await q.flush();
    expect(sent).toContain("cr-c1");
    expect(sent).toContain("cr-c2");
    expect(q.pending()).toHaveLength(0);
  });

  it("enqueue online submit thành công thì không lưu lại", async () => {
    const ok = vi.fn(async () => {});
    const q = new OfflineGradeQueue(ok);
    await q.enqueue(p("c1"));
    expect(ok).toHaveBeenCalledOnce();
    expect(q.pending()).toHaveLength(0);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run src/review/offlineQueue.test.ts`
Expected: FAIL (module chưa có).

- [ ] **Step 3: Write offlineQueue.ts**

Create `web/src/review/offlineQueue.ts`:
```ts
import type { GradePayload } from "./api";

const KEY = "memorix.pendingGrades";

// OfflineGradeQueue đảm bảo KHÔNG mất điểm (FR-22): thử gửi ngay; nếu lỗi (offline)
// thì lưu localStorage, flush lại sau. client_review_id giữ idempotency server-side.
export class OfflineGradeQueue {
  private submit: (g: GradePayload) => Promise<unknown>;

  constructor(submit: (g: GradePayload) => Promise<unknown>) {
    this.submit = submit;
  }

  pending(): GradePayload[] {
    try {
      return JSON.parse(localStorage.getItem(KEY) ?? "[]");
    } catch {
      return [];
    }
  }

  private save(list: GradePayload[]) {
    localStorage.setItem(KEY, JSON.stringify(list));
  }

  // enqueue thử gửi ngay; thất bại → xếp hàng bền vững.
  async enqueue(g: GradePayload): Promise<void> {
    try {
      await this.submit(g);
    } catch {
      const list = this.pending();
      if (!list.some((x) => x.client_review_id === g.client_review_id)) {
        list.push(g);
        this.save(list);
      }
    }
  }

  // flush gửi lần lượt; mục nào lỗi giữ lại (không mất). An toàn gọi lặp.
  async flush(): Promise<void> {
    const list = this.pending();
    const remain: GradePayload[] = [];
    for (const g of list) {
      try {
        await this.submit(g);
      } catch {
        remain.push(g);
      }
    }
    this.save(remain);
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && npx vitest run src/review/offlineQueue.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/review/offlineQueue.ts web/src/review/offlineQueue.test.ts
git commit -m "feat(web): offline grade queue backed by localStorage — never lose a grade (FR-22)"
```

---

### Task 19: web — ReviewScreen (flip, 4 grade buttons + intervals, keys 1-4/Space, optimistic) (TDD vitest)

Stories 3.4 + 3.5: mặt trước term → Space/nút lật → mặt sau; chấm 1-4 hoặc phím → thẻ kế hiện ngay (optimistic), điểm qua OfflineGradeQueue.

**Files:**
- Create: `web/src/review/ReviewScreen.tsx`
- Test: `web/src/review/ReviewScreen.test.tsx`

- [ ] **Step 1: Write the failing test**

Create `web/src/review/ReviewScreen.test.tsx`:
```tsx
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { ReviewScreen } from "./ReviewScreen";
import type { QueueItem } from "./api";

const items: QueueItem[] = [
  { card_id: "c1", term: "ephemeral", ipa: "/ɪ'fem/", meaning: "chóng tàn", example: "ephemeral trend",
    next_intervals: { again_seconds: 600, hard_seconds: 3600, good_seconds: 86400, easy_seconds: 777600 } },
  { card_id: "c2", term: "ubiquitous", meaning: "phổ biến khắp nơi",
    next_intervals: { again_seconds: 600, hard_seconds: 3600, good_seconds: 172800, easy_seconds: 1209600 } },
];

describe("ReviewScreen", () => {
  it("mặt trước ẩn nghĩa; Space lật hiện mặt sau", () => {
    render(<ReviewScreen items={items} onGrade={vi.fn()} onDone={vi.fn()} />);
    expect(screen.getByText("ephemeral")).toBeInTheDocument();
    expect(screen.queryByText("chóng tàn")).not.toBeInTheDocument();
    fireEvent.keyDown(window, { key: " " });
    expect(screen.getByText("chóng tàn")).toBeInTheDocument();
  });

  it("nút grade hiển thị khoảng cách ôn kế", () => {
    render(<ReviewScreen items={items} onGrade={vi.fn()} onDone={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /Lật thẻ/ }));
    expect(screen.getByRole("button", { name: /Again/ })).toHaveTextContent("10 phút");
    expect(screen.getByRole("button", { name: /Good/ })).toHaveTextContent("1 ngày");
  });

  it("phím 3 chấm Good và advance sang thẻ kế ngay (optimistic)", async () => {
    const onGrade = vi.fn(async () => {});
    render(<ReviewScreen items={items} onGrade={onGrade} onDone={vi.fn()} />);
    fireEvent.keyDown(window, { key: " " }); // lật
    fireEvent.keyDown(window, { key: "3" }); // Good
    expect(onGrade).toHaveBeenCalledWith(expect.objectContaining({ card_id: "c1", grade: 3 }));
    await waitFor(() => expect(screen.getByText("ubiquitous")).toBeInTheDocument());
  });

  it("hết thẻ gọi onDone", async () => {
    const onDone = vi.fn();
    render(<ReviewScreen items={[items[0]]} onGrade={vi.fn(async () => {})} onDone={onDone} />);
    fireEvent.keyDown(window, { key: " " });
    fireEvent.keyDown(window, { key: "3" });
    await waitFor(() => expect(onDone).toHaveBeenCalled());
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run src/review/ReviewScreen.test.tsx`
Expected: FAIL (component chưa có).

- [ ] **Step 3: Write ReviewScreen.tsx**

Create `web/src/review/ReviewScreen.tsx`:
```tsx
import { useCallback, useEffect, useState } from "react";
import type { GradePayload, QueueItem } from "./api";
import { formatInterval } from "./format";

interface Props {
  items: QueueItem[];
  onGrade: (g: GradePayload) => Promise<void>;
  onDone: () => void;
}

const GRADES = [
  { g: 1, label: "Again", key: "1", color: "var(--again)", field: "again_seconds" },
  { g: 2, label: "Hard", key: "2", color: "var(--hard)", field: "hard_seconds" },
  { g: 3, label: "Good", key: "3", color: "var(--good)", field: "good_seconds" },
  { g: 4, label: "Easy", key: "4", color: "var(--easy)", field: "easy_seconds" },
] as const;

// clientReviewID ổn định theo (card, lần chấm) để server idempotent (AD-3).
function reviewID(cardID: string): string {
  return `${cardID}:${Date.now()}:${Math.random().toString(36).slice(2, 8)}`;
}

export function ReviewScreen({ items, onGrade, onDone }: Props) {
  const [idx, setIdx] = useState(0);
  const [flipped, setFlipped] = useState(false);
  const card = items[idx];

  const advance = useCallback(() => {
    setFlipped(false);
    if (idx + 1 >= items.length) {
      onDone();
    } else {
      setIdx((i) => i + 1);
    }
  }, [idx, items.length, onDone]);

  const grade = useCallback(
    (g: number) => {
      if (!card) return;
      const payload: GradePayload = { card_id: card.card_id, grade: g, client_review_id: reviewID(card.card_id) };
      // optimistic: advance NGAY, gửi nền (không chờ mạng — FR-21).
      void onGrade(payload);
      advance();
    },
    [card, onGrade, advance],
  );

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === " ") {
        e.preventDefault();
        setFlipped(true);
        return;
      }
      if (flipped && ["1", "2", "3", "4"].includes(e.key)) {
        e.preventDefault();
        grade(Number(e.key));
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [flipped, grade]);

  if (!card) return null;

  return (
    <div style={{ maxWidth: 560, margin: "0 auto", padding: 16 }}>
      <p style={{ color: "var(--muted)" }} aria-live="polite">
        {idx + 1} / {items.length}
      </p>

      <section
        aria-label="Thẻ ôn"
        style={{ background: "var(--surface)", border: "1px solid var(--line)", borderRadius: "var(--radius)", padding: 24, textAlign: "center" }}
      >
        <h1 style={{ fontSize: 32 }}>{card.term}</h1>
        {flipped && (
          <div>
            {card.ipa && <p style={{ color: "var(--muted)" }}>{card.ipa}</p>}
            {card.meaning && <p style={{ fontSize: 18 }}>{card.meaning}</p>}
            {card.example && <p style={{ fontStyle: "italic", color: "var(--muted)" }}>{card.example}</p>}
          </div>
        )}
      </section>

      {!flipped ? (
        <button
          onClick={() => setFlipped(true)}
          style={{ width: "100%", minHeight: "var(--tap)", marginTop: 16, borderRadius: "var(--radius)", border: "none", background: "var(--accent)", color: "#fff", fontSize: 16 }}
        >
          Lật thẻ (Space)
        </button>
      ) : (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(4,1fr)", gap: 8, marginTop: 16 }}>
          {GRADES.map((b) => (
            <button
              key={b.g}
              onClick={() => grade(b.g)}
              aria-label={`${b.label} — phím ${b.key}`}
              style={{ minHeight: "var(--tap)", borderRadius: "var(--radius)", border: "none", background: b.color, color: "#fff", padding: 8 }}
            >
              <span style={{ display: "block", fontWeight: 600 }}>{b.label}</span>
              <span style={{ display: "block", fontSize: 12 }}>{formatInterval(card.next_intervals[b.field])}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && npx vitest run src/review/ReviewScreen.test.tsx`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/review/ReviewScreen.tsx web/src/review/ReviewScreen.test.tsx
git commit -m "feat(web): review screen — flip, 4 grade buttons with intervals, keys 1-4+Space, optimistic advance (FR-20,21,23)"
```

---

### Task 20: web — SessionSummary + ReviewSession container (offline banner) (TDD vitest)

Story 3.6 màn ăn mừng + Story 3.5 banner offline non-blocking. Container nối queue → ReviewScreen → summary, dùng OfflineGradeQueue.

**Files:**
- Create: `web/src/review/SessionSummary.tsx`
- Create: `web/src/review/ReviewSession.tsx`
- Test: `web/src/review/SessionSummary.test.tsx`
- Test: `web/src/review/ReviewSession.test.tsx`

- [ ] **Step 1: Write the failing tests**

Create `web/src/review/SessionSummary.test.tsx`:
```tsx
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { SessionSummary } from "./SessionSummary";

describe("SessionSummary", () => {
  it("hiển thị số từ nhớ được + forecast mai (không màn trống)", () => {
    render(<SessionSummary summary={{ reviewed: 12, remembered: 9, forecast_tomorrow: 15 }} />);
    expect(screen.getByText(/9/)).toBeInTheDocument();
    expect(screen.getByText(/nhớ được/i)).toBeInTheDocument();
    expect(screen.getByText(/15/)).toBeInTheDocument();
  });
});
```

Create `web/src/review/ReviewSession.test.tsx`:
```tsx
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { ReviewSession } from "./ReviewSession";
import type { QueueItem, SessionSummary as Sum } from "./api";

const items: QueueItem[] = [
  { card_id: "c1", term: "ephemeral", meaning: "chóng tàn",
    next_intervals: { again_seconds: 600, hard_seconds: 3600, good_seconds: 86400, easy_seconds: 777600 } },
];
const sum: Sum = { reviewed: 1, remembered: 1, forecast_tomorrow: 3 };

describe("ReviewSession", () => {
  it("chấm hết thẻ → chuyển sang màn tổng kết", async () => {
    const submit = vi.fn(async () => {});
    render(<ReviewSession loadQueue={async () => items} submitGrade={submit} loadSummary={async () => sum} />);
    await waitFor(() => screen.getByText("ephemeral"));
    fireEvent.keyDown(window, { key: " " });
    fireEvent.keyDown(window, { key: "3" });
    await waitFor(() => expect(screen.getByText(/nhớ được/i)).toBeInTheDocument());
  });

  it("submit lỗi (offline) vẫn advance + hiện banner, không mất điểm", async () => {
    const submit = vi.fn(async () => { throw new Error("offline"); });
    render(<ReviewSession loadQueue={async () => items} submitGrade={submit} loadSummary={async () => sum} />);
    await waitFor(() => screen.getByText("ephemeral"));
    fireEvent.keyDown(window, { key: " " });
    fireEvent.keyDown(window, { key: "3" });
    await waitFor(() => expect(screen.getByText(/điểm đã lưu/i)).toBeInTheDocument());
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && npx vitest run src/review/SessionSummary.test.tsx src/review/ReviewSession.test.tsx`
Expected: FAIL (component chưa có).

- [ ] **Step 3: Write SessionSummary.tsx**

Create `web/src/review/SessionSummary.tsx`:
```tsx
import type { SessionSummary as Sum } from "./api";

export function SessionSummary({ summary }: { summary: Sum }) {
  return (
    <div style={{ maxWidth: 480, margin: "0 auto", padding: 24, textAlign: "center" }}>
      <div style={{ fontSize: 48 }}>🎉</div>
      <h1>Hoàn thành phiên ôn!</h1>
      <p style={{ fontSize: 28, color: "var(--accent)", fontWeight: 700 }}>
        {summary.remembered} từ nhớ được
      </p>
      <p style={{ color: "var(--muted)" }}>Đã ôn {summary.reviewed} thẻ phiên này.</p>
      <p style={{ color: "var(--muted)" }}>Mai có khoảng {summary.forecast_tomorrow} thẻ đến hạn.</p>
    </div>
  );
}
```

- [ ] **Step 4: Write ReviewSession.tsx**

Create `web/src/review/ReviewSession.tsx`:
```tsx
import { useEffect, useMemo, useState } from "react";
import type { GradePayload, QueueItem, SessionSummary as Sum } from "./api";
import { OfflineGradeQueue } from "./offlineQueue";
import { ReviewScreen } from "./ReviewScreen";
import { SessionSummary } from "./SessionSummary";

interface Props {
  loadQueue: () => Promise<QueueItem[]>;
  submitGrade: (g: GradePayload) => Promise<unknown>;
  loadSummary: () => Promise<Sum>;
}

type Phase = "loading" | "review" | "summary";

export function ReviewSession({ loadQueue, submitGrade, loadSummary }: Props) {
  const [phase, setPhase] = useState<Phase>("loading");
  const [items, setItems] = useState<QueueItem[]>([]);
  const [summary, setSummary] = useState<Sum | null>(null);
  const [offline, setOffline] = useState(false);

  const queue = useMemo(() => new OfflineGradeQueue(submitGrade), [submitGrade]);

  useEffect(() => {
    void (async () => {
      const q = await loadQueue();
      setItems(q);
      setPhase(q.length === 0 ? "summary" : "review");
      if (q.length === 0) setSummary(await loadSummary());
    })();
  }, [loadQueue, loadSummary]);

  const onGrade = async (g: GradePayload) => {
    const before = queue.pending().length;
    await queue.enqueue(g);
    if (queue.pending().length > before) setOffline(true); // lưu offline
  };

  const onDone = async () => {
    void queue.flush();
    setSummary(await loadSummary());
    setPhase("summary");
  };

  if (phase === "loading") return <p style={{ padding: 24, color: "var(--muted)" }}>Đang tải…</p>;
  if (phase === "summary" && summary) return <SessionSummary summary={summary} />;
  return (
    <div>
      {offline && (
        <div role="status" style={{ background: "var(--hard)", color: "#fff", padding: 8, textAlign: "center" }}>
          Mất mạng — điểm đã lưu, sẽ đồng bộ sau.
        </div>
      )}
      <ReviewScreen items={items} onGrade={onGrade} onDone={onDone} />
    </div>
  );
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd web && npx vitest run src/review/`
Expected: PASS (toàn bộ review tests).

- [ ] **Step 6: Full frontend verification**

Run: `cd web && npx vitest run && npm run build`
Expected: xanh + build thành công.

- [ ] **Step 7: Commit**

```bash
cd .. && git add web/src/review/SessionSummary.tsx web/src/review/ReviewSession.tsx web/src/review/SessionSummary.test.tsx web/src/review/ReviewSession.test.tsx
git commit -m "feat(web): session summary screen + review session container with offline banner (FR-24, UX-DR14)"
```

---

## Self-Review

### Spec coverage (Story AC → task)

| Story / AC | Tasks |
|---|---|
| **3.1** server tính S/D/Due qua port FSRS (AD-5, AD-7) | 4 (SchedulerPort), 5 (fsrsadapter), 11 (Grade dùng port) |
| 3.1 update cards + insert review_logs 1 TX (AD-3, NFR-5) | 1 (WithinTx), 11 (Grade TX), 12 (integration nguyên tử) |
| 3.1 idempotent trùng (card_id, client_review_id) (FR-15) | 2 (grade_receipts unique), 10 (ReceiptRepo), 11 + 12 (idempotent test) |
| 3.1 review_logs append-only, replay-được (AD-4, NFR-6) | 2 (partition append-only), 10 (Append), 12 (**replay test**) |
| 3.1 p95 < 150ms (NFR-1) | 16 (benchmark) |
| **3.2** desired retention 0.80–0.97 (FR-17) | 2 (CHECK), 3 (RetentionInRange), 7 (validate), 8 (PUT prefs) |
| 3.2 Due server-time; "ngày học" TZ user (FR-18, AD-12) | 3 (StudyDayStart), 6 (DueCards server now), 14 (summary study-day) |
| **3.3** queue due + entry content batch-load (AD-9, FR-19) | 6 (DueCards), 9 (VocabularyPort), 13 (QueueService) |
| 3.3 next_intervals 4 mức server tính (FR-14) | 5 (Preview), 13 (QueueItem.NextIntervals) |
| **3.4** front/back flip (FR-20) | 19 (ReviewScreen flip) |
| 3.4 chấm 4 mức + advance | 19 (grade buttons) |
| 3.4 phím 1–4 + Space (FR-23, UX-DR16) | 19 (keydown handler) |
| **3.5** optimistic advance, prefetch 0 loading (FR-21, NFR-3) | 19 (advance ngay, items preloaded) |
| 3.5 offline không mất điểm + banner (FR-22, UX-DR14) | 18 (OfflineGradeQueue), 20 (banner + ReviewSession) |
| **3.6** summary đọc review_logs + forecast (FR-24, AD-8) | 14 (SummaryService), 15 (endpoint), 20 (SessionSummary) |
| 3.6 không màn trống (FR-34) | 20 (phase summary khi queue rỗng) |
| CardGraded event (AD-8) | 11 (Publish fresh-only), test trong 11 |

### Placeholder scan
- Không có TBD/TODO ngoài: (a) Task 7 chủ động XÓA method `querier()` thừa (Step 4 hướng dẫn xóa — không để lại code chết); (b) Task 16 ghi rõ 2 điểm wiring giả định Sprint 1/2 (`authmw.UserID`, `vocabAdapter.BatchGetForReview`) + shim dev header nếu chưa có — đây là ranh giới sprint thật, không phải placeholder ẩn. (c) Task 15 Step 1 có bản test tạm rồi **ghi đè** bằng bản đầy đủ ở cùng bước — cố ý để minh họa đỏ trước.
- Mọi hàm/type dùng ở task sau đều định nghĩa ở task trước (không tham chiếu ma).

### Type consistency
- `db.Querier` / `db.WithinTx` (Task 1) dùng xuyên suốt repo + service.
- `domain.Card`, `domain.Grade` (1..4), `domain.CardStatus` (0..3), `domain.ScheduleResult`, `domain.NextIntervals`, `domain.SchedulerPrefs` (Task 3) — nhất quán ở ports/adapter/service.
- `SchedulerPort.Apply/Preview` chữ ký khớp giữa Task 4 (khai báo), 5 (impl), 11/13 (dùng).
- `CardStore.Load/ApplyResult/DueCards`, `PrefsStore.Get/Upsert` khớp Task 4 ↔ 6 ↔ 11/13/14.
- `revdom.GradeCommand/GradeResult/ReviewLogRow` (Task 9) khớp repo (10), service (11), handler (15).
- `ReceiptRepo.Insert(...)→(bool,error)` / `Get(...)→(GradeResult,bool,error)` khớp Task 9 ↔ 10 ↔ 11.
- `service.QueueItem/QueueIntervals/SessionSummary` khớp Go (13,14,15) ↔ TS (`api.ts` field names snake_case) ở Task 17.
- FE: `GradePayload{card_id,grade,client_review_id}` (Task 17) khớp handler `gradeBody` (Task 15) và AD-5 (client chỉ gửi 3 field).
- go-fsrs alignment: `domain.CardStatus`↔`fsrs.State` (New=0..Relearning=3), `domain.Grade`↔`fsrs.Rating` (Again=1..Easy=4) — kiểm bằng test Task 3 + adapter Task 5.

### Gaps (ghi nhận có chủ đích — ngoài scope Sprint 3)
- **Idempotency guard tách bảng:** Postgres không cho `unique(card_id, client_review_id)` trực tiếp trên bảng partitioned → guard nằm ở `review.grade_receipts` (Task 2). Thoả **ý định** AC 3.1; `review_logs` giữ `unique(...,reviewed_at)` phòng thủ + là replay source thuần.
- **Partition tháng tự động:** sprint tạo DEFAULT partition (đủ cho test + an toàn). Job River tạo partition tháng trước = Sprint 4/ops (không cản Sprint 3).
- **Queue priority/daily-limit/chống-nổ (FR-25/26/27/28):** Epic 4 — Task 13 chỉ due cơ bản (đã ghi ở Scope boundary).
- **Progress read model (daily_stats/streak/North Star):** Sprint 5 — Sprint 3 chỉ phát `CardGraded` + summary đọc thẳng logs.
- **Fuzz interval:** TẮT có chủ đích (determinism replay AD-4); bật + load-balancing = future extension (spec §Future).
- **Wiring authmw/vocabulary:** giả định Sprint 1/2 API; Task 16 nêu shim dev nếu tên khác.

---

## Execution Handoff

Plan hoàn tất và lưu tại `docs/superpowers/plans/2026-07-07-sprint-3-fsrs-review.md`. Hai lựa chọn thực thi:

1. **Subagent-Driven (khuyến nghị)** — dispatch subagent mới mỗi task, review giữa các task, lặp nhanh. REQUIRED SUB-SKILL: `superpowers:subagent-driven-development`.
2. **Inline Execution** — chạy tuần tự trong session này với checkpoint. REQUIRED SUB-SKILL: `superpowers:executing-plans`.

Chọn cách nào?








