# Sprint 4 — Smart Queue & Daily Limits Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Nâng cấp queue `scheduling` từ "due cơ bản" (Sprint 3) lên **queue ưu tiên + giới hạn ngày + chống nổ + luồng học thẻ mới** — Epic 4, Story 4.1–4.5 (FR-25..29).

**Architecture:** `scheduling` là module LÕI (full hexagonal). Trọng tâm sprint = **domain thuần, unit-test được không cần DB**: `Retrievability`, `StartOfStudyDay`, `BuildQueue`, `PlanAntiFlood`. Service orchestrate (nạp thẻ qua `CardRepo`, đếm đã-phục-vụ-hôm-nay qua `ReviewActivityPort` — cross-module AD-9). Repo = pgx trên index `cards(owner_id, due_at)` (NFR-2). Domain giữ purity (depguard S5): không import gin/pgx/net/http.

**Tech Stack:** Go 1.26, Gin v1.10, pgx v5, Postgres 18, testcontainers-go, testify, React 19 + Vite 7 + Vitest. `github.com/google/uuid` cho ID (được phép trong domain — không phải hạ tầng).

**Nguồn:** `epics.md` Epic 4 (Story 4.1–4.5) · `ARCHITECTURE-SPINE.md` (AD-5/7/8/9/10/12) · `prd.md` FR-25..29 (FR-28 mặc định: overdue ≤ ~2× review-limit theo R thấp nhất, rải phần dư ≤7 ngày) · `addendum-structure.md` (S1–S7). Tái dùng platform types Sprint 0 + domain types Sprint 3.

### Reused types

**Platform (Sprint 0):** `httpx.APIError` + `httpx.NewError(code,msg)` + `CodeValidation/CodeUnauthenticated/CodeNotFound` + `HTTPStatus()`; `httpx.Cursor/Page`; `config.Config`; `logger.New/Scrub`; `eventbus.Bus`.

**Scheduling domain (Sprint 3 — hiển thị để test chạy được; KHÔNG tạo lại):**
```go
// internal/scheduling/domain/card.go  (đã có từ Sprint 3)
package domain

import (
	"time"

	"github.com/google/uuid"
)

type CardState int16

const (
	StateNew        CardState = 0
	StateLearning   CardState = 1
	StateReview     CardState = 2
	StateRelearning CardState = 3
)

type Card struct {
	ID             uuid.UUID
	OwnerID        uuid.UUID
	EntryID        uuid.UUID
	State          CardState
	Stability      float64   // FSRS S (ngày)
	Difficulty     float64   // FSRS D
	DueAt          time.Time // server-time
	LastReviewedAt time.Time // zero cho thẻ chưa ôn
	Reps           int
	Lapses         int
	CreatedAt      time.Time
}

type SchedulerPrefs struct {
	UserID           uuid.UUID
	DesiredRetention float64 // 0.80–0.97
	DailyNewLimit    int     // mặc định 20
	DailyReviewLimit int     // mặc định 200
	Timezone         string  // IANA, vd "Asia/Ho_Chi_Minh"
}
```

**DB (Sprint 3 — bảng đã tồn tại):** `scheduling.cards` (cột: `id, owner_id, entry_id, state, stability, difficulty, due_at, last_reviewed_at, reps, lapses, created_at, updated_at, deleted_at`) · `scheduling.user_scheduler_prefs` (`user_id, desired_retention, daily_new_limit, daily_review_limit, timezone, updated_at`) · `review.review_logs` (append-only; cột dùng ở sprint này: `user_id, reviewed_at, prev_state`).

**Scope boundary:** KHÔNG đụng thuật toán FSRS reschedule (grade flow = Sprint 3, qua `SchedulerPort`, AD-7). `Retrievability` ở đây chỉ để XẾP ƯU TIÊN queue (read-model), không ghi lịch. KHÔNG làm dashboard/stats (Sprint 5). Batch-load nội dung entry cho queue (VocabularyPort) đã có từ Sprint 3 — sprint này chỉ trả card refs đã sắp xếp.

---

## Cross-Sprint Auth Contract (canonical — Sprint 1)

Sprint 1 sở hữu `internal/platform/authmw`. Downstream **phải** dùng đúng API này, không tự chế reader/context-key:
- `authmw.RequireAuth(jwtManager) gin.HandlerFunc` — guard route cần đăng nhập.
- `authmw.PrincipalFrom(c) (Principal, bool)` · `Principal{UserID string, Role string, Plan string}`.
- `authmw.UserID(c) (string, bool)` — reader tiện lợi; **UserID là uuid dạng string**. `uuid.Parse(uid)` ở ranh giới repo nếu cần `uuid.UUID`.
- **Timezone KHÔNG nằm trong principal/context** — lấy qua `IdentityPort.UserTimezone(ctx, userID) (string, error)` (AD-9) rồi `time.LoadLocation` (AD-12).
- Test: fake bằng middleware test gọi `c.Set` với đúng `authmw.Principal{...}`.

> Áp dụng: fake middleware test `c.Set("user_id", uuid.New())` và `c.Get("user_id")` → dùng `authmw.Principal{UserID: ...}` + `authmw.UserID(c)`.

---

### Task 1: Migration — index NFR-2 + bảng study_profiles (coach flag)

**Files:**
- Create: `migrations/0009_scheduling_queue.up.sql`, `migrations/0009_scheduling_queue.down.sql`
- Test: `internal/scheduling/repo/migration_test.go`

- [ ] **Step 1: Viết migration SQL**

Create `migrations/0009_scheduling_queue.up.sql`:
```sql
-- NFR-2: index phục vụ BuildQueue (overdue/due theo owner), p95<500ms cho 10k thẻ.
CREATE INDEX IF NOT EXISTS idx_cards_owner_due
	ON scheduling.cards (owner_id, due_at);

-- Cờ "đã xem hướng dẫn chấm lần đầu" cho luồng học thẻ mới (FR-29, Story 4.5).
CREATE TABLE IF NOT EXISTS scheduling.study_profiles (
	user_id             uuid PRIMARY KEY,
	learn_coach_seen_at timestamptz,
	created_at          timestamptz NOT NULL DEFAULT now(),
	updated_at          timestamptz NOT NULL DEFAULT now()
);
```

Create `migrations/0009_scheduling_queue.down.sql`:
```sql
DROP TABLE IF EXISTS scheduling.study_profiles;
DROP INDEX IF EXISTS scheduling.idx_cards_owner_due;
```

- [ ] **Step 2: Viết integration test (testcontainers, tự dựng bảng tối thiểu)**

Create `internal/scheduling/repo/migration_test.go`:
```go
package repo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startPG khởi Postgres + tạo schema/bảng scheduling tối thiểu (thay cho migrations Sprint 1-3).
func startPG(t *testing.T) (string, func()) {
	t.Helper()
	if testing.Short() {
		t.Skip("skip container test in -short mode")
	}
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "postgres:18",
		postgres.WithDatabase("memorix"),
		postgres.WithUsername("test"), postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start pg: %v", err)
	}
	dsn, _ := pg.ConnectionString(ctx, "sslmode=disable")
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)
	schema := `
		CREATE SCHEMA IF NOT EXISTS scheduling;
		CREATE TABLE scheduling.cards (
			id uuid PRIMARY KEY, owner_id uuid NOT NULL, entry_id uuid NOT NULL,
			state smallint NOT NULL, stability double precision NOT NULL DEFAULT 0,
			difficulty double precision NOT NULL DEFAULT 0, due_at timestamptz NOT NULL,
			last_reviewed_at timestamptz, reps int NOT NULL DEFAULT 0, lapses int NOT NULL DEFAULT 0,
			created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
			deleted_at timestamptz);
		CREATE TABLE scheduling.user_scheduler_prefs (
			user_id uuid PRIMARY KEY, desired_retention double precision NOT NULL DEFAULT 0.9,
			daily_new_limit int NOT NULL DEFAULT 20, daily_review_limit int NOT NULL DEFAULT 200,
			timezone text NOT NULL DEFAULT 'UTC', updated_at timestamptz NOT NULL DEFAULT now());`
	if _, err := conn.Exec(ctx, schema); err != nil {
		t.Fatalf("base schema: %v", err)
	}
	up, err := os.ReadFile("../../../migrations/0009_scheduling_queue.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := conn.Exec(ctx, string(up)); err != nil {
		t.Fatalf("apply 0009: %v", err)
	}
	return dsn, func() { pg.Terminate(ctx) }
}

func TestMigration0009_IndexAndTable(t *testing.T) {
	dsn, done := startPG(t)
	defer done()
	ctx := context.Background()
	conn, _ := pgx.Connect(ctx, dsn)
	defer conn.Close(ctx)

	var n int
	if err := conn.QueryRow(ctx,
		`SELECT count(*) FROM pg_indexes WHERE schemaname='scheduling' AND indexname='idx_cards_owner_due'`).Scan(&n); err != nil {
		t.Fatalf("query index: %v", err)
	}
	if n != 1 {
		t.Errorf("expected idx_cards_owner_due to exist, got %d", n)
	}
	if err := conn.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_schema='scheduling' AND table_name='study_profiles'`).Scan(&n); err != nil {
		t.Fatalf("query table: %v", err)
	}
	if n != 1 {
		t.Errorf("expected study_profiles table, got %d", n)
	}
}
```

- [ ] **Step 3: Chạy test → PASS (cần Docker)**

Run: `go test ./internal/scheduling/repo/ -run TestMigration0009 -v`
Expected: PASS (dựng pg, áp base + 0009, thấy index + bảng).

- [ ] **Step 4: Commit**

```bash
git add migrations/0009_scheduling_queue.up.sql migrations/0009_scheduling_queue.down.sql internal/scheduling/repo/migration_test.go
git commit -m "feat(scheduling): index cards(owner_id,due_at) + study_profiles coach flag"
```

---

### Task 2: Domain — Retrievability (đường quên FSRS, thuần) — TDD

**Files:**
- Create: `internal/scheduling/domain/retrievability.go`
- Test: `internal/scheduling/domain/retrievability_test.go`

- [ ] **Step 1: Viết test thất bại (table)**

Create `internal/scheduling/domain/retrievability_test.go`:
```go
package domain

import (
	"math"
	"testing"
	"time"
)

func TestRetrievability(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		card Card
		now  time.Time
		want float64
		tol  float64
	}{
		{"t=S ⇒ R≈0.9 (bất biến FSRS)", Card{Stability: 10, LastReviewedAt: base}, base.AddDate(0, 0, 10), 0.90, 0.005},
		{"t=0 ⇒ R=1", Card{Stability: 10, LastReviewedAt: base}, base, 1.0, 0.0001},
		{"stability=0 ⇒ cấp thiết nhất R=0", Card{Stability: 0, LastReviewedAt: base}, base.AddDate(0, 0, 1), 0.0, 0.0001},
		{"elapsed âm bị kẹp về 0 ⇒ R=1", Card{Stability: 5, LastReviewedAt: base}, base.Add(-2 * time.Hour), 1.0, 0.0001},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Retrievability(tc.card, tc.now)
			if math.Abs(got-tc.want) > tc.tol {
				t.Errorf("Retrievability = %v, want %v (±%v)", got, tc.want, tc.tol)
			}
		})
	}
}

func TestRetrievability_MonotoneDecreasing(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	c := Card{Stability: 5, LastReviewedAt: base}
	r1 := Retrievability(c, base.AddDate(0, 0, 2))
	r2 := Retrievability(c, base.AddDate(0, 0, 20))
	if !(r2 < r1) {
		t.Errorf("R phải giảm khi càng quá hạn: r@2d=%v r@20d=%v", r1, r2)
	}
}
```

- [ ] **Step 2: Chạy test → FAIL**

Run: `go test ./internal/scheduling/domain/ -run TestRetrievability -v`
Expected: FAIL (build error — `Retrievability` undefined).

- [ ] **Step 3: Viết implementation**

Create `internal/scheduling/domain/retrievability.go`:
```go
package domain

import (
	"math"
	"time"
)

// Tham số đường quên FSRS-4.5/5: R(t) = (1 + FACTOR * t/S)^DECAY.
const (
	fsrsFactor = 19.0 / 81.0
	fsrsDecay  = -0.5
)

// Retrievability trả xác suất nhớ lại R∈(0,1] tại now, với t = số ngày kể từ lần
// ôn gần nhất, S = stability. R thấp = càng sắp quên = càng cấp thiết. Thuần, dùng
// để XẾP ƯU TIÊN queue; toán reschedule vẫn qua SchedulerPort (AD-7), không ở đây.
func Retrievability(c Card, now time.Time) float64 {
	if c.Stability <= 0 {
		return 0 // chưa có stability ⇒ coi cấp thiết nhất
	}
	elapsedDays := now.Sub(c.LastReviewedAt).Hours() / 24.0
	if elapsedDays < 0 {
		elapsedDays = 0
	}
	return math.Pow(1+fsrsFactor*elapsedDays/c.Stability, fsrsDecay)
}
```

- [ ] **Step 4: Chạy test → PASS**

Run: `go test ./internal/scheduling/domain/ -run TestRetrievability -v`
Expected: PASS (5 sub-tests + monotone).

- [ ] **Step 5: Commit**

```bash
git add internal/scheduling/domain/retrievability.go internal/scheduling/domain/retrievability_test.go
git commit -m "feat(scheduling): FSRS retrievability for queue prioritization (pure)"
```

---

### Task 3: Domain — StartOfStudyDay (ranh giới "ngày học" theo TZ, AD-12) — TDD

**Files:**
- Create: `internal/scheduling/domain/studyday.go`
- Test: `internal/scheduling/domain/studyday_test.go`

- [ ] **Step 1: Viết test thất bại**

Create `internal/scheduling/domain/studyday_test.go`:
```go
package domain

import (
	"testing"
	"time"
)

func TestStartOfStudyDay_UserTZBoundary(t *testing.T) {
	// 02:00Z = 09:00 giờ VN cùng ngày ⇒ đầu ngày học = 2026-07-07 00:00+07 = 2026-07-06 17:00Z.
	now := time.Date(2026, 7, 7, 2, 0, 0, 0, time.UTC)
	got, err := StartOfStudyDay(now, "Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := time.Date(2026, 7, 6, 17, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("StartOfStudyDay = %v, want %v", got.UTC(), want)
	}
}

func TestStartOfStudyDay_LocalMidnightDSTZone(t *testing.T) {
	// TZ có DST: kết quả luôn là 00:00 giờ địa phương (không phải 00:00Z).
	now := time.Date(2026, 3, 8, 18, 0, 0, 0, time.UTC)
	got, err := StartOfStudyDay(now, "America/New_York")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Hour() != 0 || got.Minute() != 0 {
		t.Errorf("phải là nửa đêm giờ địa phương, got %v", got)
	}
}

func TestStartOfStudyDay_InvalidTZ(t *testing.T) {
	if _, err := StartOfStudyDay(time.Now(), "Not/AZone"); err == nil {
		t.Error("expected error cho TZ không hợp lệ")
	}
}
```

- [ ] **Step 2: Chạy test → FAIL**

Run: `go test ./internal/scheduling/domain/ -run TestStartOfStudyDay -v`
Expected: FAIL (`StartOfStudyDay` undefined).

- [ ] **Step 3: Viết implementation**

Create `internal/scheduling/domain/studyday.go`:
```go
package domain

import "time"

// StartOfStudyDay trả mốc 00:00 "ngày học" theo TZ người dùng (AD-12), an toàn DST.
// So Due dùng server-time; đếm daily-limit dùng ngày học này.
func StartOfStudyDay(now time.Time, tz string) (time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, err
	}
	l := now.In(loc)
	y, m, d := l.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc), nil
}
```

- [ ] **Step 4: Chạy test → PASS**

Run: `go test ./internal/scheduling/domain/ -run TestStartOfStudyDay -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/scheduling/domain/studyday.go internal/scheduling/domain/studyday_test.go
git commit -m "feat(scheduling): study-day boundary per user timezone (AD-12)"
```

---

### Task 4: Domain — BuildQueue ưu tiên + giới hạn ngày (Story 4.1/4.2/4.3) — TDD

**Files:**
- Create: `internal/scheduling/domain/queue.go`
- Test: `internal/scheduling/domain/queue_test.go`

- [ ] **Step 1: Viết test thất bại (table + limits + ApplyDayCounts)**

Create `internal/scheduling/domain/queue_test.go`:
```go
package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func mkCard(state CardState, dueOffsetDays float64, stability float64, created time.Time, now time.Time) Card {
	return Card{
		ID:             uuid.New(),
		OwnerID:        uuid.Nil,
		State:          state,
		Stability:      stability,
		DueAt:          now.Add(time.Duration(dueOffsetDays * 24 * float64(time.Hour))),
		LastReviewedAt: now.Add(time.Duration(dueOffsetDays * 24 * float64(time.Hour))).Add(-time.Duration(stability * 24 * float64(time.Hour))),
		CreatedAt:      created,
	}
}

func TestBuildQueue_PriorityOrder(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 5, DailyReviewLimit: 50}

	// overdue: due trước đầu ngày hôm nay; R thấp (stability nhỏ) phải đứng trước.
	overdueLowR := mkCard(StateReview, -3, 1, now, now)    // R rất thấp
	overdueHighR := mkCard(StateReview, -4, 100, now, now) // quá hạn lâu hơn nhưng S lớn ⇒ R cao hơn
	relearn := mkCard(StateRelearning, -0.02, 2, now, now) // due ~30' trước, trong ngày
	reviewDue := mkCard(StateReview, -0.05, 5, now, now)   // due trong hôm nay, sau đầu ngày
	newA := mkCard(StateNew, 0, 0, now.Add(-2*time.Hour), now)
	newB := mkCard(StateNew, 0, 0, now.Add(-1*time.Hour), now)

	res := BuildQueue([]Card{newB, reviewDue, overdueHighR, relearn, overdueLowR, newA}, prefs, now)

	order := make([]CardState, len(res.Cards))
	for i, c := range res.Cards {
		order[i] = c.State
	}
	// Kỳ vọng: overdue(2) → relearning(1) → review-due(1) → new(2).
	if len(res.Cards) != 6 {
		t.Fatalf("len = %d, want 6 (%v)", len(res.Cards), order)
	}
	if res.Cards[0].ID != overdueLowR.ID {
		t.Errorf("overdue R-thấp phải đứng đầu, got %v", res.Cards[0].ID)
	}
	if res.Cards[1].ID != overdueHighR.ID {
		t.Errorf("overdue R-cao phải sau, got %v", res.Cards[1].ID)
	}
	if res.Cards[2].State != StateRelearning {
		t.Errorf("vị trí 2 phải relearning, got %v", res.Cards[2].State)
	}
	if res.Cards[3].State != StateReview || res.Cards[3].ID != reviewDue.ID {
		t.Errorf("vị trí 3 phải review-due, got %v", res.Cards[3].ID)
	}
	if res.Cards[4].State != StateNew || res.Cards[5].State != StateNew {
		t.Errorf("2 thẻ cuối phải là new, got %v", order)
	}
	if res.Cards[4].ID != newA.ID {
		t.Errorf("new sắp theo created_at (newA trước), got %v", res.Cards[4].ID)
	}
	if res.NewCount != 2 || res.ReviewCount != 4 {
		t.Errorf("counts = new %d review %d, want 2/4", res.NewCount, res.ReviewCount)
	}
}

func TestBuildQueue_RespectsDailyLimits(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 2, DailyReviewLimit: 3}
	var cards []Card
	for i := 0; i < 10; i++ {
		cards = append(cards, mkCard(StateReview, -1, 5, now, now))
	}
	for i := 0; i < 10; i++ {
		cards = append(cards, mkCard(StateNew, 0, 0, now.Add(-time.Duration(i)*time.Minute), now))
	}
	res := BuildQueue(cards, prefs, now)
	if res.ReviewCount != 3 {
		t.Errorf("review cap = %d, want 3", res.ReviewCount)
	}
	if res.NewCount != 2 {
		t.Errorf("new cap = %d, want 2", res.NewCount)
	}
	if len(res.Cards) != 5 {
		t.Errorf("total = %d, want 5", len(res.Cards))
	}
}

func TestBuildQueue_ExcludesFutureAndDeleted(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	future := mkCard(StateReview, 2, 5, now, now) // due 2 ngày tới ⇒ loại
	res := BuildQueue([]Card{future}, prefs, now)
	if len(res.Cards) != 0 {
		t.Errorf("thẻ tương lai không được vào queue, got %d", len(res.Cards))
	}
}

func TestApplyDayCounts_SubtractsServed(t *testing.T) {
	prefs := SchedulerPrefs{DailyNewLimit: 20, DailyReviewLimit: 200}
	got := ApplyDayCounts(prefs, DayCounts{NewServed: 5, ReviewServed: 50})
	if got.DailyNewLimit != 15 || got.DailyReviewLimit != 150 {
		t.Errorf("effective = new %d review %d, want 15/150", got.DailyNewLimit, got.DailyReviewLimit)
	}
	// không âm khi đã vượt hạn
	got2 := ApplyDayCounts(prefs, DayCounts{NewServed: 999, ReviewServed: 999})
	if got2.DailyNewLimit != 0 || got2.DailyReviewLimit != 0 {
		t.Errorf("phải kẹp về 0, got %d/%d", got2.DailyNewLimit, got2.DailyReviewLimit)
	}
}
```

- [ ] **Step 2: Chạy test → FAIL**

Run: `go test ./internal/scheduling/domain/ -run 'TestBuildQueue|TestApplyDayCounts' -v`
Expected: FAIL (`BuildQueue`/`QueueResult`/`DayCounts`/`ApplyDayCounts` undefined).

- [ ] **Step 3: Viết implementation**

Create `internal/scheduling/domain/queue.go`:
```go
package domain

import (
	"sort"
	"time"
)

// QueueResult là queue đã sắp xếp + đếm loại.
type QueueResult struct {
	Cards       []Card
	NewCount    int
	ReviewCount int
}

// DayCounts = số thẻ đã phục vụ hôm nay (đếm theo ngày học TZ user), từ review_logs.
type DayCounts struct {
	NewServed    int
	ReviewServed int
}

// ApplyDayCounts trừ hạn ngày còn lại theo số đã phục vụ hôm nay (kẹp ≥0).
func ApplyDayCounts(prefs SchedulerPrefs, counts DayCounts) SchedulerPrefs {
	p := prefs
	p.DailyNewLimit = clampMin0(prefs.DailyNewLimit - counts.NewServed)
	p.DailyReviewLimit = clampMin0(prefs.DailyReviewLimit - counts.ReviewServed)
	return p
}

func clampMin0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

// BuildQueue dựng queue ưu tiên THUẦN (Story 4.1): overdue nặng (R thấp) → relearning
// → review đến hạn → new. Tôn trọng hạn ngày trong prefs — gọi ApplyDayCounts trước
// để prefs là "ngân sách còn lại hôm nay" (Story 4.2/4.3: rải new, giữ phần dư qua ngày sau).
func BuildQueue(cards []Card, prefs SchedulerPrefs, now time.Time) QueueResult {
	dayStart, err := StartOfStudyDay(now, prefs.Timezone)
	if err != nil {
		dayStart = now.UTC().Truncate(24 * time.Hour)
	}

	var overdue, relearning, reviewDue, newCards []Card
	for _, c := range cards {
		switch c.State {
		case StateNew:
			newCards = append(newCards, c)
		case StateRelearning:
			if !c.DueAt.After(now) {
				relearning = append(relearning, c)
			}
		case StateLearning, StateReview:
			switch {
			case c.DueAt.Before(dayStart):
				overdue = append(overdue, c)
			case !c.DueAt.After(now):
				reviewDue = append(reviewDue, c)
			}
		}
	}

	sort.SliceStable(overdue, func(i, j int) bool {
		return Retrievability(overdue[i], now) < Retrievability(overdue[j], now)
	})
	sort.SliceStable(relearning, func(i, j int) bool { return relearning[i].DueAt.Before(relearning[j].DueAt) })
	sort.SliceStable(reviewDue, func(i, j int) bool { return reviewDue[i].DueAt.Before(reviewDue[j].DueAt) })
	sort.SliceStable(newCards, func(i, j int) bool { return newCards[i].CreatedAt.Before(newCards[j].CreatedAt) })

	review := make([]Card, 0, len(overdue)+len(relearning)+len(reviewDue))
	review = append(review, overdue...)
	review = append(review, relearning...)
	review = append(review, reviewDue...)
	if len(review) > prefs.DailyReviewLimit {
		review = review[:prefs.DailyReviewLimit]
	}
	if len(newCards) > prefs.DailyNewLimit {
		newCards = newCards[:prefs.DailyNewLimit]
	}

	out := make([]Card, 0, len(review)+len(newCards))
	out = append(out, review...)
	out = append(out, newCards...)
	return QueueResult{Cards: out, NewCount: len(newCards), ReviewCount: len(review)}
}
```

- [ ] **Step 4: Chạy test → PASS**

Run: `go test ./internal/scheduling/domain/ -run 'TestBuildQueue|TestApplyDayCounts' -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/scheduling/domain/queue.go internal/scheduling/domain/queue_test.go
git commit -m "feat(scheduling): priority queue with daily new/review limits (FR-25/26/27)"
```

---

### Task 5: Domain — BuildQueue performance p95<500ms cho 10k thẻ (NFR-2) — TDD

**Files:**
- Test: `internal/scheduling/domain/queue_perf_test.go`

- [ ] **Step 1: Viết test đo p95 + benchmark**

Create `internal/scheduling/domain/queue_perf_test.go`:
```go
package domain

import (
	"math/rand"
	"sort"
	"testing"
	"time"
)

func gen10k(now time.Time) []Card {
	r := rand.New(rand.NewSource(42))
	cards := make([]Card, 0, 10000)
	states := []CardState{StateNew, StateLearning, StateReview, StateRelearning}
	for i := 0; i < 10000; i++ {
		st := states[r.Intn(len(states))]
		dueOffset := time.Duration(r.Intn(20)-15) * 24 * time.Hour // -15..+4 ngày
		cards = append(cards, Card{
			State:          st,
			Stability:      1 + r.Float64()*200,
			DueAt:          now.Add(dueOffset),
			LastReviewedAt: now.Add(dueOffset - 48*time.Hour),
			CreatedAt:      now.Add(-time.Duration(r.Intn(1000)) * time.Hour),
		})
	}
	return cards
}

func TestBuildQueue_Performance10k(t *testing.T) {
	if testing.Short() {
		t.Skip("skip perf test in -short mode")
	}
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	cards := gen10k(now)

	const iters = 30
	durs := make([]time.Duration, iters)
	for i := 0; i < iters; i++ {
		start := time.Now()
		_ = BuildQueue(cards, prefs, now)
		durs[i] = time.Since(start)
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	p95 := durs[int(float64(iters)*0.95)-1]
	if p95 > 500*time.Millisecond {
		t.Errorf("NFR-2 vi phạm: p95 BuildQueue(10k) = %v, want <500ms", p95)
	}
	t.Logf("BuildQueue(10k) p95 = %v", p95)
}

func BenchmarkBuildQueue10k(b *testing.B) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	cards := gen10k(now)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildQueue(cards, prefs, now)
	}
}
```

- [ ] **Step 2: Chạy test → PASS (đo thực)**

Run: `go test ./internal/scheduling/domain/ -run TestBuildQueue_Performance10k -v`
Expected: PASS; log `p95 = <vài ms>` (thuần CPU, dưới ngưỡng 500ms rất xa).

Run (tham khảo): `go test ./internal/scheduling/domain/ -run=^$ -bench BenchmarkBuildQueue10k -benchmem`

- [ ] **Step 3: Commit**

```bash
git add internal/scheduling/domain/queue_perf_test.go
git commit -m "test(scheduling): BuildQueue p95<500ms for 10k cards (NFR-2)"
```

---

### Task 6: Domain — PlanAntiFlood chống nổ queue sau nghỉ (Story 4.4) — TDD

**Files:**
- Create: `internal/scheduling/domain/antiflood.go`
- Test: `internal/scheduling/domain/antiflood_test.go`

- [ ] **Step 1: Viết test thất bại (bao gồm "về sau 5 ngày, 300 overdue")**

Create `internal/scheduling/domain/antiflood_test.go`:
```go
package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func overdueSet(n int, now time.Time) []Card {
	cards := make([]Card, n)
	for i := 0; i < n; i++ {
		// stability tăng dần ⇒ R tăng dần ⇒ thẻ i nhỏ = R thấp = cấp thiết hơn.
		cards[i] = Card{
			ID:             uuid.New(),
			State:          StateReview,
			Stability:      float64(i + 1),
			DueAt:          now.AddDate(0, 0, -5),
			LastReviewedAt: now.AddDate(0, 0, -6),
		}
	}
	return cards
}

func TestPlanAntiFlood_300OverdueAfter5Days_UnderCap(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyReviewLimit: 200} // cap = 2×200 = 400
	overdue := overdueSet(300, now)

	plan := PlanAntiFlood(overdue, prefs, now)
	// 300 ≤ 400 ⇒ tất cả hiển thị hôm nay, không rải; và sắp R thấp trước.
	if len(plan.Today) != 300 {
		t.Fatalf("Today = %d, want 300 (dưới cap)", len(plan.Today))
	}
	if len(plan.Deferred) != 0 {
		t.Errorf("Deferred = %d, want 0", len(plan.Deferred))
	}
	if Retrievability(plan.Today[0], now) > Retrievability(plan.Today[len(plan.Today)-1], now) {
		t.Error("Today phải sắp theo R tăng dần (R thấp nhất đứng đầu)")
	}
}

func TestPlanAntiFlood_SpreadsRemainderOver7Days(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := SchedulerPrefs{Timezone: "UTC", DailyReviewLimit: 200} // cap 400
	overdue := overdueSet(1000, now)

	plan := PlanAntiFlood(overdue, prefs, now)
	if len(plan.Today) != 400 {
		t.Fatalf("Today = %d, want 400 (cap)", len(plan.Today))
	}
	if len(plan.Deferred) != 600 {
		t.Fatalf("Deferred = %d, want 600", len(plan.Deferred))
	}
	// mọi deferred rơi vào 1..7 ngày sau đầu ngày học, không dồn 1 ngày.
	dayStart, _ := StartOfStudyDay(now, prefs.Timezone)
	minDue, maxDue := plan.Deferred[0].NewDueAt, plan.Deferred[0].NewDueAt
	for _, d := range plan.Deferred {
		off := int(d.NewDueAt.Sub(dayStart).Hours() / 24)
		if off < 1 || off > AntiFloodSpreadDays {
			t.Errorf("deferred offset %d ngoài [1,%d]", off, AntiFloodSpreadDays)
		}
		if d.NewDueAt.Before(minDue) {
			minDue = d.NewDueAt
		}
		if d.NewDueAt.After(maxDue) {
			maxDue = d.NewDueAt
		}
	}
	if minDue.Equal(maxDue) {
		t.Error("phần dư phải rải nhiều ngày, không dồn 1 ngày")
	}
	// thẻ R thấp nhất trong phần dư được xếp ngày sớm nhất.
	if !plan.Deferred[0].NewDueAt.Equal(minDue) {
		t.Error("phần dư R thấp nhất phải vào ngày sớm nhất")
	}
}
```

- [ ] **Step 2: Chạy test → FAIL**

Run: `go test ./internal/scheduling/domain/ -run TestPlanAntiFlood -v`
Expected: FAIL (`PlanAntiFlood`/`AntiFloodPlan`/`DeferredCard`/`AntiFloodSpreadDays` undefined).

- [ ] **Step 3: Viết implementation**

Create `internal/scheduling/domain/antiflood.go`:
```go
package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// AntiFloodSpreadDays = số ngày tối đa rải phần overdue dư (FR-28: ≤7 ngày).
const AntiFloodSpreadDays = 7

type DeferredCard struct {
	CardID   uuid.UUID
	NewDueAt time.Time
}

type AntiFloodPlan struct {
	Today    []Card         // hiển thị hôm nay (≤ 2×review-limit, R thấp trước)
	Deferred []DeferredCard // phần dư, due mới rải qua ≤7 ngày
	DayStart time.Time
}

// PlanAntiFlood chống nổ queue sau nghỉ dài (Story 4.4, FR-28) — THUẦN. Giữ tối đa
// 2×review-limit thẻ overdue (ưu tiên R thấp nhất) cho hôm nay; rải phần dư đều qua
// tối đa 7 ngày kế, R thấp hơn nhận ngày sớm hơn. Không dồn toàn bộ overdue một ngày.
func PlanAntiFlood(overdue []Card, prefs SchedulerPrefs, now time.Time) AntiFloodPlan {
	dayStart, err := StartOfStudyDay(now, prefs.Timezone)
	if err != nil {
		dayStart = now.UTC().Truncate(24 * time.Hour)
	}

	sorted := make([]Card, len(overdue))
	copy(sorted, overdue)
	sort.SliceStable(sorted, func(i, j int) bool {
		return Retrievability(sorted[i], now) < Retrievability(sorted[j], now)
	})

	limit := 2 * prefs.DailyReviewLimit
	if limit < 0 {
		limit = 0
	}
	if len(sorted) <= limit {
		return AntiFloodPlan{Today: sorted, DayStart: dayStart}
	}

	today := sorted[:limit]
	rest := sorted[limit:]
	perDay := (len(rest) + AntiFloodSpreadDays - 1) / AntiFloodSpreadDays
	if perDay < 1 {
		perDay = 1
	}
	deferred := make([]DeferredCard, 0, len(rest))
	for i, c := range rest {
		day := i/perDay + 1
		if day > AntiFloodSpreadDays {
			day = AntiFloodSpreadDays
		}
		deferred = append(deferred, DeferredCard{CardID: c.ID, NewDueAt: dayStart.AddDate(0, 0, day)})
	}
	return AntiFloodPlan{Today: today, Deferred: deferred, DayStart: dayStart}
}
```

- [ ] **Step 4: Chạy test → PASS**

Run: `go test ./internal/scheduling/domain/ -run TestPlanAntiFlood -v`
Expected: PASS (2 tests). Toàn domain: `go test ./internal/scheduling/domain/ -v` xanh.

- [ ] **Step 5: Commit**

```bash
git add internal/scheduling/domain/antiflood.go internal/scheduling/domain/antiflood_test.go
git commit -m "feat(scheduling): anti-flood plan caps overdue at 2x review limit, spreads over 7 days (FR-28)"
```

---

### Task 7: Ports — interface repo/port cho service (compile check)

**Files:**
- Create: `internal/scheduling/ports/ports.go`

- [ ] **Step 1: Viết interface**

Create `internal/scheduling/ports/ports.go`:
```go
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/scheduling/domain"
)

// CardRepo — driven adapter (Postgres) cho thẻ scheduling.
type CardRepo interface {
	// LoadCandidates trả thẻ New + thẻ due (due_at<=now) của user, dùng index cards(owner_id,due_at).
	LoadCandidates(ctx context.Context, ownerID uuid.UUID, now time.Time) ([]domain.Card, error)
	// BulkDefer đẩy due_at các thẻ (anti-flood) trong 1 batch.
	BulkDefer(ctx context.Context, deferred []domain.DeferredCard) error
}

// PrefsRepo — user_scheduler_prefs (đọc + đổi giới hạn ngày, Story 4.2).
type PrefsRepo interface {
	Get(ctx context.Context, userID uuid.UUID) (domain.SchedulerPrefs, error)
	UpdateLimits(ctx context.Context, userID uuid.UUID, newLimit, reviewLimit int) (domain.SchedulerPrefs, error)
}

// ReviewActivityPort — cross-module (AD-9): scheduling hỏi review đã phục vụ bao nhiêu
// thẻ new/review kể từ đầu ngày học. Review module implement, wire ở cmd/api.
type ReviewActivityPort interface {
	CountServedSince(ctx context.Context, userID uuid.UUID, since time.Time) (domain.DayCounts, error)
}

// StudyProfileRepo — cờ "đã xem coach lần đầu" (Story 4.5).
type StudyProfileRepo interface {
	CoachSeen(ctx context.Context, userID uuid.UUID) (bool, error)
	MarkCoachSeen(ctx context.Context, userID uuid.UUID) error
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/scheduling/...`
Expected: no error.

- [ ] **Step 3: Commit**

```bash
git add internal/scheduling/ports/ports.go
git commit -m "feat(scheduling): repo + cross-module port interfaces for queue service"
```

---

### Task 8: Service — QueueService (anti-flood + limits, fakes) — TDD

**Files:**
- Create: `internal/scheduling/service/queue.go`
- Test: `internal/scheduling/service/queue_test.go`

- [ ] **Step 1: Viết test thất bại (fakes, không DB)**

Create `internal/scheduling/service/queue_test.go`:
```go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/scheduling/domain"
)

type fakeCardRepo struct {
	cards    []domain.Card
	deferred []domain.DeferredCard
}

func (f *fakeCardRepo) LoadCandidates(_ context.Context, _ uuid.UUID, _ time.Time) ([]domain.Card, error) {
	return f.cards, nil
}
func (f *fakeCardRepo) BulkDefer(_ context.Context, d []domain.DeferredCard) error {
	f.deferred = append(f.deferred, d...)
	return nil
}

type fakePrefs struct{ p domain.SchedulerPrefs }

func (f fakePrefs) Get(_ context.Context, _ uuid.UUID) (domain.SchedulerPrefs, error) {
	return f.p, nil
}
func (f fakePrefs) UpdateLimits(_ context.Context, _ uuid.UUID, n, r int) (domain.SchedulerPrefs, error) {
	f.p.DailyNewLimit, f.p.DailyReviewLimit = n, r
	return f.p, nil
}

type fakeActivity struct{ counts domain.DayCounts }

func (f fakeActivity) CountServedSince(_ context.Context, _ uuid.UUID, _ time.Time) (domain.DayCounts, error) {
	return f.counts, nil
}

func TestQueueService_SubtractsServedToday(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := domain.SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	var cards []domain.Card
	for i := 0; i < 30; i++ {
		cards = append(cards, domain.Card{ID: uuid.New(), State: domain.StateReview, Stability: 5, DueAt: now.Add(-time.Hour), LastReviewedAt: now.Add(-48 * time.Hour)})
	}
	repo := &fakeCardRepo{cards: cards}
	svc := NewQueueService(repo, fakePrefs{p: prefs}, fakeActivity{counts: domain.DayCounts{ReviewServed: 195}})

	res, err := svc.BuildToday(context.Background(), uuid.New(), now)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// còn 200-195 = 5 review được phục vụ hôm nay.
	if res.ReviewCount != 5 {
		t.Errorf("ReviewCount = %d, want 5 (200-195 served)", res.ReviewCount)
	}
}

func TestQueueService_TriggersAntiFloodAndDefers(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := domain.SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	// 1000 overdue (due trước đầu ngày) ⇒ > 2×200 ⇒ rải bớt.
	var cards []domain.Card
	for i := 0; i < 1000; i++ {
		cards = append(cards, domain.Card{ID: uuid.New(), State: domain.StateReview, Stability: float64(i + 1), DueAt: now.AddDate(0, 0, -3), LastReviewedAt: now.AddDate(0, 0, -4)})
	}
	repo := &fakeCardRepo{cards: cards}
	svc := NewQueueService(repo, fakePrefs{p: prefs}, fakeActivity{})

	res, err := svc.BuildToday(context.Background(), uuid.New(), now)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(repo.deferred) != 600 {
		t.Errorf("BulkDefer nhận %d, want 600 (1000-400)", len(repo.deferred))
	}
	// sau khi rải, queue hôm nay còn ≤ review limit (200).
	if res.ReviewCount != 200 {
		t.Errorf("ReviewCount = %d, want 200", res.ReviewCount)
	}
}
```

- [ ] **Step 2: Chạy test → FAIL**

Run: `go test ./internal/scheduling/service/ -run TestQueueService -v`
Expected: FAIL (`NewQueueService`/`BuildToday` undefined).

- [ ] **Step 3: Viết implementation**

Create `internal/scheduling/service/queue.go`:
```go
package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/ports"
)

type QueueService struct {
	cards  ports.CardRepo
	prefs  ports.PrefsRepo
	review ports.ReviewActivityPort
}

func NewQueueService(c ports.CardRepo, p ports.PrefsRepo, r ports.ReviewActivityPort) *QueueService {
	return &QueueService{cards: c, prefs: p, review: r}
}

// BuildToday dựng queue hôm nay: nạp candidate → đếm đã phục vụ (TZ user) → chống nổ
// (rải overdue dư) → BuildQueue với ngân sách còn lại. Story 4.1/4.2/4.3/4.4.
func (s *QueueService) BuildToday(ctx context.Context, userID uuid.UUID, now time.Time) (domain.QueueResult, error) {
	prefs, err := s.prefs.Get(ctx, userID)
	if err != nil {
		return domain.QueueResult{}, err
	}
	dayStart, err := domain.StartOfStudyDay(now, prefs.Timezone)
	if err != nil {
		return domain.QueueResult{}, err
	}
	counts, err := s.review.CountServedSince(ctx, userID, dayStart)
	if err != nil {
		return domain.QueueResult{}, err
	}
	cards, err := s.cards.LoadCandidates(ctx, userID, now)
	if err != nil {
		return domain.QueueResult{}, err
	}

	overdue := overdueCards(cards, dayStart)
	if len(overdue) > 2*prefs.DailyReviewLimit {
		plan := domain.PlanAntiFlood(overdue, prefs, now)
		if len(plan.Deferred) > 0 {
			if err := s.cards.BulkDefer(ctx, plan.Deferred); err != nil {
				return domain.QueueResult{}, err
			}
			cards = dropDeferred(cards, plan.Deferred)
		}
	}

	effective := domain.ApplyDayCounts(prefs, counts)
	return domain.BuildQueue(cards, effective, now), nil
}

func overdueCards(cards []domain.Card, dayStart time.Time) []domain.Card {
	var out []domain.Card
	for _, c := range cards {
		if (c.State == domain.StateReview || c.State == domain.StateLearning) && c.DueAt.Before(dayStart) {
			out = append(out, c)
		}
	}
	return out
}

func dropDeferred(cards []domain.Card, deferred []domain.DeferredCard) []domain.Card {
	skip := make(map[uuid.UUID]bool, len(deferred))
	for _, d := range deferred {
		skip[d.CardID] = true
	}
	kept := make([]domain.Card, 0, len(cards))
	for _, c := range cards {
		if !skip[c.ID] {
			kept = append(kept, c)
		}
	}
	return kept
}
```

- [ ] **Step 4: Chạy test → PASS**

Run: `go test ./internal/scheduling/service/ -run TestQueueService -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/scheduling/service/queue.go internal/scheduling/service/queue_test.go
git commit -m "feat(scheduling): queue service orchestrates limits + anti-flood (AD-9, AD-12)"
```

---

### Task 9: Service — LearnService + coach flag (Story 4.5, fakes) — TDD

**Files:**
- Create: `internal/scheduling/service/learn.go`
- Test: `internal/scheduling/service/learn_test.go`

- [ ] **Step 1: Viết test thất bại**

Create `internal/scheduling/service/learn_test.go`:
```go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/scheduling/domain"
)

type fakeProfiles struct {
	seen   bool
	marked bool
}

func (f *fakeProfiles) CoachSeen(_ context.Context, _ uuid.UUID) (bool, error) { return f.seen, nil }
func (f *fakeProfiles) MarkCoachSeen(_ context.Context, _ uuid.UUID) error {
	f.marked = true
	return nil
}

func TestLearnService_NewCardsUpToRemainingLimit(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := domain.SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	var cards []domain.Card
	for i := 0; i < 50; i++ {
		cards = append(cards, domain.Card{ID: uuid.New(), State: domain.StateNew, CreatedAt: now.Add(-time.Duration(i) * time.Minute)})
	}
	cards = append(cards, domain.Card{ID: uuid.New(), State: domain.StateReview, DueAt: now.Add(-time.Hour)}) // không phải new
	prof := &fakeProfiles{seen: false}
	svc := NewLearnService(&fakeCardRepo{cards: cards}, fakePrefs{p: prefs}, fakeActivity{counts: domain.DayCounts{NewServed: 5}}, prof)

	sess, err := svc.StartSession(context.Background(), uuid.New(), now)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(sess.Cards) != 15 { // 20 - 5 đã học hôm nay
		t.Errorf("new session = %d thẻ, want 15", len(sess.Cards))
	}
	for _, c := range sess.Cards {
		if c.State != domain.StateNew {
			t.Errorf("chỉ được trả thẻ New, got state %v", c.State)
		}
	}
	if !sess.ShowCoach {
		t.Error("lần đầu phải ShowCoach=true")
	}
}

func TestLearnService_AckHidesCoach(t *testing.T) {
	prof := &fakeProfiles{seen: false}
	svc := NewLearnService(&fakeCardRepo{}, fakePrefs{}, fakeActivity{}, prof)
	if err := svc.AckCoach(context.Background(), uuid.New()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !prof.marked {
		t.Error("AckCoach phải gọi MarkCoachSeen")
	}
}

func TestLearnService_CoachHiddenWhenSeen(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	prefs := domain.SchedulerPrefs{Timezone: "UTC", DailyNewLimit: 20, DailyReviewLimit: 200}
	svc := NewLearnService(&fakeCardRepo{}, fakePrefs{p: prefs}, fakeActivity{}, &fakeProfiles{seen: true})
	sess, _ := svc.StartSession(context.Background(), uuid.New(), now)
	if sess.ShowCoach {
		t.Error("đã xem coach ⇒ ShowCoach=false")
	}
}
```

- [ ] **Step 2: Chạy test → FAIL**

Run: `go test ./internal/scheduling/service/ -run TestLearnService -v`
Expected: FAIL (`NewLearnService`/`StartSession`/`AckCoach` undefined).

- [ ] **Step 3: Viết implementation**

Create `internal/scheduling/service/learn.go`:
```go
package service

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/ports"
)

type LearnSession struct {
	Cards     []domain.Card
	ShowCoach bool
}

type LearnService struct {
	cards    ports.CardRepo
	prefs    ports.PrefsRepo
	review   ports.ReviewActivityPort
	profiles ports.StudyProfileRepo
}

func NewLearnService(c ports.CardRepo, p ports.PrefsRepo, r ports.ReviewActivityPort, sp ports.StudyProfileRepo) *LearnService {
	return &LearnService{cards: c, prefs: p, review: r, profiles: sp}
}

// StartSession trả luồng học thẻ mới RIÊNG (Story 4.5): chỉ thẻ New tới hạn new còn
// lại hôm nay (rải theo giới hạn — FR-27), kèm cờ ShowCoach cho lần đầu (FR-29).
func (s *LearnService) StartSession(ctx context.Context, userID uuid.UUID, now time.Time) (LearnSession, error) {
	prefs, err := s.prefs.Get(ctx, userID)
	if err != nil {
		return LearnSession{}, err
	}
	dayStart, err := domain.StartOfStudyDay(now, prefs.Timezone)
	if err != nil {
		return LearnSession{}, err
	}
	counts, err := s.review.CountServedSince(ctx, userID, dayStart)
	if err != nil {
		return LearnSession{}, err
	}
	remaining := prefs.DailyNewLimit - counts.NewServed
	if remaining < 0 {
		remaining = 0
	}

	cards, err := s.cards.LoadCandidates(ctx, userID, now)
	if err != nil {
		return LearnSession{}, err
	}
	newCards := make([]domain.Card, 0, remaining)
	for _, c := range cards {
		if c.State == domain.StateNew {
			newCards = append(newCards, c)
		}
	}
	sort.SliceStable(newCards, func(i, j int) bool { return newCards[i].CreatedAt.Before(newCards[j].CreatedAt) })
	if len(newCards) > remaining {
		newCards = newCards[:remaining]
	}

	seen, err := s.profiles.CoachSeen(ctx, userID)
	if err != nil {
		return LearnSession{}, err
	}
	return LearnSession{Cards: newCards, ShowCoach: !seen}, nil
}

// AckCoach ghi nhận đã xem hướng dẫn (persist "seen coach").
func (s *LearnService) AckCoach(ctx context.Context, userID uuid.UUID) error {
	return s.profiles.MarkCoachSeen(ctx, userID)
}
```

- [ ] **Step 4: Chạy test → PASS**

Run: `go test ./internal/scheduling/service/ -v`
Expected: PASS (QueueService + LearnService).

- [ ] **Step 5: Commit**

```bash
git add internal/scheduling/service/learn.go internal/scheduling/service/learn_test.go
git commit -m "feat(scheduling): learn session with new-card spread + first-time coach flag (FR-29)"
```

---

### Task 10: Repo — pgx adapters (scheduling) + ReviewActivity adapter (testcontainers)

**Files:**
- Create: `internal/scheduling/repo/repo.go`
- Create: `internal/review/service/activity.go`
- Test: `internal/scheduling/repo/repo_test.go`
- Create (sqlc source, codegen về sau): `db/queries/scheduling/queue.sql`

- [ ] **Step 1: Thêm deps (nếu chưa có từ Sprint trước)**

Run:
```bash
go get github.com/jackc/pgx/v5@latest
go get github.com/jackc/pgx/v5/pgxpool@latest
```

- [ ] **Step 2: Viết sqlc source (nguồn chân lý query, gen về repo/gen — S7)**

Create `db/queries/scheduling/queue.sql`:
```sql
-- name: LoadCandidates :many
SELECT id, owner_id, entry_id, state, stability, difficulty, due_at,
       COALESCE(last_reviewed_at, 'epoch'::timestamptz), reps, lapses, created_at
FROM scheduling.cards
WHERE owner_id = $1 AND deleted_at IS NULL AND (state = 0 OR due_at <= $2)
ORDER BY owner_id, due_at;

-- name: DeferCard :exec
UPDATE scheduling.cards SET due_at = $2, updated_at = now() WHERE id = $1;

-- name: GetPrefs :one
SELECT user_id, desired_retention, daily_new_limit, daily_review_limit, timezone
FROM scheduling.user_scheduler_prefs WHERE user_id = $1;

-- name: UpdateLimits :one
UPDATE scheduling.user_scheduler_prefs
SET daily_new_limit = $2, daily_review_limit = $3, updated_at = now()
WHERE user_id = $1
RETURNING user_id, desired_retention, daily_new_limit, daily_review_limit, timezone;

-- name: CoachSeenAt :one
SELECT learn_coach_seen_at FROM scheduling.study_profiles WHERE user_id = $1;

-- name: MarkCoachSeen :exec
INSERT INTO scheduling.study_profiles (user_id, learn_coach_seen_at)
VALUES ($1, now())
ON CONFLICT (user_id) DO UPDATE SET learn_coach_seen_at = now(), updated_at = now();
```

- [ ] **Step 3: Viết repo adapter (pgx, hiện thực các port Task 7)**

Create `internal/scheduling/repo/repo.go`:
```go
package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/scheduling/domain"
)

type Repo struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) LoadCandidates(ctx context.Context, ownerID uuid.UUID, now time.Time) ([]domain.Card, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, owner_id, entry_id, state, stability, difficulty, due_at,
		       COALESCE(last_reviewed_at, 'epoch'::timestamptz), reps, lapses, created_at
		FROM scheduling.cards
		WHERE owner_id = $1 AND deleted_at IS NULL AND (state = 0 OR due_at <= $2)
		ORDER BY owner_id, due_at`, ownerID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Card
	for rows.Next() {
		var c domain.Card
		if err := rows.Scan(&c.ID, &c.OwnerID, &c.EntryID, &c.State, &c.Stability, &c.Difficulty,
			&c.DueAt, &c.LastReviewedAt, &c.Reps, &c.Lapses, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repo) BulkDefer(ctx context.Context, deferred []domain.DeferredCard) error {
	b := &pgx.Batch{}
	for _, d := range deferred {
		b.Queue(`UPDATE scheduling.cards SET due_at = $2, updated_at = now() WHERE id = $1`, d.CardID, d.NewDueAt)
	}
	br := r.pool.SendBatch(ctx, b)
	defer br.Close()
	for range deferred {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) Get(ctx context.Context, userID uuid.UUID) (domain.SchedulerPrefs, error) {
	var p domain.SchedulerPrefs
	err := r.pool.QueryRow(ctx, `
		SELECT user_id, desired_retention, daily_new_limit, daily_review_limit, timezone
		FROM scheduling.user_scheduler_prefs WHERE user_id = $1`, userID).
		Scan(&p.UserID, &p.DesiredRetention, &p.DailyNewLimit, &p.DailyReviewLimit, &p.Timezone)
	return p, err
}

func (r *Repo) UpdateLimits(ctx context.Context, userID uuid.UUID, newLimit, reviewLimit int) (domain.SchedulerPrefs, error) {
	var p domain.SchedulerPrefs
	err := r.pool.QueryRow(ctx, `
		UPDATE scheduling.user_scheduler_prefs
		SET daily_new_limit = $2, daily_review_limit = $3, updated_at = now()
		WHERE user_id = $1
		RETURNING user_id, desired_retention, daily_new_limit, daily_review_limit, timezone`,
		userID, newLimit, reviewLimit).
		Scan(&p.UserID, &p.DesiredRetention, &p.DailyNewLimit, &p.DailyReviewLimit, &p.Timezone)
	return p, err
}

func (r *Repo) CoachSeen(ctx context.Context, userID uuid.UUID) (bool, error) {
	var seenAt *time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT learn_coach_seen_at FROM scheduling.study_profiles WHERE user_id = $1`, userID).Scan(&seenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return seenAt != nil, nil
}

func (r *Repo) MarkCoachSeen(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO scheduling.study_profiles (user_id, learn_coach_seen_at)
		VALUES ($1, now())
		ON CONFLICT (user_id) DO UPDATE SET learn_coach_seen_at = now(), updated_at = now()`, userID)
	return err
}
```

- [ ] **Step 4: Viết ReviewActivity adapter (review sở hữu review_logs — AD-9)**

Create `internal/review/service/activity.go`:
```go
package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	scheddomain "github.com/memorix/memorix/internal/scheduling/domain"
)

// ActivityAdapter hiện thực scheduling ports.ReviewActivityPort từ review.review_logs.
// Glue composition (wire ở cmd/api); đếm "new served" = log có prev_state=New(0),
// "review served" = phần còn lại. Nguồn chân lý = review_logs (AD-4).
type ActivityAdapter struct{ pool *pgxpool.Pool }

func NewActivityAdapter(pool *pgxpool.Pool) *ActivityAdapter { return &ActivityAdapter{pool: pool} }

func (a *ActivityAdapter) CountServedSince(ctx context.Context, userID uuid.UUID, since time.Time) (scheddomain.DayCounts, error) {
	var d scheddomain.DayCounts
	err := a.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE prev_state = 0),
		  COUNT(*) FILTER (WHERE prev_state <> 0)
		FROM review.review_logs
		WHERE user_id = $1 AND reviewed_at >= $2`, userID, since).
		Scan(&d.NewServed, &d.ReviewServed)
	return d, err
}
```

- [ ] **Step 5: Viết integration test (testcontainers, dùng startPG từ Task 1)**

Create `internal/scheduling/repo/repo_test.go`:
```go
package repo

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/scheduling/domain"
)

func TestRepo_LoadCandidates_OrderedByDue_AndProfileUpsert(t *testing.T) {
	dsn, done := startPG(t)
	defer done()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	owner := uuid.New()
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	seed := []struct {
		state    int
		dueDelta time.Duration
	}{
		{2, -72 * time.Hour}, // overdue
		{0, 240 * time.Hour}, // new (due tương lai vẫn nạp vì state=0)
		{2, -1 * time.Hour},  // due hôm nay
		{2, 48 * time.Hour},  // tương lai ⇒ KHÔNG nạp
	}
	for _, s := range seed {
		_, err := pool.Exec(ctx, `
			INSERT INTO scheduling.cards (id, owner_id, entry_id, state, stability, difficulty, due_at, last_reviewed_at)
			VALUES ($1,$2,$3,$4,5,5,$5,$6)`,
			uuid.New(), owner, uuid.New(), s.state, now.Add(s.dueDelta), now.Add(s.dueDelta-48*time.Hour))
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	_, _ = pool.Exec(ctx, `INSERT INTO scheduling.user_scheduler_prefs (user_id, timezone) VALUES ($1,'UTC')`, owner)

	r := New(pool)
	cards, err := r.LoadCandidates(ctx, owner, now)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cards) != 3 {
		t.Fatalf("nạp %d thẻ, want 3 (loại thẻ due tương lai không phải new)", len(cards))
	}
	for i := 1; i < len(cards); i++ {
		if cards[i].DueAt.Before(cards[i-1].DueAt) {
			t.Errorf("ORDER BY due_at sai tại %d", i)
		}
	}

	// BulkDefer đẩy due
	def := []domain.DeferredCard{{CardID: cards[0].ID, NewDueAt: now.AddDate(0, 0, 3)}}
	if err := r.BulkDefer(ctx, def); err != nil {
		t.Fatalf("defer: %v", err)
	}

	// Prefs update
	p, err := r.UpdateLimits(ctx, owner, 30, 150)
	if err != nil {
		t.Fatalf("update limits: %v", err)
	}
	if p.DailyNewLimit != 30 || p.DailyReviewLimit != 150 {
		t.Errorf("limits = %d/%d, want 30/150", p.DailyNewLimit, p.DailyReviewLimit)
	}

	// Coach flag upsert idempotent
	if seen, _ := r.CoachSeen(ctx, owner); seen {
		t.Error("chưa mark ⇒ CoachSeen=false")
	}
	if err := r.MarkCoachSeen(ctx, owner); err != nil {
		t.Fatalf("mark: %v", err)
	}
	if err := r.MarkCoachSeen(ctx, owner); err != nil {
		t.Fatalf("mark lần 2 (idempotent): %v", err)
	}
	if seen, _ := r.CoachSeen(ctx, owner); !seen {
		t.Error("sau mark ⇒ CoachSeen=true")
	}
}
```

- [ ] **Step 6: Chạy test → PASS (cần Docker)**

Run: `go test ./internal/scheduling/repo/ -run 'TestRepo_LoadCandidates' -v`
Expected: PASS. Build cả cây: `go build ./...` xanh.

- [ ] **Step 7: Commit**

```bash
git add internal/scheduling/repo/repo.go internal/scheduling/repo/repo_test.go internal/review/service/activity.go db/queries/scheduling/queue.sql
git commit -m "feat(scheduling): pgx repo (index-backed) + review activity adapter (AD-9, AD-10)"
```

---

### Task 11: Handler — queue/learn/coach/prefs endpoints (httptest) — TDD

**Files:**
- Create: `internal/scheduling/handler/handler.go`
- Test: `internal/scheduling/handler/handler_test.go`

- [ ] **Step 1: Viết test thất bại (gin httptest + fakes)**

Create `internal/scheduling/handler/handler_test.go`:
```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/service"
)

type fakeQueue struct{ res domain.QueueResult }

func (f fakeQueue) BuildToday(_ context.Context, _ uuid.UUID, _ time.Time) (domain.QueueResult, error) {
	return f.res, nil
}

type fakeLearn struct{ acked bool }

func (f *fakeLearn) StartSession(_ context.Context, _ uuid.UUID, _ time.Time) (service.LearnSession, error) {
	return service.LearnSession{Cards: []domain.Card{{ID: uuid.New(), State: domain.StateNew}}, ShowCoach: true}, nil
}
func (f *fakeLearn) AckCoach(_ context.Context, _ uuid.UUID) error { f.acked = true; return nil }

type fakePrefsUpd struct{ p domain.SchedulerPrefs }

func (f fakePrefsUpd) UpdateLimits(_ context.Context, _ uuid.UUID, n, r int) (domain.SchedulerPrefs, error) {
	f.p.DailyNewLimit, f.p.DailyReviewLimit = n, r
	return f.p, nil
}

func setup(q QueueBuilder, l LearnProvider, p PrefsUpdater) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { authmw.SetPrincipal(c, authmw.Principal{UserID: uuid.NewString()}); c.Next() }) // fake authmw (Auth Contract)
	RegisterRoutes(r.Group("/api/v1"), q, l, p)
	return r
}

func TestQueueEndpoint(t *testing.T) {
	res := domain.QueueResult{Cards: []domain.Card{{ID: uuid.New(), State: domain.StateReview, DueAt: time.Now()}}, NewCount: 0, ReviewCount: 1}
	r := setup(fakeQueue{res: res}, &fakeLearn{}, fakePrefsUpd{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/queue", nil))
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["review_count"].(float64) != 1 {
		t.Errorf("review_count = %v, want 1", body["review_count"])
	}
}

func TestLearnAndCoachAck(t *testing.T) {
	fl := &fakeLearn{}
	r := setup(fakeQueue{}, fl, fakePrefsUpd{})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/learn", nil))
	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["show_coach"] != true {
		t.Errorf("show_coach = %v, want true", body["show_coach"])
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/api/v1/learn/coach/ack", nil))
	if w2.Code != 204 || !fl.acked {
		t.Errorf("ack status=%d acked=%v", w2.Code, fl.acked)
	}
}

func TestUpdatePrefs_Validation(t *testing.T) {
	r := setup(fakeQueue{}, &fakeLearn{}, fakePrefsUpd{})
	// hợp lệ
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/api/v1/scheduler/prefs",
		strings.NewReader(`{"daily_new_limit":30,"daily_review_limit":150}`)))
	if w.Code != 200 {
		t.Fatalf("valid update status = %d", w.Code)
	}
	// ngoài khoảng 1..9999
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPatch, "/api/v1/scheduler/prefs",
		strings.NewReader(`{"daily_new_limit":0,"daily_review_limit":150}`)))
	if w2.Code != 400 {
		t.Errorf("invalid limit status = %d, want 400", w2.Code)
	}
}
```

- [ ] **Step 2: Chạy test → FAIL**

Run: `go test ./internal/scheduling/handler/ -v`
Expected: FAIL (`RegisterRoutes`/`QueueBuilder`/… undefined).

- [ ] **Step 3: Viết implementation**

Create `internal/scheduling/handler/handler.go`:
```go
package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/scheduling/domain"
	"github.com/memorix/memorix/internal/scheduling/service"
)

type QueueBuilder interface {
	BuildToday(ctx context.Context, userID uuid.UUID, now time.Time) (domain.QueueResult, error)
}

type LearnProvider interface {
	StartSession(ctx context.Context, userID uuid.UUID, now time.Time) (service.LearnSession, error)
	AckCoach(ctx context.Context, userID uuid.UUID) error
}

type PrefsUpdater interface {
	UpdateLimits(ctx context.Context, userID uuid.UUID, newLimit, reviewLimit int) (domain.SchedulerPrefs, error)
}

// RegisterRoutes gắn endpoint queue/learn/coach/prefs vào group /api/v1.
func RegisterRoutes(g *gin.RouterGroup, q QueueBuilder, l LearnProvider, p PrefsUpdater) {
	h := &handlers{q: q, l: l, p: p}
	g.GET("/queue", h.getQueue)
	g.GET("/learn", h.getLearn)
	g.POST("/learn/coach/ack", h.ackCoach)
	g.PATCH("/scheduler/prefs", h.updatePrefs)
}

type handlers struct {
	q QueueBuilder
	l LearnProvider
	p PrefsUpdater
}

type cardDTO struct {
	ID      uuid.UUID       `json:"id"`
	EntryID uuid.UUID       `json:"entry_id"`
	State   domain.CardState `json:"state"`
	DueAt   time.Time       `json:"due_at"`
}

func toDTOs(cards []domain.Card) []cardDTO {
	out := make([]cardDTO, len(cards))
	for i, c := range cards {
		out[i] = cardDTO{ID: c.ID, EntryID: c.EntryID, State: c.State, DueAt: c.DueAt}
	}
	return out
}

func principalID(c *gin.Context) (uuid.UUID, bool) {
	// Auth Contract (Sprint 1): authmw.UserID trả (string, bool), UserID là
	// uuid dạng string → parse ở ranh giới. KHÔNG đọc key thô "user_id".
	uid, ok := authmw.UserID(c)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(uid)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func abort(c *gin.Context, e *httpx.APIError) { c.JSON(e.HTTPStatus(), e) }

func (h *handlers) getQueue(c *gin.Context) {
	uid, ok := principalID(c)
	if !ok {
		abort(c, httpx.NewError(httpx.CodeUnauthenticated, "cần đăng nhập"))
		return
	}
	res, err := h.q.BuildToday(c.Request.Context(), uid, time.Now())
	if err != nil {
		abort(c, httpx.NewError(httpx.CodeInternal, "không dựng được queue"))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"cards":        toDTOs(res.Cards),
		"new_count":    res.NewCount,
		"review_count": res.ReviewCount,
	})
}

func (h *handlers) getLearn(c *gin.Context) {
	uid, ok := principalID(c)
	if !ok {
		abort(c, httpx.NewError(httpx.CodeUnauthenticated, "cần đăng nhập"))
		return
	}
	sess, err := h.l.StartSession(c.Request.Context(), uid, time.Now())
	if err != nil {
		abort(c, httpx.NewError(httpx.CodeInternal, "không mở được phiên học"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"cards": toDTOs(sess.Cards), "show_coach": sess.ShowCoach})
}

func (h *handlers) ackCoach(c *gin.Context) {
	uid, ok := principalID(c)
	if !ok {
		abort(c, httpx.NewError(httpx.CodeUnauthenticated, "cần đăng nhập"))
		return
	}
	if err := h.l.AckCoach(c.Request.Context(), uid); err != nil {
		abort(c, httpx.NewError(httpx.CodeInternal, "không lưu được"))
		return
	}
	c.Status(http.StatusNoContent)
}

type updateLimitsReq struct {
	DailyNewLimit    int `json:"daily_new_limit"`
	DailyReviewLimit int `json:"daily_review_limit"`
}

func (h *handlers) updatePrefs(c *gin.Context) {
	uid, ok := principalID(c)
	if !ok {
		abort(c, httpx.NewError(httpx.CodeUnauthenticated, "cần đăng nhập"))
		return
	}
	var req updateLimitsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		abort(c, httpx.NewError(httpx.CodeValidation, "body không hợp lệ"))
		return
	}
	if e := validateLimit("daily_new_limit", req.DailyNewLimit); e != nil {
		abort(c, e)
		return
	}
	if e := validateLimit("daily_review_limit", req.DailyReviewLimit); e != nil {
		abort(c, e)
		return
	}
	prefs, err := h.p.UpdateLimits(c.Request.Context(), uid, req.DailyNewLimit, req.DailyReviewLimit)
	if err != nil {
		abort(c, httpx.NewError(httpx.CodeInternal, "không cập nhật được"))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"daily_new_limit":    prefs.DailyNewLimit,
		"daily_review_limit": prefs.DailyReviewLimit,
	})
}

func validateLimit(field string, v int) *httpx.APIError {
	if v < 1 || v > 9999 {
		return httpx.NewError(httpx.CodeValidation, "giới hạn phải trong 1..9999").WithField(field, "1..9999")
	}
	return nil
}
```

- [ ] **Step 4: Chạy test → PASS**

Run: `go test ./internal/scheduling/handler/ -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/scheduling/handler/handler.go internal/scheduling/handler/handler_test.go
git commit -m "feat(scheduling): HTTP endpoints queue/learn/coach-ack/prefs (AD-14)"
```

---

### Task 12: Frontend — màn Learn (mini-onboarding + loading skeleton) — TDD

**Files:**
- Create: `web/src/Learn.tsx`
- Test: `web/src/Learn.test.tsx`
- Edit: `web/src/tokens.css` (thêm class `.skeleton`)

- [ ] **Step 1: Viết test thất bại (vitest, mock fetch)**

Create `web/src/Learn.test.tsx`:
```tsx
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import Learn from "./Learn";

function mockFetch(session: unknown) {
  return vi.fn((url: string) => {
    if (url === "/api/v1/learn") {
      return Promise.resolve({ json: () => Promise.resolve(session) });
    }
    return Promise.resolve({ json: () => Promise.resolve({}) });
  });
}

describe("Learn screen", () => {
  beforeEach(() => vi.restoreAllMocks());

  it("shows loading skeleton before data arrives", () => {
    (globalThis as unknown as { fetch: unknown }).fetch = vi.fn(() => new Promise(() => {}));
    render(<Learn />);
    expect(screen.getByTestId("learn-skeleton")).toBeInTheDocument();
  });

  it("shows first-time coach then acks + hides on dismiss", async () => {
    const f = mockFetch({ cards: [{ id: "1", entry_id: "e", state: 0, due_at: "x" }], show_coach: true });
    (globalThis as unknown as { fetch: unknown }).fetch = f;
    render(<Learn />);
    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "Bắt đầu học" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(f).toHaveBeenCalledWith("/api/v1/learn/coach/ack", { method: "POST" });
  });

  it("hides coach when already seen", async () => {
    (globalThis as unknown as { fetch: unknown }).fetch = mockFetch({ cards: [], show_coach: false });
    render(<Learn />);
    await waitFor(() => expect(screen.getByRole("heading", { name: "Học thẻ mới" })).toBeInTheDocument());
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Chạy test → FAIL**

Run: `cd web && npx vitest run src/Learn.test.tsx`
Expected: FAIL (`./Learn` chưa tồn tại).

- [ ] **Step 3: Viết component + skeleton style**

Create `web/src/Learn.tsx`:
```tsx
import { useEffect, useState } from "react";

type Card = { id: string; entry_id: string; state: number; due_at: string };
type Session = { cards: Card[]; show_coach: boolean };

const GRADES = [
  { key: "1", label: "Again", desc: "Quên hẳn — học lại từ đầu" },
  { key: "2", label: "Hard", desc: "Nhớ nhưng khó khăn" },
  { key: "3", label: "Good", desc: "Nhớ được như mong đợi" },
  { key: "4", label: "Easy", desc: "Nhớ rất dễ dàng" },
];

export default function Learn() {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [coach, setCoach] = useState(false);

  useEffect(() => {
    fetch("/api/v1/learn")
      .then((r) => r.json())
      .then((s: Session) => {
        setSession(s);
        setCoach(s.show_coach);
        setLoading(false);
      });
  }, []);

  const dismissCoach = () => {
    setCoach(false);
    fetch("/api/v1/learn/coach/ack", { method: "POST" });
  };

  if (loading) {
    return (
      <div data-testid="learn-skeleton" aria-busy="true">
        <div className="skeleton" style={{ height: 220 }} />
        <div className="skeleton" style={{ height: 48, marginTop: 12 }} />
      </div>
    );
  }

  return (
    <section>
      {coach && (
        <div role="dialog" aria-label="Hướng dẫn chấm thẻ" style={{ background: "var(--surface)", padding: 16, borderRadius: "var(--radius)" }}>
          <h2>Cách chấm thẻ mới</h2>
          <ul>
            {GRADES.map((g) => (
              <li key={g.key}>
                <strong>{g.key} · {g.label}</strong> — {g.desc}
              </li>
            ))}
          </ul>
          <button onClick={dismissCoach} style={{ minHeight: "var(--tap)", color: "var(--accent)" }}>
            Bắt đầu học
          </button>
        </div>
      )}
      <h1>Học thẻ mới</h1>
      <p style={{ color: "var(--muted)" }}>{session?.cards.length ?? 0} thẻ mới hôm nay</p>
    </section>
  );
}
```

Edit `web/src/tokens.css` — thêm cuối file:
```css
.skeleton{background:linear-gradient(90deg,var(--line),var(--surface),var(--line));background-size:200% 100%;border-radius:var(--radius);animation:sk 1.2s ease-in-out infinite}
@keyframes sk{0%{background-position:200% 0}100%{background-position:-200% 0}}
@media (prefers-reduced-motion:reduce){.skeleton{animation:none}}
```

- [ ] **Step 4: Chạy test → PASS**

Run: `cd web && npx vitest run src/Learn.test.tsx`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/Learn.tsx web/src/Learn.test.tsx web/src/tokens.css
git commit -m "feat(web): learn screen with first-time coach onboarding + loading skeleton (FR-29, UX-DR5/13)"
```

---

## Self-Review

**Spec coverage (AC → task):**

| Story / AC | Task |
| --- | --- |
| 4.1 ưu tiên overdue(R thấp)→relearning→review→new | Task 2 (Retrievability) + Task 4 (BuildQueue) |
| 4.1 index cards(owner_id,due_at) | Task 1 (migration) + Task 10 (query dùng index) |
| 4.1 NFR-2 dựng 10k p95<500ms | Task 5 (perf test) |
| 4.2 giới hạn mặc định 20/200, đổi 1..9999 | Task 4 (cap) + Task 11 (PATCH validate 1..9999) + Task 10 (UpdateLimits) |
| 4.2 vượt review-limit → giữ phần dư sang ngày sau | Task 4 (cap review, phần dư vẫn due) + Task 8 (served-today) |
| 4.3 rải new ≤ daily_new_limit/ngày | Task 4 (cap new) + Task 8/9 (trừ NewServed hôm nay) |
| 4.4 anti-flood ~2× review overdue (R thấp), rải ≤7 ngày | Task 6 (PlanAntiFlood) + Task 8 (BulkDefer) + Task 10 |
| 4.4 không dồn toàn bộ overdue 1 ngày | Task 6 (test rải nhiều ngày) |
| 4.5 luồng học thẻ mới riêng | Task 9 (LearnService) + Task 11 (GET /learn) |
| 4.5 mini-onboarding lần đầu (persist seen coach) | Task 1 (study_profiles) + Task 9 (ShowCoach/AckCoach) + Task 10 (upsert) + Task 12 (dialog) |
| 4.5 loading skeleton | Task 12 (skeleton) |
| AD-12 "ngày học" theo TZ user | Task 3 (StartOfStudyDay) + Task 8/9 (đếm theo dayStart) |
| AD-9 cross-module đọc review_logs qua port | Task 7 (port) + Task 10 (ActivityAdapter) |

**Placeholder scan:** Không có TODO/TBD/`...`/stub. Mọi hàm có thân đầy đủ; test có assertion thật. `db/queries/scheduling/queue.sql` là nguồn sqlc (S7) — repo dùng pgx song song, không phải placeholder.

**Type consistency:** `domain.Card/CardState/SchedulerPrefs` (Sprint 3) dùng nguyên vẹn. Mới thêm: `domain.QueueResult`, `domain.DayCounts`, `domain.DeferredCard`, `domain.AntiFloodPlan`, `service.LearnSession`. Port `CardRepo/PrefsRepo/ReviewActivityPort/StudyProfileRepo` — `*repo.Repo` hiện thực CardRepo+PrefsRepo+StudyProfileRepo; `review/service.ActivityAdapter` hiện thực ReviewActivityPort. Handler interface `QueueBuilder/LearnProvider/PrefsUpdater` khớp `*service.QueueService`/`*service.LearnService`/`*repo.Repo`. `httpx.APIError.HTTPStatus()` + `CodeValidation/CodeUnauthenticated/CodeInternal` từ Sprint 0. Frontend `Session{cards,show_coach}` khớp JSON handler (`cards`,`show_coach`,`new_count`,`review_count`).

**Purity (depguard S5):** domain chỉ import `math`, `sort`, `time`, `github.com/google/uuid` — không gin/pgx/net/http. Anti-flood/priority thuần, test không cần DB (Task 2–6,8,9). DB chỉ ở Task 1/10; HTTP chỉ ở Task 11.

**Gaps / giả định (được phép, ghi rõ):**
- `review.review_logs.prev_state` (smallint, 0=New) được giả định có từ Sprint 3 để phân biệt new-served vs review-served. Nếu Sprint 3 lưu khác (vd bảng transitions), chỉ cần chỉnh query trong `ActivityAdapter` (Task 10) — không ảnh hưởng domain/service (dùng fake).
- Wire `*repo.Repo` + `ActivityAdapter` vào `QueueService/LearnService` và mount `RegisterRoutes` sau `authmw` thực hiện ở `cmd/api/main.go` (Wire) — thuộc bước ráp app, ngoài phạm vi TDD từng unit; các interface đã khớp sẵn nên chỉ là dây nối.
- Ngưỡng anti-flood (2×, 7 ngày) là mặc định FR-28/OQ-2 — hằng số `AntiFloodSpreadDays` + biểu thức `2*DailyReviewLimit`, dễ tinh chỉnh bằng dữ liệu beta.

---

## Execution Handoff
Sau khi lưu, chọn cách chạy: subagent-driven (khuyến nghị) hoặc inline executing-plans. Task cần Docker: 1, 10 (testcontainers, bỏ khi `-short`).
