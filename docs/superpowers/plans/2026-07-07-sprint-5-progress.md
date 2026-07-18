# Sprint 5 — Progress & Motivation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dựng module `progress` (read model tiến độ) + trang chủ, North Star, streak thật, forecast, thống kê và trạng thái empty/loading/error toàn app — Epic 5 (Story 5.1–5.6).

**Architecture:** `progress` là **read-model module nhẹ** (service + repo, không full hexagonal — theo `addendum-structure.md`). Ghi read model theo AD-8: subscribe event `CardGraded` in-process **fire-and-forget** cập nhật `progress.daily_stats`/`study_profiles` NGOÀI TX grade; **River reconcile job** rebuild `daily_stats` từ `review_logs` (nguồn chân lý, AD-4). **Số tức thì** (North Star tuần này) đọc **thẳng `review_logs`**, không lấy `daily_stats` đang lag (AD-8). Streak/ngày-học theo TZ user (AD-12). Forecast đếm `cards.due_at` theo ngày.

**Tech Stack:** Go 1.26, Gin v1.10, pgx v5 + sqlc, golang-migrate v4, River (Postgres-backed), slog, Postgres 18, testcontainers-go, testify; React 19 + Vite 7 + TS + Vitest.

**Nguồn:** epics.md (Story 5.1–5.6) · ARCHITECTURE-SPINE.md (AD-4, AD-8, AD-9, AD-12) · addendum-structure.md (progress = read model nhẹ) · prd.md (FR-30..34, North Star = recall đúng **và** interval kế ≥7 ngày, N=7) · 08-database-design.md (schema `daily_stats`/`study_profiles`/`review_logs`/`cards`).

**REUSE từ Sprint 0** (`docs/superpowers/plans/2026-07-07-sprint-0-foundation.md`): `platform/httpx` (envelope lỗi, cursor), `platform/eventbus` (`Event{Name,Payload any}`, `InProcess`, `Subscribe/Publish` async goroutine), `platform/config`, `platform/logger`, `platform/db` (migrate + `*pgxpool.Pool`), River worker skeleton ở `cmd/worker/main.go`, sqlc per-module, testcontainers pattern. **Giả định Sprint 1–4 đã có:** identity (auth; `authmw.RequireAuth` set `Principal` vào gin context — đọc qua `authmw.UserID(c)`, TZ qua `IdentityPort.UserTimezone`, xem Auth Contract), `vocabulary.entries`, `scheduling.cards` (cột: `id,owner_id,entry_id,direction,stability,difficulty,status,reps,lapses,due_at,last_review_at,created_at,updated_at,deleted_at`), `review.review_logs` (cột: `id,card_id,owner_id,grade,retrievability,stability_before/after,difficulty_before/after,elapsed_days,scheduled_days,reviewed_at,client_review_id,duration_ms`, partition tháng, `unique(card_id,client_review_id)`), và review service **phát event `CardGraded`** sau mỗi lần chấm.

**Scope boundary:** CHỈ module `progress` + 3 màn FE (dashboard, stats, app-wide states). KHÔNG sửa scheduling/review domain. Cấu hình lịch (desired-retention slider, daily limit — UX-DR10) thuộc scheduling prefs (Story 3.2/4.2) — ngoài sprint này, màn Stats chỉ **link** tới Settings. TZ per-user ở background/reconcile mặc định UTC qua seam `TZResolver` (prod wire IdentityPort — deferred).

**Grade convention (load-bearing):** `Again=1, Hard=2, Good=3, Easy=4`. **Recall đúng** = `grade ≥ 2` (không phải Again). **Retained (North Star)** = `grade ≥ 2` **VÀ** `scheduled_days ≥ 7`.

---

## Cross-Sprint Auth Contract (canonical — Sprint 1)

Sprint 1 sở hữu `internal/platform/authmw`. Downstream **phải** dùng đúng API này, không tự chế reader/context-key:
- `authmw.RequireAuth(jwtManager) gin.HandlerFunc` — guard route cần đăng nhập.
- `authmw.PrincipalFrom(c) (Principal, bool)` · `Principal{UserID string, Role string, Plan string}`.
- `authmw.UserID(c) (string, bool)` — reader tiện lợi; **UserID là uuid dạng string**.
- **Timezone KHÔNG nằm trong principal/context.** Handler lấy qua `IdentityPort.UserTimezone(ctx, userID)` (thêm vào `Handler` deps) rồi `time.LoadLocation` (AD-9, AD-12). Background/reconcile không có principal → dùng `TZResolver` (mặc định UTC) như đã note.

> Áp dụng: handler `dashboard`/`stats` — thay `c.GetString("user_id")` bằng `authmw.UserID(c)`, và `c.GetString("timezone")` bằng TZ resolve từ IdentityPort (xem patch bên dưới). Fake middleware test set `authmw.Principal{...}`, TZ inject qua resolver test double.

---

### Task 1: Migration — `progress.daily_stats` + `progress.study_profiles`

**Files:**
- Create: `migrations/0008_progress_read_model.up.sql`, `migrations/0008_progress_read_model.down.sql`
- Test: `internal/progress/repo/migrate_test.go`

- [ ] **Step 1: Xác nhận số thứ tự migration kế tiếp**

Run: `ls migrations/`
Expected: file cao nhất hiện tại là `0007_*`. Nếu KHÔNG phải 0007, đổi prefix `0008` thành số kế tiếp liền sau file cao nhất ở mọi file/def dưới đây.

- [ ] **Step 2: Viết migration up**

Create `migrations/0008_progress_read_model.up.sql`:
```sql
-- Read model Progress (AD-8). Không FK chéo schema (AD-10): user_id là ref logic tới identity.users.
CREATE TABLE progress.daily_stats (
    user_id      uuid    NOT NULL,
    day          date    NOT NULL,
    reviews_done integer NOT NULL DEFAULT 0,
    new_done     integer NOT NULL DEFAULT 0,
    retained     integer NOT NULL DEFAULT 0,  -- North Star/ngày: grade>=2 AND scheduled_days>=7
    again        integer NOT NULL DEFAULT 0,
    hard         integer NOT NULL DEFAULT 0,
    good         integer NOT NULL DEFAULT 0,
    easy         integer NOT NULL DEFAULT 0,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, day)
);

CREATE TABLE progress.study_profiles (
    user_id         uuid PRIMARY KEY,
    streak_current  integer NOT NULL DEFAULT 0,
    streak_best     integer NOT NULL DEFAULT 0,
    last_study_date date,                          -- NULL khi chưa có ngày recall thật
    total_reviews   integer NOT NULL DEFAULT 0,
    total_retained  integer NOT NULL DEFAULT 0,    -- tích lũy, KHÔNG reset khi streak reset (FR-32)
    updated_at      timestamptz NOT NULL DEFAULT now()
);
```

Create `migrations/0008_progress_read_model.down.sql`:
```sql
DROP TABLE IF EXISTS progress.study_profiles;
DROP TABLE IF EXISTS progress.daily_stats;
```

- [ ] **Step 3: Viết integration test (testcontainers) — FAIL trước**

Create `internal/progress/repo/migrate_test.go`:
```go
package repo

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/memorix/memorix/internal/platform/db"
)

// startPG dựng Postgres + áp toàn bộ migration; tái dùng ở các test repo khác.
func startPG(t *testing.T) (context.Context, string) {
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
	t.Cleanup(func() { _ = pg.Terminate(ctx) })
	dsn, _ := pg.ConnectionString(ctx, "sslmode=disable")
	if err := db.Migrate("file://../../../migrations", dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return ctx, dsn
}

func TestMigrate_ProgressTables(t *testing.T) {
	ctx, dsn := startPG(t)
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)
	var n int
	err = conn.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables
		 WHERE table_schema='progress' AND table_name IN ('daily_stats','study_profiles')`).Scan(&n)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 progress tables, got %d", n)
	}
}
```

- [ ] **Step 4: Run test — kỳ vọng FAIL**

Run: `go test ./internal/progress/repo/ -run TestMigrate_ProgressTables -v`
Expected: FAIL (migration 0008 chưa tồn tại trước Step 2, hoặc package chưa compile). Sau khi có file up/down: chạy lại → PASS (tìm thấy 2 bảng).

- [ ] **Step 5: Run test — kỳ vọng PASS**

Run: `go test ./internal/progress/repo/ -run TestMigrate_ProgressTables -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add migrations/0008_progress_read_model.up.sql migrations/0008_progress_read_model.down.sql internal/progress/repo/migrate_test.go
git commit -m "feat(progress): read model migration daily_stats + study_profiles"
```

---

### Task 2: Domain — grade rules, `Day`, North Star `CountWordsRetained` (TDD)

**Files:**
- Create: `internal/progress/domain/grade.go`, `internal/progress/domain/day.go`, `internal/progress/domain/northstar.go`
- Test: `internal/progress/domain/northstar_test.go`, `internal/progress/domain/day_test.go`

- [ ] **Step 1: Viết test FAIL**

Create `internal/progress/domain/day_test.go`:
```go
package domain

import (
	"testing"
	"time"
)

func TestDayOf_UsesLocation(t *testing.T) {
	// 2026-07-07T23:30Z là 2026-07-08 06:30 ở Asia/Ho_Chi_Minh (UTC+7).
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	d := DayOf(time.Date(2026, 7, 7, 23, 30, 0, 0, time.UTC), loc)
	if d != (Day{2026, 7, 8}) {
		t.Errorf("DayOf = %v, want 2026-07-08", d)
	}
}

func TestDaysBetween(t *testing.T) {
	a := Day{2026, 7, 7}
	if got := DaysBetween(a, Day{2026, 7, 8}); got != 1 {
		t.Errorf("consecutive = %d, want 1", got)
	}
	if got := DaysBetween(a, Day{2026, 7, 10}); got != 3 {
		t.Errorf("gap = %d, want 3", got)
	}
	if got := DaysBetween(a, a); got != 0 {
		t.Errorf("same = %d, want 0", got)
	}
}

func TestDay_String(t *testing.T) {
	if got := (Day{2026, 7, 8}).String(); got != "2026-07-08" {
		t.Errorf("String = %q", got)
	}
}
```

Create `internal/progress/domain/northstar_test.go`:
```go
package domain

import "testing"

func TestCountWordsRetained(t *testing.T) {
	logs := []RetentionLog{
		{CardID: "a", Grade: 3, ScheduledDays: 10}, // retained
		{CardID: "a", Grade: 4, ScheduledDays: 20}, // same card, không đếm lại
		{CardID: "b", Grade: 2, ScheduledDays: 7},  // retained (biên =7)
		{CardID: "c", Grade: 3, ScheduledDays: 6},  // interval < 7 → không
		{CardID: "d", Grade: 1, ScheduledDays: 30}, // Again → không
	}
	if got := CountWordsRetained(logs); got != 2 {
		t.Errorf("CountWordsRetained = %d, want 2 (a,b distinct)", got)
	}
}

func TestIsRetained_Boundaries(t *testing.T) {
	cases := []struct {
		grade, days int
		want        bool
	}{
		{2, 7, true}, {3, 7, true}, {4, 7, true},
		{2, 6, false}, {1, 30, false}, {3, 100, true},
	}
	for _, c := range cases {
		if got := IsRetained(c.grade, c.days); got != c.want {
			t.Errorf("IsRetained(%d,%d)=%v want %v", c.grade, c.days, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run — FAIL**

Run: `go test ./internal/progress/domain/ -v`
Expected: FAIL (build error — types/functions chưa có).

- [ ] **Step 3: Viết implementation**

Create `internal/progress/domain/grade.go`:
```go
// Package domain là logic thuần của read model Progress — không import Gin/pgx/net-http (AD-2, S5).
package domain

// Mức chấm FSRS.
const (
	GradeAgain = 1
	GradeHard  = 2
	GradeGood  = 3
	GradeEasy  = 4
)

// MinRetainInterval — North Star N=7 ngày (PRD OQ-3).
const MinRetainInterval = 7

// IsRecalled: nhớ được (không Again).
func IsRecalled(grade int) bool { return grade >= GradeHard }

// IsRetained: đủ điều kiện North Star — recall đúng VÀ lịch kế ≥7 ngày.
func IsRetained(grade, scheduledDays int) bool {
	return IsRecalled(grade) && scheduledDays >= MinRetainInterval
}
```

Create `internal/progress/domain/day.go`:
```go
package domain

import (
	"fmt"
	"time"
)

// Day là ngày lịch (civil date) — không giờ, không TZ. Tránh lệch DST khi đếm streak.
type Day struct {
	Year  int
	Month int
	Day   int
}

// DayOf quy đổi thời điểm sang "ngày học" theo TZ user (AD-12).
func DayOf(t time.Time, loc *time.Location) Day {
	t = t.In(loc)
	return Day{t.Year(), int(t.Month()), t.Day()}
}

// At neo Day về nửa đêm UTC (mốc so ngày ổn định, không DST).
func (d Day) At() time.Time {
	return time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC)
}

// DaysBetween = số ngày nguyên từ a tới b (b sau a → dương).
func DaysBetween(a, b Day) int {
	return int(b.At().Sub(a.At()).Hours()) / 24
}

func (d Day) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}
```

Create `internal/progress/domain/northstar.go`:
```go
package domain

// RetentionLog là dòng review_logs tối thiểu để tính North Star.
type RetentionLog struct {
	CardID        string
	Grade         int
	ScheduledDays int
}

// CountWordsRetained = số THẺ (distinct) đạt điều kiện retained trong tập log.
// Đọc thẳng từ review_logs của tuần hiện tại (AD-8) — không dùng daily_stats lag.
func CountWordsRetained(logs []RetentionLog) int {
	seen := make(map[string]struct{})
	for _, l := range logs {
		if IsRetained(l.Grade, l.ScheduledDays) {
			seen[l.CardID] = struct{}{}
		}
	}
	return len(seen)
}
```

- [ ] **Step 4: Run — PASS**

Run: `go test ./internal/progress/domain/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/progress/domain/grade.go internal/progress/domain/day.go internal/progress/domain/northstar.go internal/progress/domain/day_test.go internal/progress/domain/northstar_test.go
git commit -m "feat(progress): domain grade rules, civil Day, North Star retained count"
```

---

### Task 3: Domain — streak transition `ApplyStudyDay` + `RebuildStudyProfile` (TDD)

**Files:**
- Create: `internal/progress/domain/streak.go`
- Test: `internal/progress/domain/streak_test.go`

- [ ] **Step 1: Viết test FAIL**

Create `internal/progress/domain/streak_test.go`:
```go
package domain

import "testing"

func dayPtr(d Day) *Day { return &d }

func TestApplyStudyDay_FirstRetainedStartsStreak(t *testing.T) {
	got := ApplyStudyDay(StudyProfile{}, Day{2026, 7, 7}, 1, 1)
	if got.StreakCurrent != 1 || got.StreakBest != 1 {
		t.Errorf("streak = %d/%d, want 1/1", got.StreakCurrent, got.StreakBest)
	}
	if got.LastStudyDate == nil || *got.LastStudyDate != (Day{2026, 7, 7}) {
		t.Errorf("last = %v", got.LastStudyDate)
	}
	if got.TotalReviews != 1 || got.TotalRetained != 1 {
		t.Errorf("totals = %d/%d, want 1/1", got.TotalReviews, got.TotalRetained)
	}
}

func TestApplyStudyDay_ConsecutiveIncrements(t *testing.T) {
	p := StudyProfile{StreakCurrent: 3, StreakBest: 3, LastStudyDate: dayPtr(Day{2026, 7, 7}), TotalRetained: 10}
	got := ApplyStudyDay(p, Day{2026, 7, 8}, 1, 1)
	if got.StreakCurrent != 4 || got.StreakBest != 4 {
		t.Errorf("streak = %d/%d, want 4/4", got.StreakCurrent, got.StreakBest)
	}
}

func TestApplyStudyDay_MissedDayResets_ButRetainedNotReset(t *testing.T) {
	p := StudyProfile{StreakCurrent: 9, StreakBest: 9, LastStudyDate: dayPtr(Day{2026, 7, 7}), TotalRetained: 100}
	got := ApplyStudyDay(p, Day{2026, 7, 10}, 1, 1) // gap 3 ngày
	if got.StreakCurrent != 1 {
		t.Errorf("streak reset = %d, want 1", got.StreakCurrent)
	}
	if got.StreakBest != 9 {
		t.Errorf("best = %d, want 9 (giữ)", got.StreakBest)
	}
	if got.TotalRetained != 101 {
		t.Errorf("total_retained = %d, want 101 (KHÔNG reset, cộng dồn)", got.TotalRetained)
	}
}

func TestApplyStudyDay_SameDaySecondEvent_NoStreakChange(t *testing.T) {
	p := StudyProfile{StreakCurrent: 2, StreakBest: 2, LastStudyDate: dayPtr(Day{2026, 7, 8}), TotalReviews: 5, TotalRetained: 3}
	got := ApplyStudyDay(p, Day{2026, 7, 8}, 1, 1)
	if got.StreakCurrent != 2 {
		t.Errorf("streak = %d, want 2 (không đổi cùng ngày)", got.StreakCurrent)
	}
	if got.TotalReviews != 6 || got.TotalRetained != 4 {
		t.Errorf("totals = %d/%d, want 6/4", got.TotalReviews, got.TotalRetained)
	}
}

func TestApplyStudyDay_NonRetained_NoStreakNoLastDate(t *testing.T) {
	p := StudyProfile{StreakCurrent: 5, StreakBest: 5, LastStudyDate: dayPtr(Day{2026, 7, 6}), TotalReviews: 20, TotalRetained: 12}
	got := ApplyStudyDay(p, Day{2026, 7, 8}, 1, 0) // ôn nhưng không nhớ được
	if got.StreakCurrent != 5 || *got.LastStudyDate != (Day{2026, 7, 6}) {
		t.Errorf("ngày không recall thật không được đụng streak/last: %+v", got)
	}
	if got.TotalReviews != 21 || got.TotalRetained != 12 {
		t.Errorf("totals = %d/%d, want 21/12", got.TotalReviews, got.TotalRetained)
	}
}

func TestRebuildStudyProfile_FoldsDays(t *testing.T) {
	stats := []DailyStat{
		{Day: Day{2026, 7, 7}, ReviewsDone: 4, Retained: 2},
		{Day: Day{2026, 7, 8}, ReviewsDone: 3, Retained: 1}, // liên tiếp → streak 2
		{Day: Day{2026, 7, 12}, ReviewsDone: 5, Retained: 3}, // gap → reset 1
	}
	got := RebuildStudyProfile(stats)
	if got.StreakCurrent != 1 || got.StreakBest != 2 {
		t.Errorf("streak = %d/%d, want 1/2", got.StreakCurrent, got.StreakBest)
	}
	if got.TotalReviews != 12 || got.TotalRetained != 6 {
		t.Errorf("totals = %d/%d, want 12/6", got.TotalReviews, got.TotalRetained)
	}
}
```

- [ ] **Step 2: Run — FAIL**

Run: `go test ./internal/progress/domain/ -run 'StudyDay|StudyProfile' -v`
Expected: FAIL (`StudyProfile`/`ApplyStudyDay`/`RebuildStudyProfile`/`DailyStat` chưa có).

- [ ] **Step 3: Viết implementation**

Create `internal/progress/domain/streak.go`:
```go
package domain

// StudyProfile — trạng thái động lực tích lũy của một user.
type StudyProfile struct {
	StreakCurrent int
	StreakBest    int
	LastStudyDate *Day // nil = chưa có ngày recall thật
	TotalReviews  int
	TotalRetained int
}

// ApplyStudyDay áp một "sự kiện học" trong ngày `today` vào profile.
//   - totals LUÔN cộng dồn (reviewsDelta/retainedDelta) — total_retained KHÔNG bao giờ reset (FR-32).
//   - streak/last_study_date CHỈ đổi khi có recall thật trong sự kiện này (retainedDelta > 0).
func ApplyStudyDay(p StudyProfile, today Day, reviewsDelta, retainedDelta int) StudyProfile {
	p.TotalReviews += reviewsDelta
	p.TotalRetained += retainedDelta

	if retainedDelta <= 0 {
		return p // ngày không có recall thật → không tính streak
	}

	switch {
	case p.LastStudyDate == nil:
		p.StreakCurrent = 1
	case *p.LastStudyDate == today:
		// đã tính streak cho hôm nay rồi
	case DaysBetween(*p.LastStudyDate, today) == 1:
		p.StreakCurrent++
	default:
		p.StreakCurrent = 1 // lỡ ≥1 ngày → reset streak (nhưng total_retained giữ nguyên)
	}
	if p.StreakCurrent > p.StreakBest {
		p.StreakBest = p.StreakCurrent
	}
	d := today
	p.LastStudyDate = &d
	return p
}

// RebuildStudyProfile dựng lại profile từ chuỗi daily stats (đã sort tăng theo Day).
// Nguồn chân lý = log → daily_stats → fold. Dùng bởi reconcile (AD-4).
func RebuildStudyProfile(stats []DailyStat) StudyProfile {
	var p StudyProfile
	for _, s := range stats {
		p = ApplyStudyDay(p, s.Day, s.ReviewsDone, s.Retained)
	}
	return p
}
```

- [ ] **Step 4: Run — PASS**

Run: `go test ./internal/progress/domain/ -v`
Expected: PASS (bao gồm cả test cũ). Lưu ý: `DailyStat` được định nghĩa ở Task 4; test này build cùng package nên Task 4 phải xong hoặc đặt `DailyStat` tạm — **thực thi Task 4 ngay sau Step 3 nếu build lỗi thiếu `DailyStat`**.

- [ ] **Step 5: Commit**

```bash
git add internal/progress/domain/streak.go internal/progress/domain/streak_test.go
git commit -m "feat(progress): streak transition tied to real recall (reset keeps cumulative retained)"
```

---

### Task 4: Domain — `RebuildDailyStats` aggregation (TDD)

**Files:**
- Create: `internal/progress/domain/rebuild.go`
- Test: `internal/progress/domain/rebuild_test.go`

- [ ] **Step 1: Viết test FAIL**

Create `internal/progress/domain/rebuild_test.go`:
```go
package domain

import (
	"testing"
	"time"
)

func TestRebuildDailyStats_AggregatesByDayAndCountsNew(t *testing.T) {
	loc := time.UTC
	ts := func(d, h int) time.Time { return time.Date(2026, 7, d, h, 0, 0, 0, time.UTC) }
	logs := []LogRow{
		{CardID: "a", Grade: 3, ScheduledDays: 10, ReviewedAt: ts(7, 8)},  // day7: new(a), retained
		{CardID: "b", Grade: 1, ScheduledDays: 0, ReviewedAt: ts(7, 9)},   // day7: new(b), again
		{CardID: "a", Grade: 4, ScheduledDays: 30, ReviewedAt: ts(8, 8)},  // day8: a lại (không new), retained
		{CardID: "b", Grade: 2, ScheduledDays: 6, ReviewedAt: ts(8, 9)},   // day8: b lại, hard, interval<7 → không retained
	}
	got := RebuildDailyStats(logs, loc)
	if len(got) != 2 {
		t.Fatalf("days = %d, want 2", len(got))
	}
	d7 := got[0]
	if d7.Day != (Day{2026, 7, 7}) || d7.ReviewsDone != 2 || d7.NewDone != 2 || d7.Retained != 1 || d7.Good != 1 || d7.Again != 1 {
		t.Errorf("day7 = %+v", d7)
	}
	d8 := got[1]
	if d8.Day != (Day{2026, 7, 8}) || d8.ReviewsDone != 2 || d8.NewDone != 0 || d8.Retained != 1 || d8.Easy != 1 || d8.Hard != 1 {
		t.Errorf("day8 = %+v", d8)
	}
}

func TestRebuildDailyStats_Empty(t *testing.T) {
	if got := RebuildDailyStats(nil, time.UTC); len(got) != 0 {
		t.Errorf("empty → %d rows", len(got))
	}
}
```

- [ ] **Step 2: Run — FAIL**

Run: `go test ./internal/progress/domain/ -run RebuildDailyStats -v`
Expected: FAIL (`LogRow`/`RebuildDailyStats` chưa có).

- [ ] **Step 3: Viết implementation**

Create `internal/progress/domain/rebuild.go`:
```go
package domain

import (
	"sort"
	"time"
)

// LogRow là dòng review_logs tối thiểu để rebuild daily_stats.
type LogRow struct {
	CardID        string
	Grade         int
	ScheduledDays int
	ReviewedAt    time.Time
}

// DailyStat = một hàng progress.daily_stats.
type DailyStat struct {
	Day         Day
	ReviewsDone int
	NewDone     int
	Retained    int
	Again       int
	Hard        int
	Good        int
	Easy        int
}

// RebuildDailyStats gộp toàn bộ log của MỘT user thành daily_stats theo TZ user,
// trả slice sort tăng theo ngày. new_done = số thẻ có lần ôn ĐẦU TIÊN rơi vào ngày đó.
func RebuildDailyStats(logs []LogRow, loc *time.Location) []DailyStat {
	byDay := make(map[Day]*DailyStat)
	firstSeen := make(map[string]bool)

	// Đảm bảo thứ tự thời gian để xác định "lần ôn đầu" đúng.
	ordered := append([]LogRow(nil), logs...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].ReviewedAt.Before(ordered[j].ReviewedAt) })

	for _, l := range ordered {
		d := DayOf(l.ReviewedAt, loc)
		s := byDay[d]
		if s == nil {
			s = &DailyStat{Day: d}
			byDay[d] = s
		}
		s.ReviewsDone++
		if !firstSeen[l.CardID] {
			firstSeen[l.CardID] = true
			s.NewDone++
		}
		switch l.Grade {
		case GradeAgain:
			s.Again++
		case GradeHard:
			s.Hard++
		case GradeGood:
			s.Good++
		case GradeEasy:
			s.Easy++
		}
		if IsRetained(l.Grade, l.ScheduledDays) {
			s.Retained++
		}
	}

	out := make([]DailyStat, 0, len(byDay))
	for _, s := range byDay {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return DaysBetween(out[j].Day, out[i].Day) < 0 })
	return out
}
```

- [ ] **Step 4: Run — PASS**

Run: `go test ./internal/progress/domain/ -v`
Expected: PASS (toàn bộ domain).

- [ ] **Step 5: Commit**

```bash
git add internal/progress/domain/rebuild.go internal/progress/domain/rebuild_test.go
git commit -m "feat(progress): rebuild daily_stats aggregation from review_logs"
```

---

### Task 5: Event contract `shared/events` + ingest service `HandleCardGraded` (TDD)

**Files:**
- Create: `internal/shared/events/events.go`
- Create: `internal/progress/service/ports.go`, `internal/progress/service/ingest.go`
- Test: `internal/progress/service/ingest_test.go`

- [ ] **Step 1: Viết event contract (shared kernel — mọi module dùng, không phụ thuộc ngược)**

Create `internal/shared/events/events.go`:
```go
// Package events là hợp đồng domain-event dùng chung qua eventbus (Payload any).
// Publisher (review) và subscriber (progress) cùng thống nhất struct này.
package events

import "time"

// CardGradedName là tên event khớp eventbus (PascalCase quá khứ — AD conventions).
const CardGradedName = "CardGraded"

// CardGraded phát sau mỗi lần chấm thành công (ngoài TX grade — AD-8).
type CardGraded struct {
	OwnerID       string
	CardID        string
	Grade         int       // 1..4
	ScheduledDays int       // interval kế do FSRS tính
	WasNew        bool      // thẻ chưa từng ôn trước lần chấm này
	ReviewedAt    time.Time // server-ts
}
```

- [ ] **Step 2: Viết test FAIL cho ingest service**

Create `internal/progress/service/ingest_test.go`:
```go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/memorix/memorix/internal/progress/domain"
	"github.com/memorix/memorix/internal/shared/events"
)

// fakeIngestRepo lưu trạng thái trong bộ nhớ để kiểm chứng ghi read model.
type fakeIngestRepo struct {
	bumps    []bumpCall
	profiles map[string]domain.StudyProfile
}
type bumpCall struct {
	owner    string
	day      domain.Day
	wasNew   bool
	grade    int
	retained bool
}

func newFakeIngestRepo() *fakeIngestRepo {
	return &fakeIngestRepo{profiles: map[string]domain.StudyProfile{}}
}
func (f *fakeIngestRepo) BumpDailyStat(_ context.Context, owner string, day domain.Day, wasNew bool, grade int, retained bool) error {
	f.bumps = append(f.bumps, bumpCall{owner, day, wasNew, grade, retained})
	return nil
}
func (f *fakeIngestRepo) GetStudyProfile(_ context.Context, userID string) (domain.StudyProfile, bool, error) {
	p, ok := f.profiles[userID]
	return p, ok, nil
}
func (f *fakeIngestRepo) UpsertStudyProfile(_ context.Context, userID string, p domain.StudyProfile) error {
	f.profiles[userID] = p
	return nil
}

func TestIngest_HandleCardGraded_RetainedUpdatesStatsAndStreak(t *testing.T) {
	repo := newFakeIngestRepo()
	ing := NewIngestor(repo, UTCResolver{}, nil)
	e := events.CardGraded{
		OwnerID: "u1", CardID: "c1", Grade: domain.GradeGood, ScheduledDays: 12,
		WasNew: true, ReviewedAt: time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC),
	}
	if err := ing.HandleCardGraded(context.Background(), e); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(repo.bumps) != 1 {
		t.Fatalf("bumps = %d, want 1", len(repo.bumps))
	}
	b := repo.bumps[0]
	if b.owner != "u1" || b.day != (domain.Day{2026, 7, 8}) || !b.wasNew || !b.retained {
		t.Errorf("bump = %+v", b)
	}
	p := repo.profiles["u1"]
	if p.StreakCurrent != 1 || p.TotalRetained != 1 || p.TotalReviews != 1 {
		t.Errorf("profile = %+v", p)
	}
}

func TestIngest_HandleCardGraded_AgainNoStreak(t *testing.T) {
	repo := newFakeIngestRepo()
	ing := NewIngestor(repo, UTCResolver{}, nil)
	e := events.CardGraded{OwnerID: "u2", CardID: "c9", Grade: domain.GradeAgain, ScheduledDays: 0, ReviewedAt: time.Now()}
	if err := ing.HandleCardGraded(context.Background(), e); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if repo.bumps[0].retained {
		t.Error("Again không được tính retained")
	}
	p := repo.profiles["u2"]
	if p.StreakCurrent != 0 || p.TotalReviews != 1 {
		t.Errorf("profile = %+v, want streak 0 / reviews 1", p)
	}
}
```

- [ ] **Step 3: Run — FAIL**

Run: `go test ./internal/progress/service/ -run Ingest -v`
Expected: FAIL (`NewIngestor`/`UTCResolver` chưa có).

- [ ] **Step 4: Viết ports + ingest implementation**

Create `internal/progress/service/ports.go`:
```go
// Package service điều phối read model Progress (module nhẹ: service + repo).
package service

import (
	"context"
	"time"

	"github.com/memorix/memorix/internal/progress/domain"
)

// IngestRepo — ghi read model từ event (write side).
type IngestRepo interface {
	BumpDailyStat(ctx context.Context, ownerID string, day domain.Day, wasNew bool, grade int, retained bool) error
	GetStudyProfile(ctx context.Context, userID string) (domain.StudyProfile, bool, error)
	UpsertStudyProfile(ctx context.Context, userID string, p domain.StudyProfile) error
}

// TZResolver phân giải TZ user cho "ngày học" (AD-12). MVP background mặc định UTC;
// prod wire IdentityPort (deferred).
type TZResolver interface {
	Location(ctx context.Context, userID string) *time.Location
}

// UTCResolver mặc định UTC.
type UTCResolver struct{}

func (UTCResolver) Location(context.Context, string) *time.Location { return time.UTC }
```

Create `internal/progress/service/ingest.go`:
```go
package service

import (
	"context"
	"log/slog"

	"github.com/memorix/memorix/internal/platform/eventbus"
	"github.com/memorix/memorix/internal/progress/domain"
	"github.com/memorix/memorix/internal/shared/events"
)

// Ingestor cập nhật read model khi có CardGraded (fire-and-forget, ngoài TX grade — AD-8).
type Ingestor struct {
	repo IngestRepo
	tz   TZResolver
	log  *slog.Logger
}

func NewIngestor(repo IngestRepo, tz TZResolver, log *slog.Logger) *Ingestor {
	if log == nil {
		log = slog.Default()
	}
	return &Ingestor{repo: repo, tz: tz, log: log}
}

// HandleCardGraded áp một event vào daily_stats + study_profiles.
func (i *Ingestor) HandleCardGraded(ctx context.Context, e events.CardGraded) error {
	loc := i.tz.Location(ctx, e.OwnerID)
	day := domain.DayOf(e.ReviewedAt, loc)
	retained := domain.IsRetained(e.Grade, e.ScheduledDays)

	if err := i.repo.BumpDailyStat(ctx, e.OwnerID, day, e.WasNew, e.Grade, retained); err != nil {
		return err
	}
	prof, _, err := i.repo.GetStudyProfile(ctx, e.OwnerID)
	if err != nil {
		return err
	}
	retainedDelta := 0
	if retained {
		retainedDelta = 1
	}
	prof = domain.ApplyStudyDay(prof, day, 1, retainedDelta)
	return i.repo.UpsertStudyProfile(ctx, e.OwnerID, prof)
}

// Subscribe gắn Ingestor vào bus. Bus in-process phát async (Sprint 0) → fire-and-forget.
// Lỗi read model KHÔNG làm hỏng grade: chỉ log (AD-8).
func (i *Ingestor) Subscribe(bus eventbus.Bus) {
	bus.Subscribe(events.CardGradedName, func(ctx context.Context, ev eventbus.Event) {
		e, ok := ev.Payload.(events.CardGraded)
		if !ok {
			i.log.Warn("progress: bỏ qua payload CardGraded sai kiểu")
			return
		}
		if err := i.HandleCardGraded(ctx, e); err != nil {
			i.log.Error("progress: cập nhật read model thất bại (sẽ do reconcile chữa)",
				slog.String("owner_id", e.OwnerID), slog.Any("err", err))
		}
	})
}
```

- [ ] **Step 5: Run — PASS**

Run: `go test ./internal/progress/service/ -run Ingest -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/shared/events/events.go internal/progress/service/ports.go internal/progress/service/ingest.go internal/progress/service/ingest_test.go
git commit -m "feat(progress): CardGraded event contract + fire-and-forget read model ingest"
```

---

### Task 6: sqlc queries + repo write side (Bump/Get/Upsert) + integration (TDD)

**Files:**
- Modify: `sqlc.yaml` (add progress block)
- Create: `db/queries/progress/stats.sql`
- Create: `internal/progress/repo/repo.go`
- Test: `internal/progress/repo/write_test.go`

- [ ] **Step 1: Thêm block progress vào sqlc.yaml**

Edit `sqlc.yaml` — thêm block sau block identity (dưới `sql:`):
```yaml
  - engine: postgresql
    schema: migrations
    queries: db/queries/progress
    gen:
      go:
        package: gen
        out: internal/progress/repo/gen
        sql_package: pgx/v5
        overrides:
          - db_type: "uuid"
            go_type: "string"
```
(date → `pgtype.Date`, timestamptz → `time.Time` theo mặc định pgx/v5; repo tự convert.)

- [ ] **Step 2: Viết toàn bộ query progress**

Create `db/queries/progress/stats.sql`:
```sql
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
SELECT card_id, grade, scheduled_days, reviewed_at
FROM review.review_logs
WHERE owner_id = @owner_id
ORDER BY reviewed_at;

-- name: WeekRetentionLogs :many
SELECT card_id, grade, scheduled_days
FROM review.review_logs
WHERE owner_id = @owner_id
  AND reviewed_at >= @from_ts AND reviewed_at < @to_ts;

-- name: DueCount :one
SELECT count(*) FROM scheduling.cards
WHERE owner_id = @owner_id AND deleted_at IS NULL AND status <> 'suspended' AND due_at <= @now;

-- name: ForecastDue :many
SELECT (due_at AT TIME ZONE @tz)::date AS day, count(*) AS due
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
```

- [ ] **Step 3: Generate sqlc**

Run:
```bash
mkdir -p internal/progress/repo/gen
sqlc generate
```
Expected: sinh `internal/progress/repo/gen/{db.go,models.go,stats.sql.go}` không lỗi. (Cần Sprint 3/4 đã có `scheduling.cards` + `review.review_logs` trong `migrations/` để sqlc parse query chéo schema.)

- [ ] **Step 4: Viết repo wrapper (convert gen ↔ domain)**

Create `internal/progress/repo/repo.go`:
```go
// Package repo là adapter Postgres cho read model Progress (sqlc/pgx). Đọc review_logs
// (nguồn chân lý, AD-4/AD-8) và cards.due_at (forecast) — read-only cho read model.
package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/progress/domain"
	"github.com/memorix/memorix/internal/progress/repo/gen"
)

type Repo struct {
	q *gen.Queries
}

func New(pool *pgxpool.Pool) *Repo { return &Repo{q: gen.New(pool)} }

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

func (r *Repo) BumpDailyStat(ctx context.Context, ownerID string, day domain.Day, wasNew bool, grade int, retained bool) error {
	b2i := func(b bool) int32 {
		if b {
			return 1
		}
		return 0
	}
	p := gen.BumpDailyStatParams{
		UserID:   ownerID,
		Day:      toPgDate(day),
		NewDone:  b2i(wasNew),
		Retained: b2i(retained),
		Again:    b2i(grade == domain.GradeAgain),
		Hard:     b2i(grade == domain.GradeHard),
		Good:     b2i(grade == domain.GradeGood),
		Easy:     b2i(grade == domain.GradeEasy),
	}
	return r.q.BumpDailyStat(ctx, p)
}

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

func (r *Repo) DistinctOwners(ctx context.Context) ([]string, error) {
	return r.q.DistinctOwners(ctx)
}

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

var _ = time.Time{} // giữ import time cho read side (Task 7)
```

- [ ] **Step 5: Viết integration test write side — FAIL trước**

Create `internal/progress/repo/write_test.go`:
```go
package repo

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/progress/domain"
)

func openPool(t *testing.T) (*Repo, func()) {
	ctx, dsn := startPG(t)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	return New(pool), func() { pool.Close() }
}

func TestRepo_BumpDailyStat_Accumulates(t *testing.T) {
	r, done := openPool(t)
	defer done()
	ctx := t.Context()
	u := "11111111-1111-1111-1111-111111111111"
	day := domain.Day{2026, 7, 8}
	if err := r.BumpDailyStat(ctx, u, day, true, domain.GradeGood, true); err != nil {
		t.Fatalf("bump1: %v", err)
	}
	if err := r.BumpDailyStat(ctx, u, day, false, domain.GradeAgain, false); err != nil {
		t.Fatalf("bump2: %v", err)
	}
	rev, nw, err := r.TodayStat(ctx, u, day)
	if err != nil {
		t.Fatalf("today: %v", err)
	}
	if rev != 2 || nw != 1 {
		t.Errorf("today = reviews %d / new %d, want 2 / 1", rev, nw)
	}
}

func TestRepo_StudyProfile_RoundTrip(t *testing.T) {
	r, done := openPool(t)
	defer done()
	ctx := t.Context()
	u := "22222222-2222-2222-2222-222222222222"
	if _, found, err := r.GetStudyProfile(ctx, u); err != nil || found {
		t.Fatalf("mới phải không tồn tại: found=%v err=%v", found, err)
	}
	last := domain.Day{2026, 7, 8}
	p := domain.StudyProfile{StreakCurrent: 3, StreakBest: 5, LastStudyDate: &last, TotalReviews: 20, TotalRetained: 12}
	if err := r.UpsertStudyProfile(ctx, u, p); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, found, err := r.GetStudyProfile(ctx, u)
	if err != nil || !found {
		t.Fatalf("get: found=%v err=%v", found, err)
	}
	if got.StreakCurrent != 3 || got.StreakBest != 5 || got.TotalRetained != 12 || got.LastStudyDate == nil || *got.LastStudyDate != last {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}
```
(`TodayStat` được thêm ở Task 7; nếu build lỗi thiếu method, thực thi Task 7 Step 2 trước rồi quay lại.)

- [ ] **Step 6: Run — FAIL rồi PASS**

Run: `go test ./internal/progress/repo/ -run 'BumpDailyStat|StudyProfile_RoundTrip' -v`
Expected: FAIL nếu thiếu `TodayStat` (thêm ở Task 7), sau đó PASS khi write + read đủ.

- [ ] **Step 7: Commit**

```bash
git add sqlc.yaml db/queries/progress internal/progress/repo/gen internal/progress/repo/repo.go internal/progress/repo/write_test.go
git commit -m "feat(progress): sqlc queries + repo write side (daily_stats, study_profiles)"
```

---

### Task 7: Repo read side (WeekRetentionLogs, DueCount, Forecast, Heatmap, Distribution, TodayStat) + integration (TDD)

**Files:**
- Create: `internal/progress/repo/reads.go`
- Test: `internal/progress/repo/read_test.go`

- [ ] **Step 1: Viết read wrappers**

Create `internal/progress/repo/reads.go`:
```go
package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/memorix/memorix/internal/progress/domain"
	"github.com/memorix/memorix/internal/progress/repo/gen"
)

// WeekRetentionLogs đọc THẲNG review_logs cho North Star tức thì (AD-8).
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

func (r *Repo) DueCount(ctx context.Context, ownerID string, now time.Time) (int, error) {
	n, err := r.q.DueCount(ctx, gen.DueCountParams{OwnerID: ownerID, Now: now})
	return int(n), err
}

// Forecast trả map "YYYY-MM-DD" → số thẻ due (theo TZ user), cho 7/30 ngày tới.
func (r *Repo) Forecast(ctx context.Context, ownerID string, from, to time.Time, tz string) (map[string]int, error) {
	rows, err := r.q.ForecastDue(ctx, gen.ForecastDueParams{OwnerID: ownerID, Tz: tz, FromTs: from, ToTs: to})
	if err != nil {
		return nil, err
	}
	m := make(map[string]int, len(rows))
	for _, row := range rows {
		m[fromPgDate(row.Day).String()] = int(row.Due)
	}
	return m, nil
}

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

// Distribution tổng phân bố mức chấm trong khoảng [from,to].
func (r *Repo) Distribution(ctx context.Context, userID string, from, to domain.Day) (again, hard, good, easy int, err error) {
	row, err := r.q.GradeDistribution(ctx, gen.GradeDistributionParams{UserID: userID, FromDay: toPgDate(from), ToDay: toPgDate(to)})
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return int(row.Again), int(row.Hard), int(row.Good), int(row.Easy), nil
}
```

- [ ] **Step 2: Viết integration test read side — FAIL trước**

Create `internal/progress/repo/read_test.go`:
```go
package repo

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/memorix/memorix/internal/progress/domain"
)

// seedCard chèn 1 card tối thiểu (cột theo 08-database-design.md; chỉnh nếu Sprint 3/4 khác).
func seedCard(t *testing.T, ctx context.Context, dsn, id, owner string, due time.Time) {
	t.Helper()
	conn, _ := pgx.Connect(ctx, dsn)
	defer conn.Close(ctx)
	_, err := conn.Exec(ctx, `INSERT INTO scheduling.cards
		(id, owner_id, entry_id, direction, stability, difficulty, status, reps, lapses, due_at, created_at, updated_at)
		VALUES ($1,$2, gen_random_uuid(), 'front_back', 1.0, 5.0, 'review', 1, 0, $3, now(), now())`, id, owner, due)
	if err != nil {
		t.Fatalf("seed card: %v", err)
	}
}

// seedLog chèn 1 review_log tối thiểu.
func seedLog(t *testing.T, ctx context.Context, dsn, card, owner string, grade, sched int, when time.Time) {
	t.Helper()
	conn, _ := pgx.Connect(ctx, dsn)
	defer conn.Close(ctx)
	_, err := conn.Exec(ctx, `INSERT INTO review.review_logs
		(id, card_id, owner_id, grade, retrievability, stability_before, stability_after,
		 difficulty_before, difficulty_after, elapsed_days, scheduled_days, reviewed_at, client_review_id, duration_ms)
		VALUES (gen_random_uuid(), $1,$2,$3, 0.9, 1.0, 2.0, 5.0, 5.0, 1, $4, $5, gen_random_uuid()::text, 1200)`,
		card, owner, grade, sched, when)
	if err != nil {
		t.Fatalf("seed log: %v", err)
	}
}

func TestRepo_WeekRetentionLogs_And_DueForecast(t *testing.T) {
	ctx, dsn := startPG(t)
	pool := mustPool(t, ctx, dsn)
	defer pool.Close()
	r := New(pool)
	owner := "33333333-3333-3333-3333-333333333333"

	// 2 log trong tuần: c1 retained (good/10d), c2 không (hard/3d).
	seedLog(t, ctx, dsn, "aaaaaaaa-0000-0000-0000-000000000001", owner, domain.GradeGood, 10, time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC))
	seedLog(t, ctx, dsn, "aaaaaaaa-0000-0000-0000-000000000002", owner, domain.GradeHard, 3, time.Date(2026, 7, 8, 11, 0, 0, 0, time.UTC))

	logs, err := r.WeekRetentionLogs(ctx, owner, time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("week: %v", err)
	}
	if domain.CountWordsRetained(logs) != 1 {
		t.Errorf("North Star = %d, want 1", domain.CountWordsRetained(logs))
	}

	// Forecast: 1 card due ngày mai, 1 card đã quá hạn (không nằm trong forecast tương lai).
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	seedCard(t, ctx, dsn, "cccccccc-0000-0000-0000-000000000001", owner, now.Add(24*time.Hour))   // mai
	seedCard(t, ctx, dsn, "cccccccc-0000-0000-0000-000000000002", owner, now.Add(-2*time.Hour))   // overdue

	due, err := r.DueCount(ctx, owner, now)
	if err != nil {
		t.Fatalf("due: %v", err)
	}
	if due != 1 {
		t.Errorf("DueCount = %d, want 1 (overdue)", due)
	}
	fc, err := r.Forecast(ctx, owner, now, now.Add(7*24*time.Hour), "UTC")
	if err != nil {
		t.Fatalf("forecast: %v", err)
	}
	if fc["2026-07-09"] != 1 {
		t.Errorf("forecast mai = %d, want 1 (map=%v)", fc["2026-07-09"], fc)
	}
}
```

Add helper `mustPool` to `internal/progress/repo/read_test.go`:
```go
func mustPool(t *testing.T, ctx context.Context, dsn string) *pgxpoolPool {
	t.Helper()
	p, err := pgxpoolNew(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	return p
}
```
Và thêm import + alias ở đầu file `read_test.go`:
```go
import pgxpoolpkg "github.com/jackc/pgx/v5/pgxpool"

type pgxpoolPool = pgxpoolpkg.Pool

func pgxpoolNew(ctx context.Context, dsn string) (*pgxpoolPool, error) { return pgxpoolpkg.New(ctx, dsn) }
```

- [ ] **Step 3: Run — FAIL rồi PASS**

Run: `go test ./internal/progress/repo/ -run 'WeekRetentionLogs_And_DueForecast' -v`
Expected: FAIL trước khi có `reads.go`; PASS sau khi hoàn tất (North Star=1, DueCount=1, forecast mai=1). Nếu seed lỗi cột → chỉnh INSERT theo schema Sprint 3/4 thực tế.

- [ ] **Step 4: Chạy lại write test (giờ `TodayStat` đã có)**

Run: `go test ./internal/progress/repo/ -v`
Expected: PASS toàn bộ repo (migrate + write + read).

- [ ] **Step 5: Commit**

```bash
git add internal/progress/repo/reads.go internal/progress/repo/read_test.go
git commit -m "feat(progress): repo read side — week retention (direct logs), due, forecast, heatmap, distribution"
```

---

### Task 8: Read service (Dashboard/Stats views) + HTTP handler (TDD)

**Files:**
- Create: `internal/progress/service/read.go`
- Create: `internal/progress/handler/handler.go`
- Test: `internal/progress/service/read_test.go`, `internal/progress/handler/handler_test.go`

- [ ] **Step 1: Viết test service FAIL**

Create `internal/progress/service/read_test.go`:
```go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/memorix/memorix/internal/progress/domain"
)

type fakeReadRepo struct {
	weekLogs []domain.RetentionLog
	due      int
	forecast map[string]int
	todayRev int
	todayNew int
	profile  domain.StudyProfile
	heatmap  []domain.DailyStat
	dist     [4]int // again,hard,good,easy
}

func (f *fakeReadRepo) DueCount(context.Context, string, time.Time) (int, error) { return f.due, nil }
func (f *fakeReadRepo) WeekRetentionLogs(context.Context, string, time.Time, time.Time) ([]domain.RetentionLog, error) {
	return f.weekLogs, nil
}
func (f *fakeReadRepo) Forecast(context.Context, string, time.Time, time.Time, string) (map[string]int, error) {
	return f.forecast, nil
}
func (f *fakeReadRepo) TodayStat(context.Context, string, domain.Day) (int, int, error) {
	return f.todayRev, f.todayNew, nil
}
func (f *fakeReadRepo) GetStudyProfile(context.Context, string) (domain.StudyProfile, bool, error) {
	return f.profile, true, nil
}
func (f *fakeReadRepo) Heatmap(context.Context, string, domain.Day, domain.Day) ([]domain.DailyStat, error) {
	return f.heatmap, nil
}
func (f *fakeReadRepo) Distribution(context.Context, string, domain.Day, domain.Day) (int, int, int, int, error) {
	return f.dist[0], f.dist[1], f.dist[2], f.dist[3], nil
}

func TestReader_Dashboard_NorthStarFromLogs(t *testing.T) {
	repo := &fakeReadRepo{
		due:      24,
		todayNew: 5,
		profile:  domain.StudyProfile{StreakCurrent: 3},
		forecast: map[string]int{"2026-07-09": 8},
		weekLogs: []domain.RetentionLog{
			{CardID: "a", Grade: 3, ScheduledDays: 10},
			{CardID: "b", Grade: 4, ScheduledDays: 8},
			{CardID: "c", Grade: 2, ScheduledDays: 3}, // không retained
		},
	}
	rd := NewReader(repo)
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	v, err := rd.Dashboard(context.Background(), "u1", now, time.UTC)
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if v.DueCount != 24 || v.NewToday != 5 || v.StreakCurrent != 3 {
		t.Errorf("dashboard basics = %+v", v)
	}
	if v.NorthStar != 2 { // a,b distinct; đọc thẳng review_logs (AD-8)
		t.Errorf("NorthStar = %d, want 2", v.NorthStar)
	}
	if v.TomorrowForecast != 8 {
		t.Errorf("TomorrowForecast = %d, want 8", v.TomorrowForecast)
	}
}

func TestReader_Stats_RetentionAndDistribution(t *testing.T) {
	repo := &fakeReadRepo{
		todayRev: 20,
		dist:     [4]int{2, 3, 10, 5}, // again2 hard3 good10 easy5 → retention=18/20=0.9
		profile:  domain.StudyProfile{StreakCurrent: 3, StreakBest: 9, TotalRetained: 120},
		forecast: map[string]int{"2026-07-09": 8, "2026-07-20": 4},
	}
	rd := NewReader(repo)
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	v, err := rd.Stats(context.Background(), "u1", now, time.UTC)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if v.ReviewedToday != 20 || v.Distribution.Good != 10 || v.StreakBest != 9 || v.TotalRetained != 120 {
		t.Errorf("stats basics = %+v", v)
	}
	if v.Retention < 0.899 || v.Retention > 0.901 {
		t.Errorf("Retention = %v, want ~0.9", v.Retention)
	}
	if len(v.Forecast) != 30 {
		t.Errorf("forecast length = %d, want 30 (đủ 30 ngày, 0 cho ngày trống)", len(v.Forecast))
	}
}
```

- [ ] **Step 2: Run — FAIL**

Run: `go test ./internal/progress/service/ -run Reader -v`
Expected: FAIL (`NewReader`/views chưa có).

- [ ] **Step 3: Viết read service**

Create `internal/progress/service/read.go`:
```go
package service

import (
	"context"
	"time"

	"github.com/memorix/memorix/internal/progress/domain"
)

// ReadRepo — read side (dashboard/stats).
type ReadRepo interface {
	DueCount(ctx context.Context, ownerID string, now time.Time) (int, error)
	WeekRetentionLogs(ctx context.Context, ownerID string, from, to time.Time) ([]domain.RetentionLog, error)
	Forecast(ctx context.Context, ownerID string, from, to time.Time, tz string) (map[string]int, error)
	TodayStat(ctx context.Context, userID string, day domain.Day) (int, int, error)
	GetStudyProfile(ctx context.Context, userID string) (domain.StudyProfile, bool, error)
	Heatmap(ctx context.Context, userID string, from, to domain.Day) ([]domain.DailyStat, error)
	Distribution(ctx context.Context, userID string, from, to domain.Day) (int, int, int, int, error)
}

// HeatCell là một ô heatmap.
type HeatCell struct {
	Day      string `json:"day"`
	Reviews  int    `json:"reviews"`
	Retained int    `json:"retained"`
}

// ForecastCell là tải dự báo một ngày.
type ForecastCell struct {
	Day string `json:"day"`
	Due int    `json:"due"`
}

// Distribution phân bố mức chấm.
type Distribution struct {
	Again int `json:"again"`
	Hard  int `json:"hard"`
	Good  int `json:"good"`
	Easy  int `json:"easy"`
}

// DashboardView — FR-30/31.
type DashboardView struct {
	DueCount         int        `json:"due_count"`
	NewToday         int        `json:"new_today"`
	StreakCurrent    int        `json:"streak_current"`
	NorthStar        int        `json:"north_star"`
	Heatmap          []HeatCell `json:"heatmap"`
	TomorrowForecast int        `json:"tomorrow_forecast"`
}

// StatsView — FR-33.
type StatsView struct {
	ReviewedToday int            `json:"reviewed_today"`
	Distribution  Distribution   `json:"distribution"`
	Forecast      []ForecastCell `json:"forecast"`
	Heatmap       []HeatCell     `json:"heatmap"`
	StreakCurrent int            `json:"streak_current"`
	StreakBest    int            `json:"streak_best"`
	Retention     float64        `json:"retention"`
	TotalRetained int            `json:"total_retained"`
	NorthStar     int            `json:"north_star"`
}

// Reader dựng view model dashboard/stats.
type Reader struct{ repo ReadRepo }

func NewReader(repo ReadRepo) *Reader { return &Reader{repo: repo} }

func dayStart(now time.Time, loc *time.Location) time.Time {
	n := now.In(loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
}

// weekStart = thứ Hai 00:00 theo TZ user.
func weekStart(now time.Time, loc *time.Location) time.Time {
	d := dayStart(now, loc)
	wd := int(d.Weekday())
	if wd == 0 {
		wd = 7 // Chủ nhật
	}
	return d.AddDate(0, 0, -(wd - 1))
}

func heatCells(stats []domain.DailyStat) []HeatCell {
	out := make([]HeatCell, len(stats))
	for i, s := range stats {
		out[i] = HeatCell{Day: s.Day.String(), Reviews: s.ReviewsDone, Retained: s.Retained}
	}
	return out
}

func (r *Reader) northStar(ctx context.Context, userID string, now time.Time, loc *time.Location) (int, error) {
	logs, err := r.repo.WeekRetentionLogs(ctx, userID, weekStart(now, loc), now)
	if err != nil {
		return 0, err
	}
	return domain.CountWordsRetained(logs), nil // đọc thẳng review_logs (AD-8)
}

func (r *Reader) Dashboard(ctx context.Context, userID string, now time.Time, loc *time.Location) (DashboardView, error) {
	var v DashboardView
	var err error
	if v.DueCount, err = r.repo.DueCount(ctx, userID, now); err != nil {
		return v, err
	}
	today := domain.DayOf(now, loc)
	if _, v.NewToday, err = r.repo.TodayStat(ctx, userID, today); err != nil {
		return v, err
	}
	prof, _, err := r.repo.GetStudyProfile(ctx, userID)
	if err != nil {
		return v, err
	}
	v.StreakCurrent = prof.StreakCurrent
	if v.NorthStar, err = r.northStar(ctx, userID, now, loc); err != nil {
		return v, err
	}
	from28 := domain.DayOf(dayStart(now, loc).AddDate(0, 0, -27), loc)
	hm, err := r.repo.Heatmap(ctx, userID, from28, today)
	if err != nil {
		return v, err
	}
	v.Heatmap = heatCells(hm)

	fcTo := dayStart(now, loc).AddDate(0, 0, 8)
	fc, err := r.repo.Forecast(ctx, userID, dayStart(now, loc), fcTo, loc.String())
	if err != nil {
		return v, err
	}
	tomorrow := domain.DayOf(dayStart(now, loc).AddDate(0, 0, 1), loc)
	v.TomorrowForecast = fc[tomorrow.String()]
	return v, nil
}

func (r *Reader) Stats(ctx context.Context, userID string, now time.Time, loc *time.Location) (StatsView, error) {
	var v StatsView
	today := domain.DayOf(now, loc)
	rev, _, err := r.repo.TodayStat(ctx, userID, today)
	if err != nil {
		return v, err
	}
	v.ReviewedToday = rev

	from90 := domain.DayOf(dayStart(now, loc).AddDate(0, 0, -89), loc)
	a, h, g, e, err := r.repo.Distribution(ctx, userID, from90, today)
	if err != nil {
		return v, err
	}
	v.Distribution = Distribution{Again: a, Hard: h, Good: g, Easy: e}
	total := a + h + g + e
	if total > 0 {
		v.Retention = float64(h+g+e) / float64(total)
	}

	prof, _, err := r.repo.GetStudyProfile(ctx, userID)
	if err != nil {
		return v, err
	}
	v.StreakCurrent, v.StreakBest, v.TotalRetained = prof.StreakCurrent, prof.StreakBest, prof.TotalRetained
	if v.NorthStar, err = r.northStar(ctx, userID, now, loc); err != nil {
		return v, err
	}

	hm, err := r.repo.Heatmap(ctx, userID, from90, today)
	if err != nil {
		return v, err
	}
	v.Heatmap = heatCells(hm)

	// Forecast 30 ngày tới — luôn đủ 30 ô (0 cho ngày trống) để FE render nhất quán.
	start := dayStart(now, loc)
	fc, err := r.repo.Forecast(ctx, userID, start, start.AddDate(0, 0, 30), loc.String())
	if err != nil {
		return v, err
	}
	v.Forecast = make([]ForecastCell, 30)
	for i := 0; i < 30; i++ {
		d := domain.DayOf(start.AddDate(0, 0, i), loc)
		v.Forecast[i] = ForecastCell{Day: d.String(), Due: fc[d.String()]}
	}
	return v, nil
}
```

- [ ] **Step 4: Run service test — PASS**

Run: `go test ./internal/progress/service/ -v`
Expected: PASS (ingest + reader).

- [ ] **Step 5: Viết handler + test FAIL**

Create `internal/progress/handler/handler_test.go`:
```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/progress/service"
)

// fakeTZ là TZResolver test double (thay cho IdentityPort ở prod).
type fakeTZ struct{ loc *time.Location }

func (f fakeTZ) Location(context.Context, string) *time.Location { return f.loc }

func mustLoc(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

type fakeReader struct{}

func (fakeReader) Dashboard(_ context.Context, userID string, _ time.Time, _ *time.Location) (service.DashboardView, error) {
	return service.DashboardView{DueCount: 24, NewToday: 5, StreakCurrent: 3, NorthStar: 12, TomorrowForecast: 8}, nil
}
func (fakeReader) Stats(context.Context, string, time.Time, *time.Location) (service.StatsView, error) {
	return service.StatsView{ReviewedToday: 20, Retention: 0.9}, nil
}

func setup() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// stub authmw: set principal đúng API Sprint 1 (không dùng key thô).
	r.Use(func(c *gin.Context) { authmw.SetPrincipal(c, authmw.Principal{UserID: "u1"}) })
	h := New(fakeReader{}, fakeTZ{loc: mustLoc("Asia/Ho_Chi_Minh")})
	h.Register(r.Group("/api/v1"))
	return r
}

func TestHandler_Dashboard(t *testing.T) {
	r := setup()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/progress/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	var body struct {
		Data service.DashboardView `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body: %v", err)
	}
	if body.Data.DueCount != 24 || body.Data.NorthStar != 12 {
		t.Errorf("data = %+v", body.Data)
	}
}

func TestHandler_Stats(t *testing.T) {
	r := setup()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/progress/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
}
```

Create `internal/progress/handler/handler.go`:
```go
// Package handler là adapter Gin cho read model Progress (bind → service → envelope).
package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/progress/service"
)

// Reader là cổng service mà handler cần (định nghĩa ở phía gọi — AD-1).
type Reader interface {
	Dashboard(ctx context.Context, userID string, now time.Time, loc *time.Location) (service.DashboardView, error)
	Stats(ctx context.Context, userID string, now time.Time, loc *time.Location) (service.StatsView, error)
}

type Handler struct {
	svc Reader
	tz  TZResolver // request-path: wrap IdentityPort.UserTimezone (AD-9); test dùng double
	now func() time.Time
}

func New(svc Reader, tz TZResolver) *Handler {
	return &Handler{svc: svc, tz: tz, now: time.Now}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	pg := g.Group("/progress")
	pg.GET("/dashboard", h.dashboard)
	pg.GET("/stats", h.stats)
}

// userLoc phân giải TZ user qua TZResolver (backed by IdentityPort ở prod),
// KHÔNG đọc từ gin context (TZ không nằm trong principal — Auth Contract).
func (h *Handler) userLoc(ctx context.Context, uid string) *time.Location {
	return h.tz.Location(ctx, uid)
}

func loadLoc(tz string) *time.Location {
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc
	}
	return time.UTC
}

func (h *Handler) dashboard(c *gin.Context) {
	uid, _ := authmw.UserID(c)                    // canonical reader (Sprint 1)
	loc := h.userLoc(c.Request.Context(), uid)    // TZ qua IdentityPort, KHÔNG từ context
	v, err := h.svc.Dashboard(c.Request.Context(), uid, h.now(), loc)
	if err != nil {
		e := httpx.NewError(httpx.CodeInternal, "không tải được trang chủ")
		c.JSON(e.HTTPStatus(), e)
		return
	}
	c.JSON(200, gin.H{"data": v})
}

func (h *Handler) stats(c *gin.Context) {
	uid, _ := authmw.UserID(c)                    // canonical reader (Sprint 1)
	loc := h.userLoc(c.Request.Context(), uid)    // TZ qua IdentityPort, KHÔNG từ context
	v, err := h.svc.Stats(c.Request.Context(), uid, h.now(), loc)
	if err != nil {
		e := httpx.NewError(httpx.CodeInternal, "không tải được thống kê")
		c.JSON(e.HTTPStatus(), e)
		return
	}
	c.JSON(200, gin.H{"data": v})
}
```

- [ ] **Step 6: Run handler test — PASS**

Run: `go test ./internal/progress/handler/ -v`
Expected: PASS (2 tests).

- [ ] **Step 7: Commit**

```bash
git add internal/progress/service/read.go internal/progress/service/read_test.go internal/progress/handler/
git commit -m "feat(progress): dashboard+stats read service and HTTP handlers (North Star from logs)"
```

---

### Task 9: River reconcile job — rebuild daily_stats từ review_logs (TDD + wire vào cmd/worker)

**Files:**
- Create: `internal/progress/service/reconcile.go`
- Test: `internal/progress/service/reconcile_test.go`
- Create: `internal/progress/worker/reconcile_worker.go`
- Modify: `cmd/worker/main.go`

- [ ] **Step 1: Viết test reconcile FAIL**

Create `internal/progress/service/reconcile_test.go`:
```go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/memorix/memorix/internal/progress/domain"
)

type fakeReconcileRepo struct {
	owners   []string
	logs     map[string][]domain.LogRow
	replaced map[string][]domain.DailyStat
	profiles map[string]domain.StudyProfile
}

func (f *fakeReconcileRepo) DistinctOwners(context.Context) ([]string, error) { return f.owners, nil }
func (f *fakeReconcileRepo) AllLogsForOwner(_ context.Context, o string) ([]domain.LogRow, error) {
	return f.logs[o], nil
}
func (f *fakeReconcileRepo) ReplaceDailyStats(_ context.Context, o string, s []domain.DailyStat) error {
	f.replaced[o] = s
	return nil
}
func (f *fakeReconcileRepo) UpsertStudyProfile(_ context.Context, o string, p domain.StudyProfile) error {
	f.profiles[o] = p
	return nil
}

func TestReconcile_RebuildsDailyStatsAndProfile(t *testing.T) {
	ts := func(d int) time.Time { return time.Date(2026, 7, d, 10, 0, 0, 0, time.UTC) }
	repo := &fakeReconcileRepo{
		owners:   []string{"u1"},
		logs:     map[string][]domain.LogRow{"u1": {
			{CardID: "a", Grade: domain.GradeGood, ScheduledDays: 10, ReviewedAt: ts(7)},
			{CardID: "a", Grade: domain.GradeEasy, ScheduledDays: 20, ReviewedAt: ts(8)},
		}},
		replaced: map[string][]domain.DailyStat{},
		profiles: map[string]domain.StudyProfile{},
	}
	rc := NewReconciler(repo, UTCResolver{})
	if err := rc.ReconcileAll(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(repo.replaced["u1"]) != 2 {
		t.Fatalf("daily_stats rebuilt = %d days, want 2", len(repo.replaced["u1"]))
	}
	p := repo.profiles["u1"]
	if p.StreakCurrent != 2 || p.TotalRetained != 2 {
		t.Errorf("profile = %+v, want streak 2 / retained 2", p)
	}
}
```

- [ ] **Step 2: Run — FAIL**

Run: `go test ./internal/progress/service/ -run Reconcile -v`
Expected: FAIL (`NewReconciler` chưa có).

- [ ] **Step 3: Viết reconcile service**

Create `internal/progress/service/reconcile.go`:
```go
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
```

- [ ] **Step 4: Run — PASS**

Run: `go test ./internal/progress/service/ -run Reconcile -v`
Expected: PASS.

- [ ] **Step 5: Viết River worker adapter**

Create `internal/progress/worker/reconcile_worker.go`:
```go
// Package worker gắn reconcile vào River (job runner Postgres-backed, ARCH-12).
package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/memorix/memorix/internal/progress/service"
)

// ReconcileArgs là job rebuild daily_stats định kỳ.
type ReconcileArgs struct{}

func (ReconcileArgs) Kind() string { return "progress_reconcile" }

// ReconcileWorker chạy Reconciler.ReconcileAll.
type ReconcileWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	Reconciler *service.Reconciler
}

func (w *ReconcileWorker) Work(ctx context.Context, _ *river.Job[ReconcileArgs]) error {
	return w.Reconciler.ReconcileAll(ctx)
}

// PeriodicSpec trả periodic job chạy mỗi giờ (chữa drift AD-8).
func PeriodicSpec() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(river.PeriodicIntervalHourly()),
		func() (river.JobArgs, *river.InsertOpts) { return ReconcileArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}
```
> Lưu ý API River: nếu phiên bản River trong `go.mod` khác chữ ký `NewPeriodicJob`/`PeriodicInterval`, chỉnh theo godoc phiên bản đó (giữ nguyên: worker `progress_reconcile`, chạy hàng giờ, RunOnStart). Nếu không có `PeriodicIntervalHourly()`, dùng `time.Hour`.

- [ ] **Step 6: Đăng ký vào cmd/worker/main.go (thay skeleton Sprint 0)**

Replace nội dung `cmd/worker/main.go` (giữ config/logger như Sprint 0, thêm River client + đăng ký reconcile):
```go
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/memorix/memorix/internal/platform/config"
	"github.com/memorix/memorix/internal/platform/logger"
	prepo "github.com/memorix/memorix/internal/progress/repo"
	psvc "github.com/memorix/memorix/internal/progress/service"
	pworker "github.com/memorix/memorix/internal/progress/worker"
)

func main() {
	log := logger.New(os.Stdout, "info")
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("config load failed", slog.Any("err", err))
		os.Exit(1)
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db pool failed", slog.Any("err", err))
		os.Exit(1)
	}
	defer pool.Close()

	repo := prepo.New(pool)
	reconciler := psvc.NewReconciler(repo, psvc.UTCResolver{}) // TZ per-user: deferred (IdentityPort)

	workers := river.NewWorkers()
	river.AddWorker(workers, &pworker.ReconcileWorker{Reconciler: reconciler})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 5}},
		Workers: workers,
		PeriodicJobs: []*river.PeriodicJob{
			pworker.PeriodicSpec(),
		},
	})
	if err != nil {
		log.Error("river client failed", slog.Any("err", err))
		os.Exit(1)
	}

	log.Info("worker starting", "env", cfg.AppEnv, "jobs", "progress_reconcile(hourly)")
	if err := client.Start(ctx); err != nil {
		log.Error("river start failed", slog.Any("err", err))
		os.Exit(1)
	}
	select {} // giữ tiến trình sống
}
```
> Cần River migration (bảng `river_job`) đã áp vào DB. Nếu Sprint 0/4 chưa chạy `river migrate`, thêm bước `go run github.com/riverqueue/river/cmd/river migrate-up --database-url "$DATABASE_URL"` vào quy trình khởi tạo DB (ngoài golang-migrate).

- [ ] **Step 7: Verify build**

Run: `go build ./cmd/worker ./internal/progress/...`
Expected: no error.

- [ ] **Step 8: Commit**

```bash
git add internal/progress/service/reconcile.go internal/progress/service/reconcile_test.go internal/progress/worker/ cmd/worker/main.go
git commit -m "feat(progress): River reconcile job rebuilds daily_stats from review_logs"
```

---

### Task 10: Wire progress vào cmd/api (subscribe CardGraded + đăng ký route)

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Thêm khởi tạo progress vào cmd/api/main.go**

Trong `cmd/api/main.go`, SAU khi đã có `bus` (eventbus.InProcess), `pool` (*pgxpool.Pool), và router group `/api/v1` được bảo vệ authmw, thêm:
```go
	// --- progress (read model) ---
	progressRepo := progressrepo.New(pool)
	ingestor := progresssvc.NewIngestor(progressRepo, progresssvc.UTCResolver{}, log)
	ingestor.Subscribe(bus) // fire-and-forget cập nhật daily_stats khi CardGraded (AD-8)

	reader := progresssvc.NewReader(progressRepo)
	// TZResolver: MVP dùng UTCResolver (TZ per-user deferred); prod wire adapter
	// bọc identity.IdentityPort.UserTimezone (AD-9). Không đọc TZ từ gin context.
	progresshandler.New(reader, progresssvc.UTCResolver{}).Register(v1auth) // v1auth đã qua authmw.RequireAuth
```

Thêm import:
```go
	progresshandler "github.com/memorix/memorix/internal/progress/handler"
	progressrepo "github.com/memorix/memorix/internal/progress/repo"
	progresssvc "github.com/memorix/memorix/internal/progress/service"
```
> `v1auth` là tên nhóm route đã gắn authmw ở Sprint 1 (đặt `user_id`/`timezone` vào context). Nếu tên khác, thay cho khớp. `bus` và `pool` là biến đã tạo ở phần khởi tạo hiện có; nếu `cmd/api` chưa tạo `pool`, thêm `pool, _ := pgxpool.New(ctx, cfg.DatabaseURL)` như cmd/worker.

- [ ] **Step 2: Verify build + smoke**

Run:
```bash
go build ./...
go vet ./internal/progress/...
```
Expected: no error.

- [ ] **Step 3: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat(api): wire progress read model — subscribe CardGraded + dashboard/stats routes"
```

---

### Task 11: Frontend — màn Dashboard (TDD Vitest)

**Files:**
- Create: `web/src/lib/api.ts`
- Create: `web/src/screens/Dashboard.tsx`
- Test: `web/src/screens/Dashboard.test.tsx`

- [ ] **Step 1: API client + kiểu dữ liệu**

Create `web/src/lib/api.ts`:
```ts
export type HeatCell = { day: string; reviews: number; retained: number };
export type ForecastCell = { day: string; due: number };

export type Dashboard = {
  due_count: number;
  new_today: number;
  streak_current: number;
  north_star: number;
  heatmap: HeatCell[];
  tomorrow_forecast: number;
};

export type Stats = {
  reviewed_today: number;
  distribution: { again: number; hard: number; good: number; easy: number };
  forecast: ForecastCell[];
  heatmap: HeatCell[];
  streak_current: number;
  streak_best: number;
  retention: number;
  total_retained: number;
  north_star: number;
};

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(`/api/v1${path}`, { credentials: "include" });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const body = await res.json();
  return body.data as T;
}

export const fetchDashboard = () => getJSON<Dashboard>("/progress/dashboard");
export const fetchStats = () => getJSON<Stats>("/progress/stats");
```

- [ ] **Step 2: Viết test FAIL**

Create `web/src/screens/Dashboard.test.tsx`:
```tsx
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, afterEach } from "vitest";
import Dashboard from "./Dashboard";

function mockFetch(data: unknown, ok = true) {
  vi.stubGlobal("fetch", vi.fn(async () => ({ ok, status: ok ? 200 : 500, json: async () => ({ data }) })));
}

afterEach(() => vi.unstubAllGlobals());

const sample = {
  due_count: 24, new_today: 5, streak_current: 3, north_star: 12,
  heatmap: [{ day: "2026-07-08", reviews: 10, retained: 6 }],
  tomorrow_forecast: 8,
};

describe("Dashboard", () => {
  it("hiển thị skeleton khi đang tải", () => {
    mockFetch(sample);
    render(<Dashboard />);
    expect(screen.getByTestId("dashboard-skeleton")).toBeInTheDocument();
  });

  it("hiển thị due count, North Star, streak, CTA Ôn ngay", async () => {
    mockFetch(sample);
    render(<Dashboard />);
    await waitFor(() => expect(screen.getByText("24")).toBeInTheDocument());
    expect(screen.getByText(/\+12 từ nhớ được/)).toBeInTheDocument();
    expect(screen.getByText(/3/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Ôn ngay/ })).toBeInTheDocument();
  });

  it("không có thẻ due → ẩn CTA, hiện trạng thái đã xong", async () => {
    mockFetch({ ...sample, due_count: 0 });
    render(<Dashboard />);
    await waitFor(() => expect(screen.getByText(/Bạn đã ôn hết/)).toBeInTheDocument());
    expect(screen.queryByRole("button", { name: /Ôn ngay/ })).not.toBeInTheDocument();
  });

  it("lỗi tải → hiện error state với nút thử lại", async () => {
    mockFetch(null, false);
    render(<Dashboard />);
    await waitFor(() => expect(screen.getByText(/Không tải được/)).toBeInTheDocument());
    expect(screen.getByRole("button", { name: /Thử lại/ })).toBeInTheDocument();
  });
});
```

- [ ] **Step 3: Run — FAIL**

Run: `cd web && npx vitest run src/screens/Dashboard.test.tsx`
Expected: FAIL (`Dashboard` chưa có).

- [ ] **Step 4: Viết Dashboard**

Create `web/src/screens/Dashboard.tsx`:
```tsx
import { useEffect, useState, useCallback } from "react";
import { fetchDashboard, type Dashboard as DashboardData } from "../lib/api";

type State =
  | { kind: "loading" }
  | { kind: "error" }
  | { kind: "ready"; data: DashboardData };

function MiniHeatmap({ cells }: { cells: DashboardData["heatmap"] }) {
  return (
    <div aria-label="Lịch học 28 ngày" style={{ display: "flex", gap: 3, flexWrap: "wrap" }}>
      {cells.map((c) => {
        const on = c.reviews > 0;
        return (
          <span
            key={c.day}
            title={`${c.day}: ${c.reviews} ôn / ${c.retained} nhớ`}
            style={{
              width: 12, height: 12, borderRadius: 3,
              background: on ? "var(--good)" : "var(--line)",
            }}
          />
        );
      })}
    </div>
  );
}

export default function Dashboard() {
  const [state, setState] = useState<State>({ kind: "loading" });

  const load = useCallback(() => {
    setState({ kind: "loading" });
    fetchDashboard()
      .then((data) => setState({ kind: "ready", data }))
      .catch(() => setState({ kind: "error" }));
  }, []);

  useEffect(() => { load(); }, [load]);

  if (state.kind === "loading") {
    return <div data-testid="dashboard-skeleton" aria-busy="true" style={{ padding: 16 }}>
      <div style={{ height: 80, background: "var(--line)", borderRadius: 14, marginBottom: 12 }} />
      <div style={{ height: 120, background: "var(--line)", borderRadius: 14 }} />
    </div>;
  }
  if (state.kind === "error") {
    return <div style={{ padding: 16 }}>
      <p>Không tải được trang chủ.</p>
      <button onClick={load} style={{ minHeight: "var(--tap)" }}>Thử lại</button>
    </div>;
  }

  const d = state.data;
  return (
    <div style={{ padding: 16, display: "grid", gap: 16 }}>
      <section style={{ background: "var(--surface)", borderRadius: 14, padding: 16 }}>
        {d.due_count > 0 ? (
          <>
            <div style={{ fontSize: 40, fontWeight: 700 }}>{d.due_count}</div>
            <div style={{ color: "var(--muted)" }}>thẻ đến hạn · {d.new_today} thẻ mới hôm nay</div>
            <button style={{ marginTop: 12, minHeight: "var(--tap)", background: "var(--accent)", color: "#fff", border: "none", borderRadius: 12, padding: "0 20px" }}>
              Ôn ngay
            </button>
          </>
        ) : (
          <p>🎉 Bạn đã ôn hết! Quay lại sau nhé.</p>
        )}
      </section>

      <section style={{ background: "var(--surface)", borderRadius: 14, padding: 16 }}>
        <div style={{ fontSize: 24, fontWeight: 700, color: "var(--good)" }}>+{d.north_star} từ nhớ được</div>
        <div style={{ color: "var(--muted)" }}>tuần này · 🔥 streak {d.streak_current} ngày</div>
      </section>

      <section style={{ background: "var(--surface)", borderRadius: 14, padding: 16 }}>
        <MiniHeatmap cells={d.heatmap} />
        <div style={{ marginTop: 8, color: "var(--muted)" }}>Ngày mai: {d.tomorrow_forecast} thẻ</div>
      </section>
    </div>
  );
}
```

- [ ] **Step 5: Run — PASS**

Run: `cd web && npx vitest run src/screens/Dashboard.test.tsx`
Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
cd .. && git add web/src/lib/api.ts web/src/screens/Dashboard.tsx web/src/screens/Dashboard.test.tsx
git commit -m "feat(web): dashboard screen — due, North Star, streak, mini heatmap, forecast"
```

---

### Task 12: Frontend — màn Statistics (TDD Vitest)

**Files:**
- Create: `web/src/screens/Stats.tsx`
- Test: `web/src/screens/Stats.test.tsx`

- [ ] **Step 1: Viết test FAIL**

Create `web/src/screens/Stats.test.tsx`:
```tsx
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, afterEach } from "vitest";
import Stats from "./Stats";

function mockFetch(data: unknown, ok = true) {
  vi.stubGlobal("fetch", vi.fn(async () => ({ ok, status: ok ? 200 : 500, json: async () => ({ data }) })));
}
afterEach(() => vi.unstubAllGlobals());

const sample = {
  reviewed_today: 20,
  distribution: { again: 2, hard: 3, good: 10, easy: 5 },
  forecast: [{ day: "2026-07-19", due: 8 }, { day: "2026-07-20", due: 4 }],
  heatmap: [{ day: "2026-07-08", reviews: 10, retained: 6 }],
  streak_current: 3, streak_best: 9, retention: 0.9, total_retained: 120, north_star: 12,
};

describe("Stats", () => {
  it("skeleton khi tải", () => {
    mockFetch(sample);
    render(<Stats />);
    expect(screen.getByTestId("stats-skeleton")).toBeInTheDocument();
  });

  it("hiển thị retention, distribution, forecast, streak best", async () => {
    mockFetch(sample);
    render(<Stats />);
    await waitFor(() => expect(screen.getByText(/90%/)).toBeInTheDocument()); // retention
    expect(screen.getByText(/Đã ôn hôm nay: 20/)).toBeInTheDocument();
    expect(screen.getByLabelText("Phân bố mức chấm")).toBeInTheDocument();
    expect(screen.getByLabelText(/Dự báo tải/)).toBeInTheDocument();
    expect(screen.getByText(/Kỷ lục 9/)).toBeInTheDocument();
  });

  it("lỗi → error state", async () => {
    mockFetch(null, false);
    render(<Stats />);
    await waitFor(() => expect(screen.getByText(/Không tải được/)).toBeInTheDocument());
  });
});
```

- [ ] **Step 2: Run — FAIL**

Run: `cd web && npx vitest run src/screens/Stats.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Viết Stats**

Create `web/src/screens/Stats.tsx`:
```tsx
import { useEffect, useState, useCallback } from "react";
import { fetchStats, type Stats as StatsData } from "../lib/api";

type State =
  | { kind: "loading" }
  | { kind: "error" }
  | { kind: "ready"; data: StatsData };

const GRADE_COLORS: Record<string, string> = {
  again: "var(--again)", hard: "var(--hard)", good: "var(--good)", easy: "var(--easy)",
};

export default function Stats() {
  const [state, setState] = useState<State>({ kind: "loading" });

  const load = useCallback(() => {
    setState({ kind: "loading" });
    fetchStats()
      .then((data) => setState({ kind: "ready", data }))
      .catch(() => setState({ kind: "error" }));
  }, []);

  useEffect(() => { load(); }, [load]);

  if (state.kind === "loading") {
    return <div data-testid="stats-skeleton" aria-busy="true" style={{ padding: 16 }}>
      <div style={{ height: 100, background: "var(--line)", borderRadius: 14, marginBottom: 12 }} />
      <div style={{ height: 160, background: "var(--line)", borderRadius: 14 }} />
    </div>;
  }
  if (state.kind === "error") {
    return <div style={{ padding: 16 }}>
      <p>Không tải được thống kê.</p>
      <button onClick={load} style={{ minHeight: "var(--tap)" }}>Thử lại</button>
    </div>;
  }

  const s = state.data;
  const distEntries = Object.entries(s.distribution) as [keyof StatsData["distribution"], number][];
  const distTotal = distEntries.reduce((a, [, v]) => a + v, 0) || 1;
  const maxDue = Math.max(1, ...s.forecast.map((f) => f.due));

  return (
    <div style={{ padding: 16, display: "grid", gap: 16 }}>
      <section style={{ background: "var(--surface)", borderRadius: 14, padding: 16 }}>
        <div>Đã ôn hôm nay: {s.reviewed_today}</div>
        <div style={{ fontSize: 28, fontWeight: 700 }}>{Math.round(s.retention * 100)}% ghi nhớ</div>
        <div style={{ color: "var(--muted)" }}>🔥 {s.streak_current} ngày · Kỷ lục {s.streak_best} · +{s.north_star} tuần này · {s.total_retained} tổng</div>
      </section>

      <section aria-label="Phân bố mức chấm" style={{ background: "var(--surface)", borderRadius: 14, padding: 16 }}>
        <div style={{ display: "flex", height: 16, borderRadius: 8, overflow: "hidden" }}>
          {distEntries.map(([k, v]) => (
            <span key={k} title={`${k}: ${v}`} style={{ width: `${(v / distTotal) * 100}%`, background: GRADE_COLORS[k] }} />
          ))}
        </div>
      </section>

      <section aria-label="Dự báo tải 30 ngày" style={{ background: "var(--surface)", borderRadius: 14, padding: 16 }}>
        <div style={{ display: "flex", alignItems: "flex-end", gap: 2, height: 80 }}>
          {s.forecast.map((f) => (
            <span key={f.day} title={`${f.day}: ${f.due}`}
              style={{ flex: 1, height: `${(f.due / maxDue) * 100}%`, background: "var(--accent)", borderRadius: 2, minHeight: 2 }} />
          ))}
        </div>
        <div style={{ marginTop: 8 }}>
          <a href="/settings" style={{ color: "var(--accent)" }}>Cấu hình lịch học →</a>
        </div>
      </section>
    </div>
  );
}
```

- [ ] **Step 4: Run — PASS**

Run: `cd web && npx vitest run src/screens/Stats.test.tsx`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
cd .. && git add web/src/screens/Stats.tsx web/src/screens/Stats.test.tsx
git commit -m "feat(web): statistics screen — retention, distribution, forecast, heatmap"
```

---

### Task 13: Frontend — trạng thái empty/loading/error toàn app + never-lose-grade (TDD)

**Files:**
- Create: `web/src/lib/gradeQueue.ts`
- Create: `web/src/components/OfflineBanner.tsx`, `web/src/components/ReviewDone.tsx`
- Test: `web/src/lib/gradeQueue.test.ts`, `web/src/components/OfflineBanner.test.tsx`, `web/src/components/ReviewDone.test.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Viết test grade queue FAIL**

Create `web/src/lib/gradeQueue.test.ts`:
```ts
import { describe, it, expect, beforeEach, vi } from "vitest";
import { enqueueGrade, pendingGrades, flushGrades, PendingGrade } from "./gradeQueue";

beforeEach(() => localStorage.clear());

describe("gradeQueue (không bao giờ mất điểm)", () => {
  it("enqueue ghi vào localStorage và đọc lại được", () => {
    enqueueGrade({ card_id: "c1", grade: 3, client_review_id: "r1" });
    const p = pendingGrades();
    expect(p).toHaveLength(1);
    expect(p[0].card_id).toBe("c1");
  });

  it("giữ nhiều grade theo thứ tự", () => {
    enqueueGrade({ card_id: "c1", grade: 3, client_review_id: "r1" });
    enqueueGrade({ card_id: "c2", grade: 1, client_review_id: "r2" });
    expect(pendingGrades().map((g) => g.card_id)).toEqual(["c1", "c2"]);
  });

  it("flush gửi từng grade; giữ lại cái gửi lỗi (idempotent retry)", async () => {
    enqueueGrade({ card_id: "c1", grade: 3, client_review_id: "r1" });
    enqueueGrade({ card_id: "c2", grade: 2, client_review_id: "r2" });
    const send = vi.fn(async (g: PendingGrade) => g.card_id === "c1"); // c2 fail
    await flushGrades(send);
    expect(send).toHaveBeenCalledTimes(2);
    expect(pendingGrades().map((g) => g.card_id)).toEqual(["c2"]); // c1 xong, c2 giữ lại
  });
});
```

- [ ] **Step 2: Run — FAIL**

Run: `cd web && npx vitest run src/lib/gradeQueue.test.ts`
Expected: FAIL.

- [ ] **Step 3: Viết gradeQueue**

Create `web/src/lib/gradeQueue.ts`:
```ts
export type PendingGrade = { card_id: string; grade: number; client_review_id: string };

const KEY = "memorix.pendingGrades";

function read(): PendingGrade[] {
  try {
    return JSON.parse(localStorage.getItem(KEY) ?? "[]") as PendingGrade[];
  } catch {
    return [];
  }
}
function write(list: PendingGrade[]) {
  localStorage.setItem(KEY, JSON.stringify(list));
}

export function enqueueGrade(g: PendingGrade): void {
  write([...read(), g]);
}

export function pendingGrades(): PendingGrade[] {
  return read();
}

// flushGrades gửi từng grade; giữ lại cái gửi lỗi để retry (server idempotent theo client_review_id).
export async function flushGrades(send: (g: PendingGrade) => Promise<boolean>): Promise<void> {
  const remaining: PendingGrade[] = [];
  for (const g of read()) {
    let ok = false;
    try {
      ok = await send(g);
    } catch {
      ok = false;
    }
    if (!ok) remaining.push(g);
  }
  write(remaining);
}
```

- [ ] **Step 4: Run — PASS**

Run: `cd web && npx vitest run src/lib/gradeQueue.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Viết OfflineBanner + ReviewDone + test**

Create `web/src/components/OfflineBanner.test.tsx`:
```tsx
import { render, screen, act } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import OfflineBanner from "./OfflineBanner";

describe("OfflineBanner", () => {
  it("ẩn khi online, hiện banner non-blocking khi offline", () => {
    render(<OfflineBanner />);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    act(() => { window.dispatchEvent(new Event("offline")); });
    const b = screen.getByRole("status");
    expect(b).toHaveTextContent(/điểm đã lưu, sẽ sync/);
    act(() => { window.dispatchEvent(new Event("online")); });
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });
});
```

Create `web/src/components/OfflineBanner.tsx`:
```tsx
import { useEffect, useState } from "react";

export default function OfflineBanner() {
  const [offline, setOffline] = useState(false);
  useEffect(() => {
    const off = () => setOffline(true);
    const on = () => setOffline(false);
    window.addEventListener("offline", off);
    window.addEventListener("online", on);
    return () => {
      window.removeEventListener("offline", off);
      window.removeEventListener("online", on);
    };
  }, []);
  if (!offline) return null;
  return (
    <div role="status" style={{
      position: "sticky", top: 0, zIndex: 10, padding: "8px 16px",
      background: "var(--hard)", color: "#fff", textAlign: "center",
    }}>
      Mất mạng — điểm đã lưu, sẽ sync khi có kết nối.
    </div>
  );
}
```

Create `web/src/components/ReviewDone.test.tsx`:
```tsx
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import ReviewDone from "./ReviewDone";

describe("ReviewDone celebration", () => {
  it("ăn mừng với số từ nhớ được + forecast mai", () => {
    render(<ReviewDone retained={12} tomorrow={8} />);
    expect(screen.getByText(/\+12 từ nhớ được/)).toBeInTheDocument();
    expect(screen.getByText(/Ngày mai: 8 thẻ/)).toBeInTheDocument();
  });
});
```

Create `web/src/components/ReviewDone.tsx`:
```tsx
export default function ReviewDone({ retained, tomorrow }: { retained: number; tomorrow: number }) {
  return (
    <div role="status" style={{ padding: 24, textAlign: "center" }}>
      <div style={{ fontSize: 48 }}>🎉</div>
      <div style={{ fontSize: 28, fontWeight: 700, color: "var(--good)" }}>+{retained} từ nhớ được</div>
      <p style={{ color: "var(--muted)" }}>Hết thẻ phiên này. Ngày mai: {tomorrow} thẻ.</p>
    </div>
  );
}
```

- [ ] **Step 6: Wire OfflineBanner + màn Dashboard/Stats vào App.tsx**

Modify `web/src/App.tsx` — import và render:
```tsx
import OfflineBanner from "./components/OfflineBanner";
import Dashboard from "./screens/Dashboard";
import Stats from "./screens/Stats";
```
Thay khối `<main>` placeholder bằng:
```tsx
      <OfflineBanner />
      <main style={{ flex: 1 }}>
        {tab === 0 && <Dashboard />}
        {tab === 3 && <Stats />}
        {(tab === 1 || tab === 2) && (
          <div style={{ padding: 16 }}>
            <h1>{TABS[tab]}</h1>
            <p style={{ color: "var(--muted)" }}>Màn do sprint khác triển khai.</p>
          </div>
        )}
      </main>
```
> `OfflineBanner` đặt ngay dưới `<header>`, trên `<main>`. Giữ nguyên `<nav>` bottom-tab của Sprint 0.

- [ ] **Step 7: Run toàn bộ FE test**

Run: `cd web && npx vitest run`
Expected: PASS (Dashboard 4 + Stats 3 + gradeQueue 3 + OfflineBanner 1 + ReviewDone 1 + App shell Sprint 0).

- [ ] **Step 8: Verify build**

Run: `cd web && npm run build`
Expected: build thành công (App render Dashboard/Stats + OfflineBanner).

- [ ] **Step 9: Commit**

```bash
cd .. && git add web/src/lib/gradeQueue.ts web/src/lib/gradeQueue.test.ts web/src/components/ web/src/App.tsx
git commit -m "feat(web): app-wide states — offline banner, review-done celebration, never-lose-grade queue"
```

---
## Self-Review

**Spec coverage (Story AC → task):**

| Story / AC | Task |
|---|---|
| 5.1 — bảng `daily_stats`+`study_profiles` | Task 1 |
| 5.1 — CardGraded async fire-and-forget cập nhật daily_stats (AD-8) | Task 5 (ingest) + Task 10 (Subscribe wiring) |
| 5.1 — River reconcile rebuild daily_stats từ review_logs (AD-4) | Task 4 (RebuildDailyStats) + Task 9 (Reconciler + River worker + register cmd/worker) |
| 5.2 — trang chủ due + CTA "Ôn ngay" + new-today + streak; badge due | Task 8 (dashboard API) + Task 11 (FE, CTA/streak/new). Badge số due tab Ôn: dữ liệu `due_count` có sẵn ở dashboard API — FE tab badge để sprint Review wire (dữ liệu sẵn). |
| 5.3 — North Star = recall đúng ∧ interval ≥7d; số tức thì đọc thẳng review_logs (AD-8) | Task 2 (CountWordsRetained) + Task 7 (WeekRetentionLogs) + Task 8 (`Reader.northStar` dùng logs, không daily_stats) + Task 11 (hiển thị) |
| 5.4 — streak gắn recall thật, reset khi lỡ ngày, retained tích lũy KHÔNG reset (AD-12) | Task 3 (ApplyStudyDay + test reset giữ total_retained) |
| 5.5 — đã ôn hôm nay, phân bố mức chấm, heatmap, forecast 7&30 ngày | Task 7 (Distribution/Heatmap/Forecast) + Task 8 (StatsView) + Task 12 (FE). Link Settings cấu hình lịch (UX-DR10). |
| 5.6 — empty/loading/error toàn app; review-done ăn mừng; skeleton; offline không mất grade; 401 refresh ngầm | Task 11 (dashboard skeleton/empty/error) + Task 12 (stats states) + Task 13 (OfflineBanner + ReviewDone + gradeQueue never-lose). 401 refresh ngầm = trách nhiệm auth client Sprint 1 (interceptor) — dữ liệu ghi cục bộ qua gradeQueue đảm bảo không mất khi 401/offline. |

**Placeholder scan:** Không có TBD/TODO. Các "lưu ý" (River API version, tên `v1auth`, seed cột cards/logs, migration number, TZ per-user deferred) là **điểm nối tới code Sprint 1–4 thật**, không phải code thiếu — mọi file/hàm trong scope đều có code hoàn chỉnh.

**Type consistency:**
- Domain: `Day`, `RetentionLog`, `LogRow`, `DailyStat`, `StudyProfile`, `CountWordsRetained`, `IsRetained`, `ApplyStudyDay`, `RebuildStudyProfile`, `RebuildDailyStats` — dùng nhất quán Task 2/3/4 → repo/service/worker.
- `events.CardGraded{OwnerID,CardID,Grade,ScheduledDays,WasNew,ReviewedAt}` + `CardGradedName` — publisher (review, giả định) ↔ subscriber (Task 5).
- Repo methods: `BumpDailyStat/GetStudyProfile/UpsertStudyProfile/DistinctOwners/AllLogsForOwner/ReplaceDailyStats` (write, Task 6) + `WeekRetentionLogs/DueCount/Forecast/TodayStat/Heatmap/Distribution` (read, Task 7) — khớp interface `IngestRepo`/`ReconcileRepo`/`ReadRepo` ở service.
- Service views `DashboardView`/`StatsView`/`HeatCell`/`ForecastCell`/`Distribution` — khớp handler (Task 8) và FE types `Dashboard`/`Stats` (Task 11, JSON tags snake_case ↔ TS fields).
- `TZResolver`/`UTCResolver` dùng chung ingest (Task 5) + reconcile (Task 9).

**Gaps (chủ ý, ngoài scope sprint):**
1. **TZ per-user** phân giải qua `TZResolver` (`Handler.userLoc` ở request-path, ingest/reconcile ở background). MVP wire `UTCResolver`; prod wire adapter bọc `IdentityPort.UserTimezone` (AD-9) — request-path và background dùng cùng seam. Handler KHÔNG đọc TZ từ gin context (TZ không nằm trong principal — Auth Contract). Đây là điểm deferred cuối (đúng AD-8: read-model eventual).
2. **Cấu hình lịch** (desired-retention slider, daily limit — UX-DR10) thuộc scheduling prefs (Story 3.2/4.2); màn Stats chỉ link `/settings`.
3. **Badge số due trên tab Ôn** (UX-DR2): API cấp `due_count`; wire badge vào bottom-nav thuộc sprint Review khi tab Ôn có logic; dữ liệu đã sẵn.
4. **AD-9 lưu ý:** progress đọc `review.review_logs` (được AD-4/AD-8 cho phép rõ ràng: read model rebuild-từ-log) và đọc read-only `scheduling.cards.due_at` cho forecast. Đây là read-model access; nếu siết AD-9 tuyệt đối, thay bằng `SchedulingPort.ForecastDue` — deferred, ghi nhận.

**Verify sweep cuối (chạy sau Task 13):**
```bash
go build ./... && go test ./... -short && golangci-lint run ./...
go test ./internal/progress/... -run 'Migrate|Repo'   # cần Docker (testcontainers)
cd web && npx vitest run && npm run build
```
Expected: tất cả xanh.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-07-sprint-5-progress.md`. Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** — execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
