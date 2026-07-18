# Sprint 2 — Vocabulary & Starter Deck Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Xây module `vocabulary` (entries + curated deck + enrollment) và một `scheduling` tối thiểu (bảng `cards` + card-creation port) để người học thêm/sửa/xóa từ, tự tạo thẻ New chéo-module, và enroll bộ IELTS khởi đầu — Epic 2 (Story 2.1–2.6).

**Architecture:** Modular Monolith + Hexagonal. `vocabulary` là module nhẹ (service + repo, port chỉ nơi expose). Cross-module theo **AD-9**: vocabulary gọi scheduling qua interface `CardService` (định nghĩa ở phía gọi `vocabulary/ports` để tránh import cycle; `scheduling.Service` hiện thực; ráp ở `cmd/api`). Vocabulary **không** ghi thẳng `scheduling.cards`. FK chỉ trong cùng schema (**AD-10**); tham chiếu chéo (`cards.entry_id`, `owner_id`) là cột id logic, không FK vật lý. Enroll bulk-create thẻ qua **River job** (idempotent). Entry/Card tách biệt (**AD-6**): curated `owner_id = NULL`.

**Tech Stack:** Go 1.26, Gin v1.10, pgx v5 (+pgxpool), golang-migrate v4, River v0, google/uuid, testcontainers-go, Postgres 18 (extensions unaccent/citext/pgcrypto từ Sprint 0). Reuse Sprint 0: `platform/httpx` (APIError/Cursor/Page), `platform/config`, `platform/logger`, `platform/eventbus`, `platform/db.Migrate`.

**Nguồn:** `_bmad-output/planning-artifacts/epics.md` (Epic 2, Story 2.1–2.6) · `ARCHITECTURE-SPINE.md` (AD-6, AD-9, AD-10, AD-14) · `addendum-structure.md` (S1–S7) · `docs/superpowers/plans/2026-07-07-sprint-0-foundation.md` (platform types).

**Assumptions từ Sprint 1:** `identity.users` tồn tại; auth middleware set principal vào context. Handler đọc principal qua một `PrincipalFunc` inject ở wiring (bọc `authmw`), nên module này **không** import internal của identity. Tests inject stub principal.

**Scope boundary:** KHÔNG làm FSRS grade/queue (Epic 3–4). `scheduling.cards` sprint này chỉ tạo card **New** (S/D/due mặc định 0/NULL). KHÔNG làm daily-limit rải thẻ (Epic 4). Onboarding goal-setting (desired retention) = Epic 3; sprint này chỉ cung cấp endpoint list curated-decks + enroll để onboarding/empty-state dùng.

**Quyết định vị trí `deck_enrollments`:** đặt trong schema **`vocabulary`** (không phải scheduling) để FK `curated_deck_id → vocabulary.curated_decks` nằm **trong cùng schema** (AD-10). `owner_id` là ref logic tới identity. Enrollment do vocabulary sở hữu; nó nhờ scheduling tạo card qua port.

---

## Cross-Sprint Auth Contract (canonical — Sprint 1)

Sprint 1 sở hữu `internal/platform/authmw`. Downstream **phải** dùng đúng API này, không tự chế reader/context-key:
- `authmw.RequireAuth(jwtManager) gin.HandlerFunc` — guard route cần đăng nhập (đặt principal vào gin context).
- `authmw.PrincipalFrom(c) (Principal, bool)` · `Principal{UserID string, Role string, Plan string}`.
- `authmw.UserID(c) (string, bool)` — reader tiện lợi; **UserID là uuid dạng string**. `uuid.Parse(uid)` ở ranh giới repo nếu cần `uuid.UUID`.
- **Timezone KHÔNG nằm trong principal/context** — lấy qua `IdentityPort.UserTimezone(ctx, userID) (string, error)` (AD-9) rồi `time.LoadLocation` (AD-12).
- Test: fake bằng middleware test gọi `c.Set` với đúng `authmw.Principal{...}`, KHÔNG dùng key thô `"user_id"`.

> Áp dụng: thay mọi `c.Get("user_id")`/`c.GetString("timezone")` thô bằng hợp đồng trên.

---

### Task 1: Migration 0002 — vocabulary schema tables

**Files:**
- Create: `migrations/0002_vocabulary.up.sql`
- Create: `migrations/0002_vocabulary.down.sql`
- Test: `internal/platform/db/migrate_test.go` (thêm case)

- [ ] **Step 1: Write migration up**

Create `migrations/0002_vocabulary.up.sql`:
```sql
-- Vocabulary schema (AD-6 Entry tách Card; AD-10 FK chỉ trong cùng schema).

-- Wrapper IMMUTABLE để dùng unaccent trong generated column / index.
-- unaccent extension cài ở public (Sprint 0). Chỉ định regdictionary để immutable-safe.
CREATE OR REPLACE FUNCTION vocabulary.immutable_unaccent(text)
RETURNS text LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS
$$ SELECT public.unaccent('public.unaccent'::regdictionary, $1) $$;

CREATE TABLE vocabulary.curated_decks (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        text NOT NULL UNIQUE,
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE vocabulary.entries (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        uuid,                                       -- NULL = curated (AD-6); ref logic identity.users (AD-10)
    curated_deck_id uuid REFERENCES vocabulary.curated_decks(id) ON DELETE CASCADE, -- FK cùng schema OK
    term            text NOT NULL,
    term_normalized text GENERATED ALWAYS AS (vocabulary.immutable_unaccent(lower(term))) STORED,
    part_of_speech  text NOT NULL DEFAULT '',
    notes           text NOT NULL DEFAULT '',
    source          text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz
);

-- Trùng theo (owner, term chuẩn hóa) chỉ tính bản chưa xóa (FR-10).
CREATE UNIQUE INDEX uq_entries_owner_termnorm
    ON vocabulary.entries (owner_id, term_normalized)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_entries_owner_created
    ON vocabulary.entries (owner_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_entries_curated_deck
    ON vocabulary.entries (curated_deck_id)
    WHERE curated_deck_id IS NOT NULL AND deleted_at IS NULL;

-- gin FTS index (yêu cầu sprint; phục vụ search list ?q=).
CREATE INDEX idx_entries_fts
    ON vocabulary.entries
    USING gin (to_tsvector('english', term || ' ' || notes));

CREATE TABLE vocabulary.meanings (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id       uuid NOT NULL REFERENCES vocabulary.entries(id) ON DELETE CASCADE,
    part_of_speech text NOT NULL DEFAULT '',
    definition     text NOT NULL,
    position       int  NOT NULL DEFAULT 0
);
CREATE INDEX idx_meanings_entry ON vocabulary.meanings(entry_id);

CREATE TABLE vocabulary.examples (
    id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id uuid NOT NULL REFERENCES vocabulary.entries(id) ON DELETE CASCADE,
    text     text NOT NULL,
    position int  NOT NULL DEFAULT 0
);
CREATE INDEX idx_examples_entry ON vocabulary.examples(entry_id);

CREATE TABLE vocabulary.pronunciations (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id  uuid NOT NULL REFERENCES vocabulary.entries(id) ON DELETE CASCADE,
    ipa       text NOT NULL DEFAULT '',
    dialect   text NOT NULL DEFAULT '',
    audio_url text NOT NULL DEFAULT ''
);
CREATE INDEX idx_pron_entry ON vocabulary.pronunciations(entry_id);

CREATE TABLE vocabulary.synonyms_antonyms (
    id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id uuid NOT NULL REFERENCES vocabulary.entries(id) ON DELETE CASCADE,
    relation text NOT NULL CHECK (relation IN ('synonym','antonym')),
    value    text NOT NULL
);
CREATE INDEX idx_synant_entry ON vocabulary.synonyms_antonyms(entry_id);

CREATE TABLE vocabulary.deck_enrollments (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        uuid NOT NULL,                              -- ref logic identity.users
    curated_deck_id uuid NOT NULL REFERENCES vocabulary.curated_decks(id) ON DELETE CASCADE,
    status          text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','completed')),
    card_count      int  NOT NULL DEFAULT 0,
    enrolled_at     timestamptz NOT NULL DEFAULT now(),
    completed_at    timestamptz
);
CREATE UNIQUE INDEX uq_enrollment_owner_deck
    ON vocabulary.deck_enrollments (owner_id, curated_deck_id);
```

- [ ] **Step 2: Write migration down**

Create `migrations/0002_vocabulary.down.sql`:
```sql
DROP TABLE IF EXISTS vocabulary.deck_enrollments;
DROP TABLE IF EXISTS vocabulary.synonyms_antonyms;
DROP TABLE IF EXISTS vocabulary.pronunciations;
DROP TABLE IF EXISTS vocabulary.examples;
DROP TABLE IF EXISTS vocabulary.meanings;
DROP TABLE IF EXISTS vocabulary.entries;
DROP TABLE IF EXISTS vocabulary.curated_decks;
DROP FUNCTION IF EXISTS vocabulary.immutable_unaccent(text);
```

- [ ] **Step 3: Write the failing test**

Append to `internal/platform/db/migrate_test.go`:
```go
func TestMigrate_CreatesVocabularyTables(t *testing.T) {
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
	defer pg.Terminate(ctx)

	dsn, _ := pg.ConnectionString(ctx, "sslmode=disable")
	if err := Migrate("file://../../../migrations", dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	conn, _ := pgx.Connect(ctx, dsn)
	defer conn.Close(ctx)

	var n int
	err = conn.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables
		 WHERE table_schema='vocabulary'
		 AND table_name IN ('entries','meanings','examples','pronunciations',
		                    'synonyms_antonyms','curated_decks','deck_enrollments')`).Scan(&n)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	if n != 7 {
		t.Errorf("expected 7 vocabulary tables, got %d", n)
	}

	// Generated column term_normalized bỏ dấu + lower.
	var norm string
	if err := conn.QueryRow(ctx,
		`INSERT INTO vocabulary.entries (owner_id, term) VALUES (gen_random_uuid(), 'Résumé')
		 RETURNING term_normalized`).Scan(&norm); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if norm != "resume" {
		t.Errorf("term_normalized = %q, want %q", norm, "resume")
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/platform/db/ -run TestMigrate_CreatesVocabularyTables -v`
Expected: FAIL (bảng chưa tồn tại → count != 7).

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/platform/db/ -run TestMigrate_CreatesVocabularyTables -v`
Expected: PASS (7 bảng; `term_normalized` = "resume").

- [ ] **Step 6: Commit**

```bash
git add migrations/0002_vocabulary.up.sql migrations/0002_vocabulary.down.sql internal/platform/db/migrate_test.go
git commit -m "feat(db): vocabulary schema tables, normalized unique index, gin FTS (AD-6, AD-10)"
```

---

### Task 2: Migration 0003 — scheduling.cards

**Files:**
- Create: `migrations/0003_scheduling_cards.up.sql`
- Create: `migrations/0003_scheduling_cards.down.sql`
- Test: `internal/platform/db/migrate_test.go` (thêm case)

- [ ] **Step 1: Write migration up**

Create `migrations/0003_scheduling_cards.up.sql`:
```sql
-- Card = trạng thái học per-user, per-direction (AD-6). entry_id/owner_id là ref
-- logic (không FK chéo schema, AD-10). Sprint 2 chỉ tạo card New; FSRS = Epic 3.
CREATE TABLE scheduling.cards (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   uuid NOT NULL,                                   -- ref logic identity.users
    entry_id   uuid NOT NULL,                                   -- ref logic vocabulary.entries
    direction  text NOT NULL DEFAULT 'front_back' CHECK (direction IN ('front_back','back_front')),
    status     text NOT NULL DEFAULT 'new' CHECK (status IN ('new','learning','review','relearning','suspended')),
    due_at     timestamptz,
    stability  double precision NOT NULL DEFAULT 0,
    difficulty double precision NOT NULL DEFAULT 0,
    reps       int NOT NULL DEFAULT 0,
    lapses     int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT uq_cards_owner_entry_dir UNIQUE (owner_id, entry_id, direction)
);
CREATE INDEX idx_cards_owner_status ON scheduling.cards (owner_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_cards_owner_entry  ON scheduling.cards (owner_id, entry_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_cards_owner_due    ON scheduling.cards (owner_id, due_at) WHERE deleted_at IS NULL;
```

- [ ] **Step 2: Write migration down**

Create `migrations/0003_scheduling_cards.down.sql`:
```sql
DROP TABLE IF EXISTS scheduling.cards;
```

- [ ] **Step 3: Write the failing test**

Append to `internal/platform/db/migrate_test.go`:
```go
func TestMigrate_CreatesSchedulingCards(t *testing.T) {
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
	defer pg.Terminate(ctx)

	dsn, _ := pg.ConnectionString(ctx, "sslmode=disable")
	if err := Migrate("file://../../../migrations", dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	conn, _ := pgx.Connect(ctx, dsn)
	defer conn.Close(ctx)

	owner := "11111111-1111-1111-1111-111111111111"
	entry := "22222222-2222-2222-2222-222222222222"
	for i := 0; i < 2; i++ {
		_, err = conn.Exec(ctx,
			`INSERT INTO scheduling.cards (owner_id, entry_id, direction) VALUES ($1,$2,'front_back')
			 ON CONFLICT (owner_id, entry_id, direction) DO NOTHING`, owner, entry)
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	var n int
	if err := conn.QueryRow(ctx,
		`SELECT count(*) FROM scheduling.cards WHERE owner_id=$1 AND entry_id=$2`, owner, entry).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("idempotent insert produced %d rows, want 1", n)
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/platform/db/ -run TestMigrate_CreatesSchedulingCards -v`
Expected: FAIL (bảng `scheduling.cards` chưa tồn tại).

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/platform/db/ -run TestMigrate_CreatesSchedulingCards -v`
Expected: PASS (unique constraint làm insert lần 2 no-op).

- [ ] **Step 6: Commit**

```bash
git add migrations/0003_scheduling_cards.up.sql migrations/0003_scheduling_cards.down.sql internal/platform/db/migrate_test.go
git commit -m "feat(db): scheduling.cards table with per-direction unique (AD-6, AD-10)"
```

---

### Task 3: platform/db — Connect pool + dbtest helper

**Files:**
- Create: `internal/platform/db/pool.go`
- Create: `internal/platform/db/dbtest/dbtest.go`

- [ ] **Step 1: Add uuid dep**

Run:
```bash
go get github.com/google/uuid@latest
```
Expected: uuid thêm vào go.mod.

- [ ] **Step 2: Write pool connector**

Create `internal/platform/db/pool.go`:
```go
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect mở pgxpool tới databaseURL.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, databaseURL)
}
```

- [ ] **Step 3: Write reusable test helper (DRY cho mọi repo test)**

Create `internal/platform/db/dbtest/dbtest.go`:
```go
// Package dbtest cung cấp Postgres testcontainer + migrate cho integration test.
package dbtest

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/memorix/memorix/internal/platform/db"
)

// migrationsURL trả file://<repo>/migrations tính từ vị trí package này.
func migrationsURL() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile = <repo>/internal/platform/db/dbtest/dbtest.go
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	return "file://" + filepath.Join(root, "migrations")
}

// RunPostgres khởi động Postgres 18, áp mọi migration, trả pool đã sẵn sàng.
// Skip khi -short. Tự dọn qua t.Cleanup.
func RunPostgres(t *testing.T) *pgxpool.Pool {
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
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	if err := db.Migrate(migrationsURL(), dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		_ = pg.Terminate(ctx)
	})
	return pool
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./internal/platform/db/...`
Expected: no error.

- [ ] **Step 5: Commit**

```bash
git add internal/platform/db/pool.go internal/platform/db/dbtest/dbtest.go go.mod go.sum
git commit -m "feat(platform): pgxpool connector and dbtest container helper"
```

---

### Task 4: vocabulary/domain — Entry, Direction, validation (pure)

**Files:**
- Create: `internal/vocabulary/domain/entry.go`
- Create: `internal/vocabulary/domain/deck.go`
- Create: `internal/vocabulary/domain/errors.go`
- Test: `internal/vocabulary/domain/entry_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/vocabulary/domain/entry_test.go`:
```go
package domain

import "testing"

func TestValidateTerm(t *testing.T) {
	got, err := ValidateTerm("  hello  ")
	if err != nil || got != "hello" {
		t.Fatalf("ValidateTerm trim = %q, %v", got, err)
	}
	if _, err := ValidateTerm("   "); err != ErrTermRequired {
		t.Errorf("blank term err = %v, want ErrTermRequired", err)
	}
}

func TestDirectionValid(t *testing.T) {
	if !DirectionFrontBack.Valid() || !DirectionBackFront.Valid() {
		t.Error("known directions must be valid")
	}
	if Direction("sideways").Valid() {
		t.Error("unknown direction must be invalid")
	}
}

func TestDefaultDirections(t *testing.T) {
	got := DefaultDirections(nil)
	if len(got) != 1 || got[0] != DirectionFrontBack {
		t.Fatalf("default = %v, want [front_back]", got)
	}
	both := DefaultDirections([]Direction{DirectionFrontBack, DirectionBackFront})
	if len(both) != 2 {
		t.Errorf("passthrough = %v", both)
	}
	if bad := DefaultDirections([]Direction{"x"}); len(bad) != 1 || bad[0] != DirectionFrontBack {
		t.Errorf("invalid direction should fall back to default, got %v", bad)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/vocabulary/domain/ -v`
Expected: FAIL (undefined `ValidateTerm`/`Direction`).

- [ ] **Step 3: Write implementation**

Create `internal/vocabulary/domain/entry.go`:
```go
// Package domain là lõi thuần của vocabulary (AD-2: không import hạ tầng).
package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Direction string

const (
	DirectionFrontBack Direction = "front_back"
	DirectionBackFront Direction = "back_front"
)

func (d Direction) Valid() bool {
	return d == DirectionFrontBack || d == DirectionBackFront
}

// DefaultDirections áp mặc định front→back (FR-8); lọc giá trị hợp lệ.
func DefaultDirections(in []Direction) []Direction {
	var out []Direction
	for _, d := range in {
		if d.Valid() {
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		return []Direction{DirectionFrontBack}
	}
	return out
}

type Relation string

const (
	RelationSynonym Relation = "synonym"
	RelationAntonym Relation = "antonym"
)

type Meaning struct {
	ID           uuid.UUID
	PartOfSpeech string
	Definition   string
	Position     int
}

type Example struct {
	ID       uuid.UUID
	Text     string
	Position int
}

type Pronunciation struct {
	ID       uuid.UUID
	IPA      string
	Dialect  string
	AudioURL string
}

type SynAnt struct {
	ID       uuid.UUID
	Relation Relation
	Value    string
}

// Entry giữ nội dung từ (AD-6). OwnerID nil = curated (owner_id NULL).
type Entry struct {
	ID             uuid.UUID
	OwnerID        *uuid.UUID
	CuratedDeckID  *uuid.UUID
	Term           string
	PartOfSpeech   string
	Notes          string
	Source         string
	Meanings       []Meaning
	Examples       []Example
	Pronunciations []Pronunciation
	Relations      []SynAnt
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// ValidateTerm trim + bắt buộc không rỗng (FR-7).
func ValidateTerm(term string) (string, error) {
	t := strings.TrimSpace(term)
	if t == "" {
		return "", ErrTermRequired
	}
	return t, nil
}
```

Create `internal/vocabulary/domain/deck.go`:
```go
package domain

import "github.com/google/uuid"

// CuratedDeck là bộ thẻ khởi đầu seed sẵn (FR-11a, AD-6).
type CuratedDeck struct {
	ID          uuid.UUID
	Slug        string
	Name        string
	Description string
	IsActive    bool
}
```

Create `internal/vocabulary/domain/errors.go`:
```go
package domain

import "errors"

var (
	ErrTermRequired    = errors.New("term is required")
	ErrEntryNotFound   = errors.New("entry not found")
	ErrDeckNotFound    = errors.New("curated deck not found")
	ErrAlreadyEnrolled = errors.New("already enrolled in deck")
	ErrDuplicateTerm   = errors.New("duplicate term for owner")
)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/vocabulary/domain/ -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/vocabulary/domain/
git commit -m "feat(vocabulary): domain entities, direction defaults, validation (AD-6)"
```

---

### Task 5: scheduling/domain — Card, CardStatus (pure)

**Files:**
- Create: `internal/scheduling/domain/card.go`
- Test: `internal/scheduling/domain/card_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/scheduling/domain/card_test.go`:
```go
package domain

import "testing"

func TestCardStatusValid(t *testing.T) {
	for _, s := range []CardStatus{StatusNew, StatusLearning, StatusReview, StatusRelearning, StatusSuspended} {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if CardStatus("done").Valid() {
		t.Error("unknown status must be invalid")
	}
}

func TestDirectionValid(t *testing.T) {
	if !DirectionFrontBack.Valid() || Direction("x").Valid() {
		t.Error("direction validity wrong")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scheduling/domain/ -v`
Expected: FAIL (undefined `CardStatus`).

- [ ] **Step 3: Write implementation**

Create `internal/scheduling/domain/card.go`:
```go
// Package domain là lõi thuần của scheduling (AD-2).
package domain

import (
	"time"

	"github.com/google/uuid"
)

type CardStatus string

const (
	StatusNew        CardStatus = "new"
	StatusLearning   CardStatus = "learning"
	StatusReview     CardStatus = "review"
	StatusRelearning CardStatus = "relearning"
	StatusSuspended  CardStatus = "suspended"
)

func (s CardStatus) Valid() bool {
	switch s {
	case StatusNew, StatusLearning, StatusReview, StatusRelearning, StatusSuspended:
		return true
	}
	return false
}

type Direction string

const (
	DirectionFrontBack Direction = "front_back"
	DirectionBackFront Direction = "back_front"
)

func (d Direction) Valid() bool {
	return d == DirectionFrontBack || d == DirectionBackFront
}

// Card giữ trạng thái học per-user/per-direction (AD-6). Sprint 2 chỉ tạo New.
type Card struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	EntryID    uuid.UUID
	Direction  Direction
	Status     CardStatus
	DueAt      *time.Time
	Stability  float64
	Difficulty float64
	Reps       int
	Lapses     int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scheduling/domain/ -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/scheduling/domain/
git commit -m "feat(scheduling): card domain entity and status/direction value objects"
```

---

### Task 6: scheduling/repo — CardRepo (pgx, container tests)

**Files:**
- Create: `internal/scheduling/repo/card_repo.go`
- Test: `internal/scheduling/repo/card_repo_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/scheduling/repo/card_repo_test.go`:
```go
package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
)

func TestCardRepo_CreateAndStatuses(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	owner := uuid.New()
	entry := uuid.New()

	// Tạo 2 direction, gọi lại lần 2 phải idempotent.
	for i := 0; i < 2; i++ {
		if err := r.CreateCardsForEntry(ctx, owner, entry, []string{"front_back", "back_front"}); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	statuses, err := r.CardStatusesByEntry(ctx, owner, []uuid.UUID{entry})
	if err != nil {
		t.Fatalf("statuses: %v", err)
	}
	if statuses[entry] != "new" {
		t.Errorf("primary status = %q, want new", statuses[entry])
	}

	ids, err := r.EntryIDsByStatus(ctx, owner, "new")
	if err != nil {
		t.Fatalf("byStatus: %v", err)
	}
	if len(ids) != 1 || ids[0] != entry {
		t.Errorf("EntryIDsByStatus = %v, want [%s]", ids, entry)
	}
}

func TestCardRepo_BulkCreateIdempotent(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	owner := uuid.New()
	e1, e2 := uuid.New(), uuid.New()

	n, err := r.BulkCreateForDeck(ctx, owner, []uuid.UUID{e1, e2})
	if err != nil || n != 2 {
		t.Fatalf("first bulk = %d, %v; want 2", n, err)
	}
	n2, err := r.BulkCreateForDeck(ctx, owner, []uuid.UUID{e1, e2})
	if err != nil || n2 != 0 {
		t.Fatalf("second bulk = %d, %v; want 0 (idempotent)", n2, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scheduling/repo/ -run TestCardRepo -v`
Expected: FAIL (undefined `New`/repo methods).

- [ ] **Step 3: Write implementation**

Create `internal/scheduling/repo/card_repo.go`:
```go
// Package repo là adapter Postgres của scheduling (implements ports).
package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CardRepo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *CardRepo { return &CardRepo{pool: pool} }

// CreateCardsForEntry tạo 1 card New / direction (idempotent qua unique).
func (r *CardRepo) CreateCardsForEntry(ctx context.Context, ownerID, entryID uuid.UUID, directions []string) error {
	for _, d := range directions {
		_, err := r.pool.Exec(ctx,
			`INSERT INTO scheduling.cards (owner_id, entry_id, direction, status)
			 VALUES ($1, $2, $3, 'new')
			 ON CONFLICT (owner_id, entry_id, direction) DO NOTHING`,
			ownerID, entryID, d)
		if err != nil {
			return err
		}
	}
	return nil
}

// CardStatusesByEntry trả status card primary (front_back) theo entry.
func (r *CardRepo) CardStatusesByEntry(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(entryIDs))
	if len(entryIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT entry_id, status FROM scheduling.cards
		 WHERE owner_id = $1 AND entry_id = ANY($2) AND direction = 'front_back' AND deleted_at IS NULL`,
		ownerID, entryIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var st string
		if err := rows.Scan(&id, &st); err != nil {
			return nil, err
		}
		out[id] = st
	}
	return out, rows.Err()
}

// EntryIDsByStatus trả entry có >=1 card ở status (lọc list, không join chéo schema).
func (r *CardRepo) EntryIDsByStatus(ctx context.Context, ownerID uuid.UUID, status string) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT entry_id FROM scheduling.cards
		 WHERE owner_id = $1 AND status = $2 AND deleted_at IS NULL`,
		ownerID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// BulkCreateForDeck tạo card New (front_back) cho mỗi entry; trả số card mới tạo.
func (r *CardRepo) BulkCreateForDeck(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (int, error) {
	if len(entryIDs) == 0 {
		return 0, nil
	}
	tag, err := r.pool.Exec(ctx,
		`INSERT INTO scheduling.cards (owner_id, entry_id, direction, status)
		 SELECT $1, e, 'front_back', 'new' FROM unnest($2::uuid[]) AS e
		 ON CONFLICT (owner_id, entry_id, direction) DO NOTHING`,
		ownerID, entryIDs)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scheduling/repo/ -run TestCardRepo -v`
Expected: PASS (2 tests; idempotency giữ số card đúng).

- [ ] **Step 5: Commit**

```bash
git add internal/scheduling/repo/
git commit -m "feat(scheduling): card repo with idempotent create and status batch queries (AD-9)"
```

---

### Task 7: scheduling/service — CardService implementation

**Files:**
- Create: `internal/scheduling/service/service.go`
- Test: `internal/scheduling/service/service_test.go`

- [ ] **Step 1: Write the failing test (fake repo)**

Create `internal/scheduling/service/service_test.go`:
```go
package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type fakeRepo struct {
	created   [][]string
	bulkCalls int
}

func (f *fakeRepo) CreateCardsForEntry(_ context.Context, _ uuid.UUID, _ uuid.UUID, dirs []string) error {
	f.created = append(f.created, dirs)
	return nil
}
func (f *fakeRepo) CardStatusesByEntry(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	m := map[uuid.UUID]string{}
	for _, id := range ids {
		m[id] = "new"
	}
	return m, nil
}
func (f *fakeRepo) EntryIDsByStatus(_ context.Context, _ uuid.UUID, _ string) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *fakeRepo) BulkCreateForDeck(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (int, error) {
	f.bulkCalls++
	return len(ids), nil
}

func TestService_CreateCardsForEntry_DefaultsDirection(t *testing.T) {
	f := &fakeRepo{}
	svc := New(f)
	in := CreateCardsInput{OwnerID: uuid.New(), EntryID: uuid.New(), Directions: nil}
	if err := svc.CreateCardsForEntry(context.Background(), in); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(f.created) != 1 || len(f.created[0]) != 1 || f.created[0][0] != "front_back" {
		t.Errorf("expected default front_back, got %v", f.created)
	}
}

func TestService_CreateCardsForEntry_FiltersInvalid(t *testing.T) {
	f := &fakeRepo{}
	svc := New(f)
	in := CreateCardsInput{OwnerID: uuid.New(), EntryID: uuid.New(), Directions: []string{"front_back", "back_front", "junk"}}
	if err := svc.CreateCardsForEntry(context.Background(), in); err != nil {
		t.Fatalf("create: %v", err)
	}
	if got := f.created[0]; len(got) != 2 {
		t.Errorf("invalid direction not filtered: %v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scheduling/service/ -v`
Expected: FAIL (undefined `New`/`CreateCardsInput`).

- [ ] **Step 3: Write implementation**

Create `internal/scheduling/service/service.go`:
```go
// Package service là use case của scheduling. Service hiện thực cổng mà
// vocabulary cần (vocabulary/ports.CardService) — ráp ở cmd/api (AD-9).
package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/scheduling/domain"
)

// CardRepo là cổng lưu trữ (repo implements).
type CardRepo interface {
	CreateCardsForEntry(ctx context.Context, ownerID, entryID uuid.UUID, directions []string) error
	CardStatusesByEntry(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]string, error)
	EntryIDsByStatus(ctx context.Context, ownerID uuid.UUID, status string) ([]uuid.UUID, error)
	BulkCreateForDeck(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (int, error)
}

// CreateCardsInput khớp vocabulary/ports.CreateCardsInput (cùng shape).
type CreateCardsInput struct {
	OwnerID    uuid.UUID
	EntryID    uuid.UUID
	Directions []string
}

type Service struct {
	repo CardRepo
}

func New(repo CardRepo) *Service { return &Service{repo: repo} }

func (s *Service) CreateCardsForEntry(ctx context.Context, in CreateCardsInput) error {
	dirs := validDirections(in.Directions)
	return s.repo.CreateCardsForEntry(ctx, in.OwnerID, in.EntryID, dirs)
}

func (s *Service) CardStatusesByEntry(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	return s.repo.CardStatusesByEntry(ctx, ownerID, entryIDs)
}

func (s *Service) EntryIDsByStatus(ctx context.Context, ownerID uuid.UUID, status string) ([]uuid.UUID, error) {
	return s.repo.EntryIDsByStatus(ctx, ownerID, status)
}

func (s *Service) BulkCreateForDeck(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (int, error) {
	return s.repo.BulkCreateForDeck(ctx, ownerID, entryIDs)
}

func validDirections(in []string) []string {
	var out []string
	for _, d := range in {
		if domain.Direction(d).Valid() {
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		return []string{string(domain.DirectionFrontBack)}
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scheduling/service/ -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/scheduling/service/
git commit -m "feat(scheduling): card service implementing cross-module card port (AD-9)"
```

---

### Task 8: vocabulary/ports — CardService interface (consumer side)

**Files:**
- Create: `internal/vocabulary/ports/scheduling.go`
- Test: `internal/vocabulary/ports/scheduling_test.go`

- [ ] **Step 1: Write the failing test (compile-time contract check)**

Create `internal/vocabulary/ports/scheduling_test.go`:
```go
package ports_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	schedsvc "github.com/memorix/memorix/internal/scheduling/service"
	"github.com/memorix/memorix/internal/vocabulary/ports"
)

// schedAdapter chứng minh scheduling.Service thỏa CardService (ráp thật ở cmd/api).
type schedAdapter struct{ *schedsvc.Service }

func (a schedAdapter) CreateCardsForEntry(ctx context.Context, in ports.CreateCardsInput) error {
	return a.Service.CreateCardsForEntry(ctx, schedsvc.CreateCardsInput(in))
}

func TestSchedulingServiceSatisfiesPort(t *testing.T) {
	var _ ports.CardService = schedAdapter{}
	_ = uuid.Nil
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/vocabulary/ports/ -v`
Expected: FAIL (undefined `ports.CardService`).

- [ ] **Step 3: Write implementation**

Create `internal/vocabulary/ports/scheduling.go`:
```go
// Package ports expose hợp đồng vocabulary cần từ module khác (AD-1, AD-9).
// CardService định nghĩa ở phía gọi (vocabulary) để tránh import cycle với
// scheduling; scheduling.Service hiện thực, ráp ở cmd/api.
package ports

import (
	"context"

	"github.com/google/uuid"
)

type CreateCardsInput struct {
	OwnerID    uuid.UUID
	EntryID    uuid.UUID
	Directions []string
}

// CardService là cổng vocabulary → scheduling. Vocabulary KHÔNG ghi thẳng
// scheduling.cards; mọi thao tác card đi qua đây (AD-9, AD-10).
type CardService interface {
	CreateCardsForEntry(ctx context.Context, in CreateCardsInput) error
	CardStatusesByEntry(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID]string, error)
	EntryIDsByStatus(ctx context.Context, ownerID uuid.UUID, status string) ([]uuid.UUID, error)
	BulkCreateForDeck(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (int, error)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/vocabulary/ports/ -v`
Expected: PASS (adapter thỏa interface → compile OK).

- [ ] **Step 5: Commit**

```bash
git add internal/vocabulary/ports/
git commit -m "feat(vocabulary): CardService port (consumer-side, avoids import cycle) (AD-9)"
```

---

### Task 9: vocabulary/repo — EntryRepo CRUD + list (container tests)

**Files:**
- Create: `internal/vocabulary/repo/entry_repo.go`
- Test: `internal/vocabulary/repo/entry_repo_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/vocabulary/repo/entry_repo_test.go`:
```go
package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
)

func ptr(u uuid.UUID) *uuid.UUID { return &u }

func TestEntryRepo_InsertFindWithChildren(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	owner := uuid.New()

	e := &domain.Entry{
		OwnerID:      ptr(owner),
		Term:         "ubiquitous",
		PartOfSpeech: "adj",
		Notes:        "note",
		Meanings:     []domain.Meaning{{PartOfSpeech: "adj", Definition: "everywhere", Position: 0}},
		Examples:     []domain.Example{{Text: "It is ubiquitous.", Position: 0}},
		Relations:    []domain.SynAnt{{Relation: domain.RelationSynonym, Value: "omnipresent"}},
	}
	if err := r.Insert(ctx, e); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if e.ID == uuid.Nil {
		t.Fatal("insert did not set ID")
	}
	got, err := r.FindByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Term != "ubiquitous" || len(got.Meanings) != 1 || len(got.Examples) != 1 || len(got.Relations) != 1 {
		t.Errorf("children not loaded: %+v", got)
	}
}

func TestEntryRepo_ExistingID_Normalized(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	owner := uuid.New()

	e := &domain.Entry{OwnerID: ptr(owner), Term: "Café"}
	if err := r.Insert(ctx, e); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// khác dấu/hoa thường vẫn coi là trùng (FR-10).
	id, ok, err := r.ExistingID(ctx, owner, "cafe")
	if err != nil {
		t.Fatalf("existing: %v", err)
	}
	if !ok || id != e.ID {
		t.Errorf("ExistingID = %s, %v; want %s, true", id, ok, e.ID)
	}
	// insert trùng normalized -> ErrDuplicateTerm.
	if err := r.Insert(ctx, &domain.Entry{OwnerID: ptr(owner), Term: "CAFE"}); err != domain.ErrDuplicateTerm {
		t.Errorf("dup insert err = %v, want ErrDuplicateTerm", err)
	}
}

func TestEntryRepo_ListPage_CursorAndSoftDelete(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	owner := uuid.New()
	for _, term := range []string{"alpha", "bravo", "charlie"} {
		if err := r.Insert(ctx, &domain.Entry{OwnerID: ptr(owner), Term: term}); err != nil {
			t.Fatalf("insert %s: %v", term, err)
		}
	}
	page1, err := r.ListPage(ctx, owner, "", httpx.Cursor{}, 2)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 len = %d, want 2", len(page1))
	}
	last := page1[len(page1)-1]
	cur := httpx.Cursor{SortKey: last.CreatedAt.Format(cursorTimeLayout), ID: last.ID.String()}
	page2, err := r.ListPage(ctx, owner, "", cur, 2)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) != 1 {
		t.Errorf("page2 len = %d, want 1", len(page2))
	}

	// soft delete ẩn khỏi list.
	if err := r.SoftDelete(ctx, page1[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	all, _ := r.ListPage(ctx, owner, "", httpx.Cursor{}, 10)
	if len(all) != 2 {
		t.Errorf("after delete len = %d, want 2", len(all))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/vocabulary/repo/ -run TestEntryRepo -v`
Expected: FAIL (undefined `New`/repo methods).

- [ ] **Step 3: Write implementation**

Create `internal/vocabulary/repo/entry_repo.go`:
```go
// Package repo là adapter Postgres của vocabulary (pgx).
package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
)

// cursorTimeLayout dùng cho SortKey của cursor (created_at).
const cursorTimeLayout = time.RFC3339Nano

const pgUniqueViolation = "23505"

type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

// Insert ghi entry + bảng con trong 1 transaction; set e.ID/CreatedAt.
func (r *Repo) Insert(ctx context.Context, e *domain.Entry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`INSERT INTO vocabulary.entries (owner_id, curated_deck_id, term, part_of_speech, notes, source)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, created_at, updated_at`,
		e.OwnerID, e.CuratedDeckID, e.Term, e.PartOfSpeech, e.Notes, e.Source).
		Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if isUnique(err) {
			return domain.ErrDuplicateTerm
		}
		return err
	}
	if err := insertChildren(ctx, tx, e); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertChildren(ctx context.Context, tx pgx.Tx, e *domain.Entry) error {
	for _, m := range e.Meanings {
		if _, err := tx.Exec(ctx,
			`INSERT INTO vocabulary.meanings (entry_id, part_of_speech, definition, position)
			 VALUES ($1,$2,$3,$4)`, e.ID, m.PartOfSpeech, m.Definition, m.Position); err != nil {
			return err
		}
	}
	for _, ex := range e.Examples {
		if _, err := tx.Exec(ctx,
			`INSERT INTO vocabulary.examples (entry_id, text, position) VALUES ($1,$2,$3)`,
			e.ID, ex.Text, ex.Position); err != nil {
			return err
		}
	}
	for _, p := range e.Pronunciations {
		if _, err := tx.Exec(ctx,
			`INSERT INTO vocabulary.pronunciations (entry_id, ipa, dialect, audio_url) VALUES ($1,$2,$3,$4)`,
			e.ID, p.IPA, p.Dialect, p.AudioURL); err != nil {
			return err
		}
	}
	for _, s := range e.Relations {
		if _, err := tx.Exec(ctx,
			`INSERT INTO vocabulary.synonyms_antonyms (entry_id, relation, value) VALUES ($1,$2,$3)`,
			e.ID, string(s.Relation), s.Value); err != nil {
			return err
		}
	}
	return nil
}

// FindByID trả entry + toàn bộ bảng con (cho màn detail).
func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Entry, error) {
	var e domain.Entry
	err := r.pool.QueryRow(ctx,
		`SELECT id, owner_id, curated_deck_id, term, part_of_speech, notes, source,
		        created_at, updated_at, deleted_at
		 FROM vocabulary.entries WHERE id = $1 AND deleted_at IS NULL`, id).
		Scan(&e.ID, &e.OwnerID, &e.CuratedDeckID, &e.Term, &e.PartOfSpeech, &e.Notes, &e.Source,
			&e.CreatedAt, &e.UpdatedAt, &e.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrEntryNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := loadChildren(ctx, r.pool, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

func loadChildren(ctx context.Context, q *pgxpool.Pool, e *domain.Entry) error {
	mrows, err := q.Query(ctx,
		`SELECT id, part_of_speech, definition, position FROM vocabulary.meanings
		 WHERE entry_id = $1 ORDER BY position, id`, e.ID)
	if err != nil {
		return err
	}
	for mrows.Next() {
		var m domain.Meaning
		if err := mrows.Scan(&m.ID, &m.PartOfSpeech, &m.Definition, &m.Position); err != nil {
			mrows.Close()
			return err
		}
		e.Meanings = append(e.Meanings, m)
	}
	mrows.Close()

	exrows, err := q.Query(ctx,
		`SELECT id, text, position FROM vocabulary.examples WHERE entry_id = $1 ORDER BY position, id`, e.ID)
	if err != nil {
		return err
	}
	for exrows.Next() {
		var x domain.Example
		if err := exrows.Scan(&x.ID, &x.Text, &x.Position); err != nil {
			exrows.Close()
			return err
		}
		e.Examples = append(e.Examples, x)
	}
	exrows.Close()

	prows, err := q.Query(ctx,
		`SELECT id, ipa, dialect, audio_url FROM vocabulary.pronunciations WHERE entry_id = $1 ORDER BY id`, e.ID)
	if err != nil {
		return err
	}
	for prows.Next() {
		var p domain.Pronunciation
		if err := prows.Scan(&p.ID, &p.IPA, &p.Dialect, &p.AudioURL); err != nil {
			prows.Close()
			return err
		}
		e.Pronunciations = append(e.Pronunciations, p)
	}
	prows.Close()

	srows, err := q.Query(ctx,
		`SELECT id, relation, value FROM vocabulary.synonyms_antonyms WHERE entry_id = $1 ORDER BY id`, e.ID)
	if err != nil {
		return err
	}
	for srows.Next() {
		var s domain.SynAnt
		var rel string
		if err := srows.Scan(&s.ID, &rel, &s.Value); err != nil {
			srows.Close()
			return err
		}
		s.Relation = domain.Relation(rel)
		e.Relations = append(e.Relations, s)
	}
	srows.Close()
	return srows.Err()
}

// ExistingID tìm entry chưa xóa của owner có term chuẩn hóa trùng (FR-10).
func (r *Repo) ExistingID(ctx context.Context, ownerID uuid.UUID, term string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM vocabulary.entries
		 WHERE owner_id = $1
		   AND term_normalized = vocabulary.immutable_unaccent(lower($2))
		   AND deleted_at IS NULL
		 LIMIT 1`, ownerID, term).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return id, true, nil
}

// Update ghi lại scalar fields + thay toàn bộ bảng con (giữ nguyên card FSRS).
func (r *Repo) Update(ctx context.Context, e *domain.Entry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`UPDATE vocabulary.entries
		 SET term=$2, part_of_speech=$3, notes=$4, source=$5, updated_at=now()
		 WHERE id=$1 AND deleted_at IS NULL`,
		e.ID, e.Term, e.PartOfSpeech, e.Notes, e.Source)
	if err != nil {
		if isUnique(err) {
			return domain.ErrDuplicateTerm
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEntryNotFound
	}
	for _, tbl := range []string{"meanings", "examples", "pronunciations", "synonyms_antonyms"} {
		if _, err := tx.Exec(ctx, "DELETE FROM vocabulary."+tbl+" WHERE entry_id=$1", e.ID); err != nil {
			return err
		}
	}
	if err := insertChildren(ctx, tx, e); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SoftDelete đặt deleted_at (FR-9; card + log giữ tới purge).
func (r *Repo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE vocabulary.entries SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEntryNotFound
	}
	return nil
}

// ListPage phân trang entries của owner theo (created_at DESC, id DESC). q FTS optional.
func (r *Repo) ListPage(ctx context.Context, ownerID uuid.UUID, q string, cur httpx.Cursor, limit int) ([]domain.Entry, error) {
	var curTime *time.Time
	var curID *uuid.UUID
	if cur.ID != "" {
		t, err := time.Parse(cursorTimeLayout, cur.SortKey)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(cur.ID)
		if err != nil {
			return nil, err
		}
		curTime, curID = &t, &id
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, owner_id, curated_deck_id, term, part_of_speech, notes, source, created_at, updated_at
		 FROM vocabulary.entries
		 WHERE owner_id = $1 AND deleted_at IS NULL
		   AND ($2 = '' OR to_tsvector('english', term || ' ' || notes) @@ plainto_tsquery('english', $2))
		   AND ($3::timestamptz IS NULL OR (created_at, id) < ($3, $4))
		 ORDER BY created_at DESC, id DESC
		 LIMIT $5`,
		ownerID, q, curTime, curID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntryList(rows)
}

// ListPageByIDs như ListPage nhưng giới hạn trong tập id (lọc theo status từ scheduling).
func (r *Repo) ListPageByIDs(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID, cur httpx.Cursor, limit int) ([]domain.Entry, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var curTime *time.Time
	var curID *uuid.UUID
	if cur.ID != "" {
		t, err := time.Parse(cursorTimeLayout, cur.SortKey)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(cur.ID)
		if err != nil {
			return nil, err
		}
		curTime, curID = &t, &id
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, owner_id, curated_deck_id, term, part_of_speech, notes, source, created_at, updated_at
		 FROM vocabulary.entries
		 WHERE owner_id = $1 AND deleted_at IS NULL AND id = ANY($2)
		   AND ($3::timestamptz IS NULL OR (created_at, id) < ($3, $4))
		 ORDER BY created_at DESC, id DESC
		 LIMIT $5`,
		ownerID, ids, curTime, curID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntryList(rows)
}

func scanEntryList(rows pgx.Rows) ([]domain.Entry, error) {
	var out []domain.Entry
	for rows.Next() {
		var e domain.Entry
		if err := rows.Scan(&e.ID, &e.OwnerID, &e.CuratedDeckID, &e.Term, &e.PartOfSpeech,
			&e.Notes, &e.Source, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/vocabulary/repo/ -run TestEntryRepo -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/vocabulary/repo/entry_repo.go internal/vocabulary/repo/entry_repo_test.go
git commit -m "feat(vocabulary): entry repo CRUD, normalized dedup, cursor list, soft delete (FR-7/9/10/11)"
```

---

### Task 10: vocabulary/repo — DeckRepo + enrollment (container tests)

**Files:**
- Create: `internal/vocabulary/repo/deck_repo.go`
- Test: `internal/vocabulary/repo/deck_repo_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/vocabulary/repo/deck_repo_test.go`:
```go
package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
	"github.com/memorix/memorix/internal/vocabulary/domain"
)

func seedDeck(ctx context.Context, r *Repo, slug string, entryTerms []string) (uuid.UUID, error) {
	var deckID uuid.UUID
	err := r.pool.QueryRow(ctx,
		`INSERT INTO vocabulary.curated_decks (slug, name) VALUES ($1, $1) RETURNING id`, slug).Scan(&deckID)
	if err != nil {
		return uuid.Nil, err
	}
	for _, term := range entryTerms {
		if _, err := r.pool.Exec(ctx,
			`INSERT INTO vocabulary.entries (owner_id, curated_deck_id, term) VALUES (NULL, $1, $2)`,
			deckID, term); err != nil {
			return uuid.Nil, err
		}
	}
	return deckID, nil
}

func TestDeckRepo_ListActiveAndCuratedEntries(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()

	deckID, err := seedDeck(ctx, r, "test-deck", []string{"one", "two", "three"})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	decks, err := r.ListActiveDecks(ctx)
	if err != nil {
		t.Fatalf("list decks: %v", err)
	}
	// Ít nhất deck vừa seed (migration seed cũng có thể thêm nữa).
	found := false
	for _, d := range decks {
		if d.ID == deckID && d.Slug == "test-deck" {
			found = true
		}
	}
	if !found {
		t.Errorf("seeded deck not in ListActiveDecks: %+v", decks)
	}
	ids, err := r.CuratedEntryIDs(ctx, deckID)
	if err != nil {
		t.Fatalf("curated ids: %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("curated entry count = %d, want 3", len(ids))
	}
}

func TestDeckRepo_EnrollmentUnique(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()
	deckID, err := seedDeck(ctx, r, "enroll-deck", []string{"a"})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	owner := uuid.New()
	if _, err := r.InsertEnrollment(ctx, owner, deckID); err != nil {
		t.Fatalf("first enroll: %v", err)
	}
	if _, err := r.InsertEnrollment(ctx, owner, deckID); err != domain.ErrAlreadyEnrolled {
		t.Errorf("second enroll err = %v, want ErrAlreadyEnrolled", err)
	}
	if err := r.CompleteEnrollment(ctx, owner, deckID, 5); err != nil {
		t.Fatalf("complete: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/vocabulary/repo/ -run TestDeckRepo -v`
Expected: FAIL (undefined deck methods).

- [ ] **Step 3: Write implementation**

Create `internal/vocabulary/repo/deck_repo.go`:
```go
package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/memorix/memorix/internal/vocabulary/domain"
)

// ListActiveDecks trả bộ curated đang bật (onboarding + empty-state).
func (r *Repo) ListActiveDecks(ctx context.Context) ([]domain.CuratedDeck, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, slug, name, description, is_active FROM vocabulary.curated_decks
		 WHERE is_active = true ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.CuratedDeck
	for rows.Next() {
		var d domain.CuratedDeck
		if err := rows.Scan(&d.ID, &d.Slug, &d.Name, &d.Description, &d.IsActive); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// FindDeckByID trả deck theo id.
func (r *Repo) FindDeckByID(ctx context.Context, id uuid.UUID) (domain.CuratedDeck, error) {
	var d domain.CuratedDeck
	err := r.pool.QueryRow(ctx,
		`SELECT id, slug, name, description, is_active FROM vocabulary.curated_decks WHERE id = $1`, id).
		Scan(&d.ID, &d.Slug, &d.Name, &d.Description, &d.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CuratedDeck{}, domain.ErrDeckNotFound
	}
	return d, err
}

// CuratedEntryIDs trả id các entry curated (owner NULL) trong deck.
func (r *Repo) CuratedEntryIDs(ctx context.Context, deckID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id FROM vocabulary.entries
		 WHERE curated_deck_id = $1 AND owner_id IS NULL AND deleted_at IS NULL ORDER BY id`, deckID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// InsertEnrollment tạo bản ghi enroll; trùng (owner, deck) -> ErrAlreadyEnrolled (FR-11b).
func (r *Repo) InsertEnrollment(ctx context.Context, ownerID, deckID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`INSERT INTO vocabulary.deck_enrollments (owner_id, curated_deck_id) VALUES ($1,$2) RETURNING id`,
		ownerID, deckID).Scan(&id)
	if err != nil {
		if isUnique(err) {
			return uuid.Nil, domain.ErrAlreadyEnrolled
		}
		return uuid.Nil, err
	}
	return id, nil
}

// CompleteEnrollment đánh dấu enroll hoàn tất + số card đã tạo (job idempotent).
func (r *Repo) CompleteEnrollment(ctx context.Context, ownerID, deckID uuid.UUID, cardCount int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE vocabulary.deck_enrollments
		 SET status='completed', card_count=$3, completed_at=now()
		 WHERE owner_id=$1 AND curated_deck_id=$2`,
		ownerID, deckID, cardCount)
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/vocabulary/repo/ -run TestDeckRepo -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/vocabulary/repo/deck_repo.go internal/vocabulary/repo/deck_repo_test.go
git commit -m "feat(vocabulary): curated deck repo and idempotent enrollment (FR-11a/11b)"
```

---

### Task 11: vocabulary/service — entry use cases (fakes)

**Files:**
- Create: `internal/vocabulary/service/service.go`
- Create: `internal/vocabulary/service/entry.go`
- Test: `internal/vocabulary/service/entry_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/vocabulary/service/entry_test.go`:
```go
package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
	"github.com/memorix/memorix/internal/vocabulary/ports"
)

// ---- fakes ----

type fakeEntryRepo struct {
	entries  map[uuid.UUID]*domain.Entry
	existing map[string]uuid.UUID // normalized term -> id
}

func newFakeEntryRepo() *fakeEntryRepo {
	return &fakeEntryRepo{entries: map[uuid.UUID]*domain.Entry{}, existing: map[string]uuid.UUID{}}
}
func (f *fakeEntryRepo) Insert(_ context.Context, e *domain.Entry) error {
	if _, ok := f.existing[e.Term]; ok {
		return domain.ErrDuplicateTerm
	}
	e.ID = uuid.New()
	f.entries[e.ID] = e
	f.existing[e.Term] = e.ID
	return nil
}
func (f *fakeEntryRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Entry, error) {
	e, ok := f.entries[id]
	if !ok {
		return nil, domain.ErrEntryNotFound
	}
	return e, nil
}
func (f *fakeEntryRepo) ExistingID(_ context.Context, _ uuid.UUID, term string) (uuid.UUID, bool, error) {
	id, ok := f.existing[term]
	return id, ok, nil
}
func (f *fakeEntryRepo) Update(_ context.Context, e *domain.Entry) error {
	if _, ok := f.entries[e.ID]; !ok {
		return domain.ErrEntryNotFound
	}
	f.entries[e.ID] = e
	return nil
}
func (f *fakeEntryRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.entries[id]; !ok {
		return domain.ErrEntryNotFound
	}
	delete(f.entries, id)
	return nil
}
func (f *fakeEntryRepo) ListPage(_ context.Context, _ uuid.UUID, _ string, _ httpx.Cursor, _ int) ([]domain.Entry, error) {
	var out []domain.Entry
	for _, e := range f.entries {
		out = append(out, *e)
	}
	return out, nil
}
func (f *fakeEntryRepo) ListPageByIDs(_ context.Context, _ uuid.UUID, ids []uuid.UUID, _ httpx.Cursor, _ int) ([]domain.Entry, error) {
	var out []domain.Entry
	for _, id := range ids {
		if e, ok := f.entries[id]; ok {
			out = append(out, *e)
		}
	}
	return out, nil
}

type fakeCards struct {
	created  []ports.CreateCancelHelper
	statuses map[uuid.UUID]string
}

// CreateCancelHelper để test bắt input (đặt tên rõ để tránh nhầm với DTO thật).
type _ = ports.CreateCardsInput

func (f *fakeCards) CreateCardsForEntry(_ context.Context, in ports.CreateCardsInput) error {
	f.created = append(f.created, ports.CreateCancelHelper{OwnerID: in.OwnerID, EntryID: in.EntryID, Directions: in.Directions})
	return nil
}
func (f *fakeCards) CardStatusesByEntry(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if s, ok := f.statuses[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}
func (f *fakeCards) EntryIDsByStatus(_ context.Context, _ uuid.UUID, _ string) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *fakeCards) BulkCreateForDeck(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (int, error) {
	return len(ids), nil
}

// ---- tests ----

func TestCreate_TermOnly_CreatesEntryAndCard(t *testing.T) {
	repo := newFakeEntryRepo()
	cards := &fakeCards{statuses: map[uuid.UUID]string{}}
	svc := New(repo, nil, cards, nil)
	owner := uuid.New()

	e, err := svc.Create(context.Background(), owner, CreateEntryInput{Term: "  Hello  "})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if e.Term != "Hello" {
		t.Errorf("term = %q, want trimmed Hello", e.Term)
	}
	if len(cards.created) != 1 || cards.created[0].EntryID != e.ID {
		t.Fatalf("card not created for entry: %+v", cards.created)
	}
	if got := cards.created[0].Directions; len(got) != 1 || got[0] != "front_back" {
		t.Errorf("default direction = %v, want [front_back]", got)
	}
}

func TestCreate_BlankTerm_Rejected(t *testing.T) {
	svc := New(newFakeEntryRepo(), nil, &fakeCards{}, nil)
	if _, err := svc.Create(context.Background(), uuid.New(), CreateEntryInput{Term: "   "}); err != domain.ErrTermRequired {
		t.Errorf("blank term err = %v, want ErrTermRequired", err)
	}
}

func TestCreate_Duplicate_ReturnsDuplicateError(t *testing.T) {
	repo := newFakeEntryRepo()
	existingID := uuid.New()
	repo.existing["dup"] = existingID
	svc := New(repo, nil, &fakeCards{}, nil)

	_, err := svc.Create(context.Background(), uuid.New(), CreateEntryInput{Term: "dup"})
	var de DuplicateError
	if !asDuplicate(err, &de) || de.ExistingID != existingID {
		t.Errorf("err = %v, want DuplicateError{%s}", err, existingID)
	}
}

func TestCreate_EscapesHTML(t *testing.T) {
	repo := newFakeEntryRepo()
	svc := New(repo, nil, &fakeCards{statuses: map[uuid.UUID]string{}}, nil)
	e, err := svc.Create(context.Background(), uuid.New(), CreateEntryInput{
		Term:  "safe",
		Notes: "<script>alert(1)</script>",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if e.Notes == "<script>alert(1)</script>" {
		t.Errorf("notes not escaped: %q", e.Notes)
	}
}

func TestGet_OwnershipEnforced(t *testing.T) {
	repo := newFakeEntryRepo()
	cards := &fakeCards{statuses: map[uuid.UUID]string{}}
	svc := New(repo, nil, cards, nil)
	owner := uuid.New()
	e, _ := svc.Create(context.Background(), owner, CreateEntryInput{Term: "mine"})

	if _, err := svc.Get(context.Background(), uuid.New(), e.ID); err != domain.ErrEntryNotFound {
		t.Errorf("other user Get err = %v, want ErrEntryNotFound (deny-by-default)", err)
	}
	view, err := svc.Get(context.Background(), owner, e.ID)
	if err != nil {
		t.Fatalf("owner Get: %v", err)
	}
	if view.Entry.ID != e.ID {
		t.Errorf("wrong entry returned")
	}
}
```

Helper for DuplicateError assertion — add to same file:
```go
func asDuplicate(err error, target *DuplicateError) bool {
	d, ok := err.(DuplicateError)
	if ok {
		*target = d
	}
	return ok
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/vocabulary/service/ -v`
Expected: FAIL (undefined `New`/`CreateEntryInput`/`DuplicateError` và `ports.CreateCancelHelper`).

> Lưu ý: `ports.CreateCancelHelper` chỉ là alias tiện lợi cho test; ta thêm nó vào `ports` ở Step 3 (nó = `CreateCardsInput`). Nếu muốn tối giản, thay `ports.CreateCancelHelper` bằng `ports.CreateCardsInput` trong test — chúng đồng shape.

- [ ] **Step 3: Write implementation**

Append alias to `internal/vocabulary/ports/scheduling.go`:
```go
// CreateCancelHelper là alias của CreateCardsInput (tiện cho test bắt input).
type CreateCancelHelper = CreateCardsInput
```

Create `internal/vocabulary/service/service.go`:
```go
// Package service là use case của vocabulary (light hexagonal).
package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
	"github.com/memorix/memorix/internal/vocabulary/ports"
)

// EntryRepo là cổng lưu trữ entry (repo implements).
type EntryRepo interface {
	Insert(ctx context.Context, e *domain.Entry) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Entry, error)
	ExistingID(ctx context.Context, ownerID uuid.UUID, term string) (uuid.UUID, bool, error)
	Update(ctx context.Context, e *domain.Entry) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListPage(ctx context.Context, ownerID uuid.UUID, q string, cur httpx.Cursor, limit int) ([]domain.Entry, error)
	ListPageByIDs(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID, cur httpx.Cursor, limit int) ([]domain.Entry, error)
}

// DeckRepo là cổng lưu trữ curated deck + enrollment.
type DeckRepo interface {
	ListActiveDecks(ctx context.Context) ([]domain.CuratedDeck, error)
	FindDeckByID(ctx context.Context, id uuid.UUID) (domain.CuratedDeck, error)
	CuratedEntryIDs(ctx context.Context, deckID uuid.UUID) ([]uuid.UUID, error)
	InsertEnrollment(ctx context.Context, ownerID, deckID uuid.UUID) (uuid.UUID, error)
	CompleteEnrollment(ctx context.Context, ownerID, deckID uuid.UUID, cardCount int) error
}

// EnrollEnqueuer đẩy job enroll (River adapter implements).
type EnrollEnqueuer interface {
	EnqueueEnroll(ctx context.Context, ownerID, deckID uuid.UUID) error
}

type Service struct {
	entries EntryRepo
	decks   DeckRepo
	cards   ports.CardService
	jobs    EnrollEnqueuer
}

func New(entries EntryRepo, decks DeckRepo, cards ports.CardService, jobs EnrollEnqueuer) *Service {
	return &Service{entries: entries, decks: decks, cards: cards, jobs: jobs}
}
```

Create `internal/vocabulary/service/entry.go`:
```go
package service

import (
	"context"
	"html"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
	"github.com/memorix/memorix/internal/vocabulary/ports"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// DuplicateError mang id entry hiện có để client mở lại (FR-10).
type DuplicateError struct{ ExistingID uuid.UUID }

func (e DuplicateError) Error() string { return "duplicate term" }

type MeaningInput struct {
	PartOfSpeech string
	Definition   string
}
type PronunciationInput struct {
	IPA      string
	Dialect  string
	AudioURL string
}

type CreateEntryInput struct {
	Term           string
	PartOfSpeech   string
	Notes          string
	Source         string
	Directions     []string
	Meanings       []MeaningInput
	Examples       []string
	Pronunciations []PronunciationInput
	Synonyms       []string
	Antonyms       []string
}

type UpdateEntryInput struct {
	Term           string
	PartOfSpeech   string
	Notes          string
	Source         string
	Meanings       []MeaningInput
	Examples       []string
	Pronunciations []PronunciationInput
	Synonyms       []string
	Antonyms       []string
}

// EntryView = entry + status card primary (từ scheduling).
type EntryView struct {
	Entry  domain.Entry
	Status string
}

type ListInput struct {
	OwnerID uuid.UUID
	Status  string
	Query   string
	Cursor  string
	Limit   int
}

type EntryListItem struct {
	Entry  domain.Entry
	Status string
}

type ListResult struct {
	Items []EntryListItem
	Page  httpx.Page
}

// Create tạo entry (term-only <10s) + tự tạo card New qua scheduling (FR-7, FR-8).
func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, in CreateEntryInput) (domain.Entry, error) {
	term, err := domain.ValidateTerm(in.Term)
	if err != nil {
		return domain.Entry{}, err
	}
	// Cảnh báo trùng: trả DuplicateError kèm id hiện có (FR-10).
	if id, ok, err := s.entries.ExistingID(ctx, ownerID, term); err != nil {
		return domain.Entry{}, err
	} else if ok {
		return domain.Entry{}, DuplicateError{ExistingID: id}
	}

	e := buildEntry(&ownerID, term, in)
	if err := s.entries.Insert(ctx, &e); err != nil {
		if err == domain.ErrDuplicateTerm {
			// Race: đọc lại id hiện có.
			if id, ok, _ := s.entries.ExistingID(ctx, ownerID, term); ok {
				return domain.Entry{}, DuplicateError{ExistingID: id}
			}
		}
		return domain.Entry{}, err
	}
	// Cross-module (AD-9): nhờ scheduling tạo card New. Không ghi thẳng cards.
	dirs := validDirs(in.Directions)
	if err := s.cards.CreateCardsForEntry(ctx, ports.CreateCardsInput{
		OwnerID: ownerID, EntryID: e.ID, Directions: dirs,
	}); err != nil {
		return domain.Entry{}, err
	}
	return e, nil
}

func buildEntry(owner *uuid.UUID, term string, in CreateEntryInput) domain.Entry {
	e := domain.Entry{
		OwnerID:      owner,
		Term:         term,
		PartOfSpeech: in.PartOfSpeech,
		Notes:        html.EscapeString(in.Notes),
		Source:       in.Source,
	}
	for i, m := range in.Meanings {
		e.Meanings = append(e.Meanings, domain.Meaning{
			PartOfSpeech: m.PartOfSpeech, Definition: html.EscapeString(m.Definition), Position: i,
		})
	}
	for i, x := range in.Examples {
		e.Examples = append(e.Examples, domain.Example{Text: html.EscapeString(x), Position: i})
	}
	for _, p := range in.Pronunciations {
		e.Pronunciations = append(e.Pronunciations, domain.Pronunciation{IPA: p.IPA, Dialect: p.Dialect, AudioURL: p.AudioURL})
	}
	for _, sy := range in.Synonyms {
		e.Relations = append(e.Relations, domain.SynAnt{Relation: domain.RelationSynonym, Value: html.EscapeString(sy)})
	}
	for _, an := range in.Antonyms {
		e.Relations = append(e.Relations, domain.SynAnt{Relation: domain.RelationAntonym, Value: html.EscapeString(an)})
	}
	return e
}

func validDirs(in []string) []string {
	var ds []domain.Direction
	for _, d := range in {
		ds = append(ds, domain.Direction(d))
	}
	valid := domain.DefaultDirections(ds)
	out := make([]string, len(valid))
	for i, d := range valid {
		out[i] = string(d)
	}
	return out
}

// Get trả entry của owner + status card (ownership deny-by-default → 404).
func (s *Service) Get(ctx context.Context, ownerID, id uuid.UUID) (EntryView, error) {
	e, err := s.entries.FindByID(ctx, id)
	if err != nil {
		return EntryView{}, err
	}
	if e.OwnerID == nil || *e.OwnerID != ownerID {
		return EntryView{}, domain.ErrEntryNotFound
	}
	statuses, err := s.cards.CardStatusesByEntry(ctx, ownerID, []uuid.UUID{id})
	if err != nil {
		return EntryView{}, err
	}
	return EntryView{Entry: *e, Status: statuses[id]}, nil
}

// Update sửa entry của owner, giữ nguyên card FSRS (FR-9); dup vẫn 409.
func (s *Service) Update(ctx context.Context, ownerID, id uuid.UUID, in UpdateEntryInput) (domain.Entry, error) {
	e, err := s.entries.FindByID(ctx, id)
	if err != nil {
		return domain.Entry{}, err
	}
	if e.OwnerID == nil || *e.OwnerID != ownerID {
		return domain.Entry{}, domain.ErrEntryNotFound
	}
	term, err := domain.ValidateTerm(in.Term)
	if err != nil {
		return domain.Entry{}, err
	}
	if existID, ok, err := s.entries.ExistingID(ctx, ownerID, term); err != nil {
		return domain.Entry{}, err
	} else if ok && existID != id {
		return domain.Entry{}, DuplicateError{ExistingID: existID}
	}
	updated := buildEntry(&ownerID, term, CreateEntryInput{
		PartOfSpeech: in.PartOfSpeech, Notes: in.Notes, Source: in.Source,
		Meanings: in.Meanings, Examples: in.Examples, Pronunciations: in.Pronunciations,
		Synonyms: in.Synonyms, Antonyms: in.Antonyms,
	})
	updated.ID = id
	if err := s.entries.Update(ctx, &updated); err != nil {
		if err == domain.ErrDuplicateTerm {
			return domain.Entry{}, DuplicateError{ExistingID: id}
		}
		return domain.Entry{}, err
	}
	return updated, nil
}

// Delete soft-delete entry của owner (FR-9).
func (s *Service) Delete(ctx context.Context, ownerID, id uuid.UUID) error {
	e, err := s.entries.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if e.OwnerID == nil || *e.OwnerID != ownerID {
		return domain.ErrEntryNotFound
	}
	return s.entries.SoftDelete(ctx, id)
}

// List phân trang + lọc status (status batch-load qua port, không join chéo — AD-9).
func (s *Service) List(ctx context.Context, in ListInput) (ListResult, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	cur, err := httpx.DecodeCursor(in.Cursor)
	if err != nil {
		return ListResult{}, err
	}

	var entries []domain.Entry
	if in.Status != "" {
		ids, err := s.cards.EntryIDsByStatus(ctx, in.OwnerID, in.Status)
		if err != nil {
			return ListResult{}, err
		}
		if len(ids) == 0 {
			return ListResult{Items: nil, Page: httpx.Page{Limit: limit}}, nil
		}
		entries, err = s.entries.ListPageByIDs(ctx, in.OwnerID, ids, cur, limit+1)
		if err != nil {
			return ListResult{}, err
		}
	} else {
		entries, err = s.entries.ListPage(ctx, in.OwnerID, in.Query, cur, limit+1)
		if err != nil {
			return ListResult{}, err
		}
	}

	page := httpx.Page{Limit: limit}
	if len(entries) > limit {
		entries = entries[:limit]
		page.HasMore = true
		last := entries[len(entries)-1]
		page.NextCursor = httpx.Cursor{SortKey: last.CreatedAt.UTC().Format(cursorLayout), ID: last.ID.String()}.Encode()
	}

	entryIDs := make([]uuid.UUID, len(entries))
	for i, e := range entries {
		entryIDs[i] = e.ID
	}
	statuses, err := s.cards.CardStatusesByEntry(ctx, in.OwnerID, entryIDs)
	if err != nil {
		return ListResult{}, err
	}
	items := make([]EntryListItem, len(entries))
	for i, e := range entries {
		items[i] = EntryListItem{Entry: e, Status: statuses[e.ID]}
	}
	return ListResult{Items: items, Page: page}, nil
}
```

Add the shared cursor layout constant in `entry.go` (must match repo's `cursorTimeLayout`):
```go
// cursorLayout phải trùng repo.cursorTimeLayout (RFC3339Nano).
const cursorLayout = "2006-01-02T15:04:05.999999999Z07:00"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/vocabulary/service/ -v`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/vocabulary/service/ internal/vocabulary/ports/scheduling.go
git commit -m "feat(vocabulary): entry service (create+card, dedup, edit, delete, list) (FR-7/8/9/10/11)"
```

---

### Task 12: vocabulary/service — enroll + curated decks (fakes)

**Files:**
- Create: `internal/vocabulary/service/enroll.go`
- Test: `internal/vocabulary/service/enroll_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/vocabulary/service/enroll_test.go`:
```go
package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/vocabulary/domain"
)

type fakeDeckRepo struct {
	decks       []domain.CuratedDeck
	enrolled    map[uuid.UUID]bool // deckID -> enrolled
	enrollErr   error
}

func (f *fakeDeckRepo) ListActiveDecks(context.Context) ([]domain.CuratedDeck, error) {
	return f.decks, nil
}
func (f *fakeDeckRepo) FindDeckByID(_ context.Context, id uuid.UUID) (domain.CuratedDeck, error) {
	for _, d := range f.decks {
		if d.ID == id {
			return d, nil
		}
	}
	return domain.CuratedDeck{}, domain.ErrDeckNotFound
}
func (f *fakeDeckRepo) CuratedEntryIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *fakeDeckRepo) InsertEnrollment(_ context.Context, _, deckID uuid.UUID) (uuid.UUID, error) {
	if f.enrollErr != nil {
		return uuid.Nil, f.enrollErr
	}
	if f.enrolled[deckID] {
		return uuid.Nil, domain.ErrAlreadyEnrolled
	}
	f.enrolled[deckID] = true
	return uuid.New(), nil
}
func (f *fakeDeckRepo) CompleteEnrollment(context.Context, uuid.UUID, uuid.UUID, int) error {
	return nil
}

type fakeEnqueuer struct{ calls int }

func (f *fakeEnqueuer) EnqueueEnroll(context.Context, uuid.UUID, uuid.UUID) error {
	f.calls++
	return nil
}

func TestEnroll_CreatesEnrollmentAndEnqueues(t *testing.T) {
	deck := domain.CuratedDeck{ID: uuid.New(), Slug: "ielts-starter", Name: "IELTS", IsActive: true}
	decks := &fakeDeckRepo{decks: []domain.CuratedDeck{deck}, enrolled: map[uuid.UUID]bool{}}
	q := &fakeEnqueuer{}
	svc := New(newFakeEntryRepo(), decks, &fakeCards{}, q)

	if _, err := svc.Enroll(context.Background(), uuid.New(), deck.ID); err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if q.calls != 1 {
		t.Errorf("enqueue calls = %d, want 1", q.calls)
	}
}

func TestEnroll_AlreadyEnrolled(t *testing.T) {
	deck := domain.CuratedDeck{ID: uuid.New(), IsActive: true}
	decks := &fakeDeckRepo{decks: []domain.CuratedDeck{deck}, enrolled: map[uuid.UUID]bool{deck.ID: true}}
	q := &fakeEnqueuer{}
	svc := New(newFakeEntryRepo(), decks, &fakeCards{}, q)

	if _, err := svc.Enroll(context.Background(), uuid.New(), deck.ID); err != domain.ErrAlreadyEnrolled {
		t.Errorf("err = %v, want ErrAlreadyEnrolled", err)
	}
	if q.calls != 0 {
		t.Errorf("should not enqueue on duplicate enroll")
	}
}

func TestEnroll_UnknownDeck(t *testing.T) {
	decks := &fakeDeckRepo{decks: nil, enrolled: map[uuid.UUID]bool{}}
	svc := New(newFakeEntryRepo(), decks, &fakeCards{}, &fakeEnqueuer{})
	if _, err := svc.Enroll(context.Background(), uuid.New(), uuid.New()); err != domain.ErrDeckNotFound {
		t.Errorf("err = %v, want ErrDeckNotFound", err)
	}
}

func TestListCuratedDecks(t *testing.T) {
	deck := domain.CuratedDeck{ID: uuid.New(), Slug: "ielts-starter", IsActive: true}
	svc := New(newFakeEntryRepo(), &fakeDeckRepo{decks: []domain.CuratedDeck{deck}, enrolled: map[uuid.UUID]bool{}}, &fakeCards{}, &fakeEnqueuer{})
	got, err := svc.ListCuratedDecks(context.Background())
	if err != nil || len(got) != 1 || got[0].Slug != "ielts-starter" {
		t.Errorf("ListCuratedDecks = %+v, %v", got, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/vocabulary/service/ -run TestEnroll -v`
Expected: FAIL (undefined `Enroll`/`ListCuratedDecks`).

- [ ] **Step 3: Write implementation**

Create `internal/vocabulary/service/enroll.go`:
```go
package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/vocabulary/domain"
)

// ListCuratedDecks trả bộ khởi đầu (onboarding gợi ý + empty-state) (FR-11a/11c).
func (s *Service) ListCuratedDecks(ctx context.Context) ([]domain.CuratedDeck, error) {
	return s.decks.ListActiveDecks(ctx)
}

// Enroll tạo enrollment (409 nếu đã có) rồi đẩy job bulk-create card New (FR-11b).
// Trả enrollmentID; việc tạo card chạy nền idempotent.
func (s *Service) Enroll(ctx context.Context, ownerID, deckID uuid.UUID) (uuid.UUID, error) {
	if _, err := s.decks.FindDeckByID(ctx, deckID); err != nil {
		return uuid.Nil, err
	}
	enrollmentID, err := s.decks.InsertEnrollment(ctx, ownerID, deckID)
	if err != nil {
		return uuid.Nil, err // ErrAlreadyEnrolled → 409
	}
	if err := s.jobs.EnqueueEnroll(ctx, ownerID, deckID); err != nil {
		return uuid.Nil, err
	}
	return enrollmentID, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/vocabulary/service/ -run 'TestEnroll|TestListCurated' -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/vocabulary/service/enroll.go internal/vocabulary/service/enroll_test.go
git commit -m "feat(vocabulary): enroll starter deck (idempotent, 409) + list curated decks (FR-11a/11b/11c)"
```

---

### Task 13: platform/jobs — River client + migrate

**Files:**
- Create: `internal/platform/jobs/jobs.go`
- Test: `internal/platform/jobs/jobs_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/platform/jobs/jobs_test.go`:
```go
package jobs

import (
	"context"
	"testing"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
)

func TestMigrate_CreatesRiverTables(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	ctx := context.Background()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("river migrate: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_name = 'river_job'`).Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 1 {
		t.Errorf("river_job table missing (n=%d)", n)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/platform/jobs/ -v`
Expected: FAIL (undefined `Migrate`).

- [ ] **Step 3: Write implementation**

Create `internal/platform/jobs/jobs.go`:
```go
// Package jobs bọc River (client + migrate). River tables migrate riêng khỏi
// golang-migrate để River tự quản version schema của nó (ARCH-12).
package jobs

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// Migrate áp schema River lên DB.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	m, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return err
	}
	_, err = m.Migrate(ctx, rivermigrate.DirectionUp, nil)
	return err
}

// NewClient tạo River client (insert-only ở API; worker truyền Workers riêng).
func NewClient(pool *pgxpool.Pool, workers *river.Workers) (*river.Client[pgx.Tx], error) {
	cfg := &river.Config{}
	if workers != nil {
		cfg.Queues = map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 10}}
		cfg.Workers = workers
	}
	return river.NewClient(riverpgxv5.New(pool), cfg)
}
```

Add missing pgx import — update the import block of `internal/platform/jobs/jobs.go`:
```go
import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)
```

- [ ] **Step 4: Add deps + run test**

Run:
```bash
go get github.com/riverqueue/river/rivermigrate@latest
go test ./internal/platform/jobs/ -v
```
Expected: PASS (`river_job` bảng tồn tại).

- [ ] **Step 5: Commit**

```bash
git add internal/platform/jobs/ go.mod go.sum
git commit -m "feat(platform): River client factory and migrate helper (ARCH-12)"
```

---

### Task 14: vocabulary/jobs — enroll worker + enqueuer

**Files:**
- Create: `internal/vocabulary/jobs/enroll.go`
- Test: `internal/vocabulary/jobs/enroll_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/vocabulary/jobs/enroll_test.go`:
```go
package jobs

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type fakeStore struct {
	entryIDs []uuid.UUID
	done     int
}

func (f *fakeStore) CuratedEntryIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return f.entryIDs, nil
}
func (f *fakeStore) CompleteEnrollment(_ context.Context, _, _ uuid.UUID, cardCount int) error {
	f.done = cardCount
	return nil
}

type fakeBulk struct{ created int }

func (f *fakeBulk) BulkCreateForDeck(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (int, error) {
	f.created += len(ids)
	return len(ids), nil
}

func TestEnrollWorker_BulkCreatesAndCompletes(t *testing.T) {
	store := &fakeStore{entryIDs: []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}}
	bulk := &fakeBulk{}
	w := &EnrollWorker{Store: store, Cards: bulk}

	job := &river.Job[EnrollDeckArgs]{Args: EnrollDeckArgs{OwnerID: uuid.New(), DeckID: uuid.New()}}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("work: %v", err)
	}
	if bulk.created != 3 {
		t.Errorf("bulk created = %d, want 3", bulk.created)
	}
	if store.done != 3 {
		t.Errorf("CompleteEnrollment card_count = %d, want 3", store.done)
	}
}

func TestEnrollDeckArgs_Kind(t *testing.T) {
	if EnrollDeckArgs{}.Kind() != "enroll_deck" {
		t.Errorf("kind = %q", EnrollDeckArgs{}.Kind())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/vocabulary/jobs/ -v`
Expected: FAIL (undefined `EnrollWorker`/`EnrollDeckArgs`).

- [ ] **Step 3: Write implementation**

Create `internal/vocabulary/jobs/enroll.go`:
```go
// Package jobs chứa River worker của vocabulary (adapter tầng job).
// Orchestrate: đọc curated entry (vocabulary) + nhờ scheduling bulk-create card.
package jobs

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

// EnrollDeckArgs là payload job enroll (River serialize JSON).
type EnrollDeckArgs struct {
	OwnerID uuid.UUID `json:"owner_id"`
	DeckID  uuid.UUID `json:"deck_id"`
}

func (EnrollDeckArgs) Kind() string { return "enroll_deck" }

// EnrollStore là phần vocabulary mà worker cần.
type EnrollStore interface {
	CuratedEntryIDs(ctx context.Context, deckID uuid.UUID) ([]uuid.UUID, error)
	CompleteEnrollment(ctx context.Context, ownerID, deckID uuid.UUID, cardCount int) error
}

// BulkCardCreator là phần scheduling mà worker cần (AD-9).
type BulkCardCreator interface {
	BulkCreateForDeck(ctx context.Context, ownerID uuid.UUID, entryIDs []uuid.UUID) (int, error)
}

type EnrollWorker struct {
	river.WorkerDefaults[EnrollDeckArgs]
	Store EnrollStore
	Cards BulkCardCreator
}

// Work bulk-create card New cho toàn bộ entry curated của deck; idempotent
// (BulkCreateForDeck ON CONFLICT DO NOTHING) nên retry an toàn (FR-11b).
func (w *EnrollWorker) Work(ctx context.Context, job *river.Job[EnrollDeckArgs]) error {
	entryIDs, err := w.Store.CuratedEntryIDs(ctx, job.Args.DeckID)
	if err != nil {
		return err
	}
	created, err := w.Cards.BulkCreateForDeck(ctx, job.Args.OwnerID, entryIDs)
	if err != nil {
		return err
	}
	return w.Store.CompleteEnrollment(ctx, job.Args.OwnerID, job.Args.DeckID, created)
}

// Enqueuer đẩy job enroll (service.EnrollEnqueuer). Bọc River client.
type Enqueuer struct {
	Client *river.Client[pgx.Tx]
}

func (e *Enqueuer) EnqueueEnroll(ctx context.Context, ownerID, deckID uuid.UUID) error {
	_, err := e.Client.Insert(ctx, EnrollDeckArgs{OwnerID: ownerID, DeckID: deckID}, nil)
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/vocabulary/jobs/ -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/vocabulary/jobs/
git commit -m "feat(vocabulary): enroll River worker (idempotent bulk card create) + enqueuer (FR-11b, AD-9)"
```

---

### Task 15: vocabulary/handler — Gin routes (httptest)

**Files:**
- Create: `internal/vocabulary/handler/handler.go`
- Create: `internal/vocabulary/handler/routes.go`
- Test: `internal/vocabulary/handler/handler_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/vocabulary/handler/handler_test.go`:
```go
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/vocabulary/domain"
	"github.com/memorix/memorix/internal/vocabulary/service"
)

type fakeSvc struct {
	created domain.Entry
	dupID   uuid.UUID
	dup     bool
}

func (f *fakeSvc) Create(_ context.Context, owner uuid.UUID, in service.CreateEntryInput) (domain.Entry, error) {
	if f.dup {
		return domain.Entry{}, service.DuplicateError{ExistingID: f.dupID}
	}
	if in.Term == "" {
		return domain.Entry{}, domain.ErrTermRequired
	}
	id := uuid.New()
	f.created = domain.Entry{ID: id, OwnerID: &owner, Term: in.Term}
	return f.created, nil
}
func (f *fakeSvc) Get(_ context.Context, _ , id uuid.UUID) (service.EntryView, error) {
	return service.EntryView{Entry: domain.Entry{ID: id, Term: "x"}, Status: "new"}, nil
}
func (f *fakeSvc) Update(_ context.Context, owner, id uuid.UUID, _ service.UpdateEntryInput) (domain.Entry, error) {
	return domain.Entry{ID: id, OwnerID: &owner, Term: "updated"}, nil
}
func (f *fakeSvc) Delete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (f *fakeSvc) List(context.Context, service.ListInput) (service.ListResult, error) {
	return service.ListResult{}, nil
}
func (f *fakeSvc) ListCuratedDecks(context.Context) ([]domain.CuratedDeck, error) {
	return []domain.CuratedDeck{{ID: uuid.New(), Slug: "ielts-starter", Name: "IELTS"}}, nil
}
func (f *fakeSvc) Enroll(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	return uuid.New(), nil
}

func setup(svc VocabService, owner uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := New(svc, func(*gin.Context) (uuid.UUID, error) { return owner, nil })
	RegisterRoutes(r.Group("/api/v1"), h)
	return r
}

func TestCreateEntry_201(t *testing.T) {
	owner := uuid.New()
	r := setup(&fakeSvc{}, owner)
	body, _ := json.Marshal(map[string]any{"term": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vocabulary/entries", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
}

func TestCreateEntry_Duplicate_409(t *testing.T) {
	dupID := uuid.New()
	r := setup(&fakeSvc{dup: true, dupID: dupID}, uuid.New())
	body, _ := json.Marshal(map[string]any{"term": "dup"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vocabulary/entries", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 409 {
		t.Fatalf("status = %d, want 409", w.Code)
	}
	var resp map[string]map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"]["code"] != "CONFLICT" {
		t.Errorf("code = %v, want CONFLICT", resp["error"]["code"])
	}
	fields, _ := resp["error"]["fields"].(map[string]any)
	if fields["existing_id"] != dupID.String() {
		t.Errorf("existing_id = %v, want %s", fields["existing_id"], dupID)
	}
}

func TestCreateEntry_BlankTerm_400(t *testing.T) {
	r := setup(&fakeSvc{}, uuid.New())
	body, _ := json.Marshal(map[string]any{"term": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vocabulary/entries", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestListCuratedDecks_200(t *testing.T) {
	r := setup(&fakeSvc{}, uuid.New())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vocabulary/curated-decks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestEnroll_202(t *testing.T) {
	r := setup(&fakeSvc{}, uuid.New())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vocabulary/curated-decks/"+uuid.New().String()+"/enroll", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 202 {
		t.Errorf("status = %d, want 202", w.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/vocabulary/handler/ -v`
Expected: FAIL (undefined `New`/`RegisterRoutes`/`VocabService`).

- [ ] **Step 3: Write implementation**

Create `internal/vocabulary/handler/handler.go`:
```go
// Package handler là adapter Gin của vocabulary (bind/validate → service).
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
	"github.com/memorix/memorix/internal/vocabulary/service"
)

// VocabService là mặt service handler cần (interface để test bằng fake).
type VocabService interface {
	Create(ctx interface{ Deadline() (t0 anyTime, ok bool) }, owner uuid.UUID, in service.CreateEntryInput) (domain.Entry, error)
}
```

> **Điều chỉnh:** interface trên sai kiểu context. Thay toàn bộ `handler.go` bằng bản đúng dưới đây (dùng `context.Context`).

Create `internal/vocabulary/handler/handler.go` (bản đúng, ghi đè):
```go
// Package handler là adapter Gin của vocabulary (bind/validate → service).
package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
	"github.com/memorix/memorix/internal/vocabulary/service"
)

// VocabService là mặt service handler cần (interface để test bằng fake).
type VocabService interface {
	Create(ctx context.Context, owner uuid.UUID, in service.CreateEntryInput) (domain.Entry, error)
	Get(ctx context.Context, owner, id uuid.UUID) (service.EntryView, error)
	Update(ctx context.Context, owner, id uuid.UUID, in service.UpdateEntryInput) (domain.Entry, error)
	Delete(ctx context.Context, owner, id uuid.UUID) error
	List(ctx context.Context, in service.ListInput) (service.ListResult, error)
	ListCuratedDecks(ctx context.Context) ([]domain.CuratedDeck, error)
	Enroll(ctx context.Context, owner, deckID uuid.UUID) (uuid.UUID, error)
}

// PrincipalFunc lấy userID từ context (bọc authmw ở wiring; stub trong test).
type PrincipalFunc func(*gin.Context) (uuid.UUID, error)

type Handler struct {
	svc       VocabService
	principal PrincipalFunc
}

func New(svc VocabService, p PrincipalFunc) *Handler {
	return &Handler{svc: svc, principal: p}
}

func (h *Handler) owner(c *gin.Context) (uuid.UUID, bool) {
	id, err := h.principal(c)
	if err != nil {
		writeErr(c, httpx.NewError(httpx.CodeUnauthenticated, "authentication required"))
		return uuid.Nil, false
	}
	return id, true
}

func writeErr(c *gin.Context, e *httpx.APIError) {
	e.WithTrace(c.GetHeader("X-Trace-Id"))
	c.JSON(e.HTTPStatus(), e)
}

// mapErr chuyển lỗi service/domain sang envelope chuẩn (AD-14).
func mapErr(c *gin.Context, err error) {
	var dup service.DuplicateError
	switch {
	case errors.As(err, &dup):
		writeErr(c, httpx.NewError(httpx.CodeConflict, "từ đã tồn tại").
			WithField("existing_id", dup.ExistingID.String()))
	case errors.Is(err, domain.ErrTermRequired):
		writeErr(c, httpx.NewError(httpx.CodeValidation, "term bắt buộc").WithField("term", "bắt buộc"))
	case errors.Is(err, domain.ErrEntryNotFound):
		writeErr(c, httpx.NewError(httpx.CodeNotFound, "không tìm thấy từ"))
	case errors.Is(err, domain.ErrDeckNotFound):
		writeErr(c, httpx.NewError(httpx.CodeNotFound, "không tìm thấy bộ thẻ"))
	case errors.Is(err, domain.ErrAlreadyEnrolled):
		writeErr(c, httpx.NewError(httpx.CodeConflict, "đã enroll bộ này"))
	default:
		writeErr(c, httpx.NewError(httpx.CodeInternal, "lỗi hệ thống"))
	}
}

func parseID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		writeErr(c, httpx.NewError(httpx.CodeValidation, "id không hợp lệ").WithField(name, "phải là uuid"))
		return uuid.Nil, false
	}
	return id, true
}

var validStatuses = map[string]bool{
	"new": true, "learning": true, "review": true, "relearning": true, "suspended": true,
}
```

Create `internal/vocabulary/handler/routes.go`:
```go
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
	"github.com/memorix/memorix/internal/vocabulary/service"
)

// RegisterRoutes gắn route vocabulary vào group /api/v1.
func RegisterRoutes(g *gin.RouterGroup, h *Handler) {
	e := g.Group("/vocabulary")
	e.POST("/entries", h.create)
	e.GET("/entries", h.list)
	e.GET("/entries/:id", h.get)
	e.PATCH("/entries/:id", h.update)
	e.DELETE("/entries/:id", h.delete)
	e.GET("/curated-decks", h.listDecks)
	e.POST("/curated-decks/:id/enroll", h.enroll)
}

// ---- request/response DTOs ----

type meaningReq struct {
	PartOfSpeech string `json:"part_of_speech"`
	Definition   string `json:"definition"`
}
type pronReq struct {
	IPA      string `json:"ipa"`
	Dialect  string `json:"dialect"`
	AudioURL string `json:"audio_url"`
}
type entryReq struct {
	Term           string       `json:"term"`
	PartOfSpeech   string       `json:"part_of_speech"`
	Notes          string       `json:"notes"`
	Source         string       `json:"source"`
	Directions     []string     `json:"directions"`
	Meanings       []meaningReq `json:"meanings"`
	Examples       []string     `json:"examples"`
	Pronunciations []pronReq    `json:"pronunciations"`
	Synonyms       []string     `json:"synonyms"`
	Antonyms       []string     `json:"antonyms"`
}

func (r entryReq) toCreate() service.CreateEntryInput {
	in := service.CreateEntryInput{
		Term: r.Term, PartOfSpeech: r.PartOfSpeech, Notes: r.Notes, Source: r.Source,
		Directions: r.Directions, Examples: r.Examples, Synonyms: r.Synonyms, Antonyms: r.Antonyms,
	}
	for _, m := range r.Meanings {
		in.Meanings = append(in.Meanings, service.MeaningInput{PartOfSpeech: m.PartOfSpeech, Definition: m.Definition})
	}
	for _, p := range r.Pronunciations {
		in.Pronunciations = append(in.Pronunciations, service.PronunciationInput{IPA: p.IPA, Dialect: p.Dialect, AudioURL: p.AudioURL})
	}
	return in
}

func (r entryReq) toUpdate() service.UpdateEntryInput {
	c := r.toCreate()
	return service.UpdateEntryInput{
		Term: c.Term, PartOfSpeech: c.PartOfSpeech, Notes: c.Notes, Source: c.Source,
		Meanings: c.Meanings, Examples: c.Examples, Pronunciations: c.Pronunciations,
		Synonyms: c.Synonyms, Antonyms: c.Antonyms,
	}
}

func entryToJSON(e domain.Entry, status string) gin.H {
	meanings := make([]gin.H, 0, len(e.Meanings))
	for _, m := range e.Meanings {
		meanings = append(meanings, gin.H{"id": m.ID, "part_of_speech": m.PartOfSpeech, "definition": m.Definition, "position": m.Position})
	}
	examples := make([]gin.H, 0, len(e.Examples))
	for _, x := range e.Examples {
		examples = append(examples, gin.H{"id": x.ID, "text": x.Text, "position": x.Position})
	}
	prons := make([]gin.H, 0, len(e.Pronunciations))
	for _, p := range e.Pronunciations {
		prons = append(prons, gin.H{"id": p.ID, "ipa": p.IPA, "dialect": p.Dialect, "audio_url": p.AudioURL})
	}
	rels := make([]gin.H, 0, len(e.Relations))
	for _, s := range e.Relations {
		rels = append(rels, gin.H{"id": s.ID, "relation": string(s.Relation), "value": s.Value})
	}
	return gin.H{
		"id": e.ID, "term": e.Term, "part_of_speech": e.PartOfSpeech, "notes": e.Notes,
		"source": e.Source, "status": status, "created_at": e.CreatedAt, "updated_at": e.UpdatedAt,
		"meanings": meanings, "examples": examples, "pronunciations": prons, "synonyms_antonyms": rels,
	}
}

// ---- handlers ----

func (h *Handler) create(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	var req entryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErr(c, httpx.NewError(httpx.CodeValidation, "body không hợp lệ"))
		return
	}
	e, err := h.svc.Create(c.Request.Context(), owner, req.toCreate())
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": entryToJSON(e, "new")})
}

func (h *Handler) get(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	view, err := h.svc.Get(c.Request.Context(), owner, id)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entryToJSON(view.Entry, view.Status)})
}

func (h *Handler) update(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req entryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErr(c, httpx.NewError(httpx.CodeValidation, "body không hợp lệ"))
		return
	}
	e, err := h.svc.Update(c.Request.Context(), owner, id, req.toUpdate())
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entryToJSON(e, "")})
}

func (h *Handler) delete(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), owner, id); err != nil {
		mapErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) list(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	status := c.Query("status")
	if status != "" && !validStatuses[status] {
		writeErr(c, httpx.NewError(httpx.CodeValidation, "status không hợp lệ").WithField("status", "whitelist"))
		return
	}
	limit := 0
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	res, err := h.svc.List(c.Request.Context(), service.ListInput{
		OwnerID: owner, Status: status, Query: c.Query("q"), Cursor: c.Query("cursor"), Limit: limit,
	})
	if err != nil {
		mapErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(res.Items))
	for _, it := range res.Items {
		items = append(items, gin.H{
			"id": it.Entry.ID, "term": it.Entry.Term, "part_of_speech": it.Entry.PartOfSpeech,
			"status": it.Status, "created_at": it.Entry.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "page": res.Page})
}

func (h *Handler) listDecks(c *gin.Context) {
	if _, ok := h.owner(c); !ok {
		return
	}
	decks, err := h.svc.ListCuratedDecks(c.Request.Context())
	if err != nil {
		mapErr(c, err)
		return
	}
	out := make([]gin.H, 0, len(decks))
	for _, d := range decks {
		out = append(out, gin.H{"id": d.ID, "slug": d.Slug, "name": d.Name, "description": d.Description})
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *Handler) enroll(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	deckID, ok := parseID(c, "id")
	if !ok {
		return
	}
	enrollmentID, err := h.svc.Enroll(c.Request.Context(), owner, deckID)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"enrollment_id": enrollmentID, "status": "pending"}})
}

var _ = uuid.Nil // giữ import uuid nếu route file không tham chiếu trực tiếp
```

> **Dọn dẹp:** Xóa dòng `var _ = uuid.Nil` nếu `uuid` đã được dùng ở nơi khác trong file. Nếu `go vet` báo `uuid` unused, giữ lại dòng đó; nếu báo redundant, xóa. Chạy `goimports -w internal/vocabulary/handler/` để tự chỉnh import.

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
goimports -w internal/vocabulary/handler/ 2>/dev/null || true
go test ./internal/vocabulary/handler/ -v
```
Expected: PASS (5 tests: 201, 409 với existing_id, 400, 200 decks, 202 enroll).

- [ ] **Step 5: Commit**

```bash
git add internal/vocabulary/handler/
git commit -m "feat(vocabulary): Gin handlers for entries CRUD, list+filter, decks, enroll (AD-14)"
```

---

### Task 16: Migration 0005 — seed starter IELTS deck

**Files:**
- Create: `migrations/0005_seed_starter_deck.up.sql`
- Create: `migrations/0005_seed_starter_deck.down.sql`
- Test: `internal/vocabulary/repo/seed_test.go`

- [ ] **Step 1: Write migration up**

Create `migrations/0005_seed_starter_deck.up.sql`:
```sql
-- Bộ khởi đầu IELTS seed (FR-11a, AD-6: curated owner_id NULL). Idempotent qua slug.
INSERT INTO vocabulary.curated_decks (id, slug, name, description)
VALUES ('00000000-0000-0000-0000-0000000d0001', 'ielts-starter', 'IELTS Starter',
        'Bộ từ vựng IELTS khởi đầu để bắt đầu học ngay (chống cold-start).')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO vocabulary.entries (owner_id, curated_deck_id, term, part_of_speech, source)
SELECT NULL, '00000000-0000-0000-0000-0000000d0001', t.term, t.pos, 'ielts-starter'
FROM (VALUES
    ('ubiquitous','adj'),
    ('meticulous','adj'),
    ('pragmatic','adj'),
    ('resilient','adj'),
    ('ambiguous','adj'),
    ('coherent','adj'),
    ('inevitable','adj'),
    ('profound','adj')
) AS t(term, pos)
WHERE NOT EXISTS (
    SELECT 1 FROM vocabulary.entries e
    WHERE e.curated_deck_id = '00000000-0000-0000-0000-0000000d0001'
      AND e.term_normalized = vocabulary.immutable_unaccent(lower(t.term))
);

INSERT INTO vocabulary.meanings (entry_id, part_of_speech, definition, position)
SELECT e.id, 'adj', d.def, 0
FROM vocabulary.entries e
JOIN (VALUES
    ('ubiquitous','present, appearing, or found everywhere'),
    ('meticulous','showing great attention to detail; very careful'),
    ('pragmatic','dealing with things sensibly and realistically'),
    ('resilient','able to recover quickly from difficulties'),
    ('ambiguous','open to more than one interpretation; unclear'),
    ('coherent','logical and consistent'),
    ('inevitable','certain to happen; unavoidable'),
    ('profound','very great or intense; showing deep insight')
) AS d(term, def) ON e.term = d.term
WHERE e.curated_deck_id = '00000000-0000-0000-0000-0000000d0001'
  AND NOT EXISTS (SELECT 1 FROM vocabulary.meanings m WHERE m.entry_id = e.id);
```

- [ ] **Step 2: Write migration down**

Create `migrations/0005_seed_starter_deck.down.sql`:
```sql
-- Xóa deck seed; cascade dọn entries + meanings curated.
DELETE FROM vocabulary.curated_decks WHERE slug = 'ielts-starter';
```

- [ ] **Step 3: Write the failing test**

Create `internal/vocabulary/repo/seed_test.go`:
```go
package repo

import (
	"context"
	"testing"

	"github.com/memorix/memorix/internal/platform/db/dbtest"
)

func TestSeed_StarterDeckPresent(t *testing.T) {
	pool := dbtest.RunPostgres(t)
	r := New(pool)
	ctx := context.Background()

	decks, err := r.ListActiveDecks(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var deckID string
	for _, d := range decks {
		if d.Slug == "ielts-starter" {
			deckID = d.ID.String()
		}
	}
	if deckID == "" {
		t.Fatal("ielts-starter deck not seeded")
	}
	var entryCount, meaningCount int
	_ = pool.QueryRow(ctx,
		`SELECT count(*) FROM vocabulary.entries
		 WHERE curated_deck_id=$1 AND owner_id IS NULL`, deckID).Scan(&entryCount)
	_ = pool.QueryRow(ctx,
		`SELECT count(*) FROM vocabulary.meanings m
		 JOIN vocabulary.entries e ON e.id=m.entry_id WHERE e.curated_deck_id=$1`, deckID).Scan(&meaningCount)
	if entryCount != 8 {
		t.Errorf("curated entries = %d, want 8", entryCount)
	}
	if meaningCount != 8 {
		t.Errorf("curated meanings = %d, want 8", meaningCount)
	}
}
```

- [ ] **Step 4: Run test to verify it fails then passes**

Run: `go test ./internal/vocabulary/repo/ -run TestSeed -v`
Expected: FAIL trước khi có migration 0005 (0 entries); PASS sau khi thêm (8 entries + 8 meanings, curated owner_id NULL).

- [ ] **Step 5: Commit**

```bash
git add migrations/0005_seed_starter_deck.up.sql migrations/0005_seed_starter_deck.down.sql internal/vocabulary/repo/seed_test.go
git commit -m "feat(db): seed IELTS starter curated deck (FR-11a, AD-6 owner_id NULL)"
```

---

### Task 17: cmd wiring — api routes + worker job registration

**Files:**
- Modify: `cmd/api/main.go`
- Modify: `cmd/worker/main.go`
- Create: `internal/platform/authmw/principal.go` (adapter đọc principal — nếu Sprint 1 chưa expose getter dùng ở đây)

- [ ] **Step 1: Add principal adapter (bọc Sprint 1 middleware)**

> Giả định Sprint 1 đã set principal vào gin context với key `authmw.CtxUserID`. Nếu key khác, chỉ sửa hằng dưới đây. File này chỉ là adapter đọc — không định nghĩa lại auth.

Create `internal/platform/authmw/principal.go`:
```go
package authmw

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CtxUserID là key gin context mà middleware auth (Sprint 1) set principal.
const CtxUserID = "principal_user_id"

// ErrNoPrincipal khi request chưa qua auth middleware.
var ErrNoPrincipal = errors.New("no principal in context")

// UserID đọc userID principal từ gin context (dùng cho handler module).
func UserID(c *gin.Context) (uuid.UUID, error) {
	v, ok := c.Get(CtxUserID)
	if !ok {
		return uuid.Nil, ErrNoPrincipal
	}
	switch id := v.(type) {
	case uuid.UUID:
		return id, nil
	case string:
		return uuid.Parse(id)
	default:
		return uuid.Nil, ErrNoPrincipal
	}
}
```

- [ ] **Step 2: Wire api (ráp adapter → port, AD-9 tại composition root)**

Replace `cmd/api/main.go`:
```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/riverqueue/river"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/platform/config"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/platform/jobs"
	"github.com/memorix/memorix/internal/platform/logger"
	schedrepo "github.com/memorix/memorix/internal/scheduling/repo"
	schedsvc "github.com/memorix/memorix/internal/scheduling/service"
	vocabhandler "github.com/memorix/memorix/internal/vocabulary/handler"
	vocabjobs "github.com/memorix/memorix/internal/vocabulary/jobs"
	vocabports "github.com/memorix/memorix/internal/vocabulary/ports"
	vocabrepo "github.com/memorix/memorix/internal/vocabulary/repo"
	vocabsvc "github.com/memorix/memorix/internal/vocabulary/service"
)

// schedCardAdapter khớp scheduling.Service với vocabulary/ports.CardService
// (2 DTO đồng shape; adapter tránh coupling cứng giữa 2 module — AD-1/AD-9).
type schedCardAdapter struct{ svc *schedsvc.Service }

func (a schedCardAdapter) CreateCardsForEntry(ctx context.Context, in vocabports.CreateCardsInput) error {
	return a.svc.CreateCardsForEntry(ctx, schedsvc.CreateCardsInput(in))
}
func (a schedCardAdapter) CardStatusesByEntry(ctx context.Context, owner uuid, ids []uuid) (map[uuid]string, error) {
	return a.svc.CardStatusesByEntry(ctx, owner, ids)
}
func (a schedCardAdapter) EntryIDsByStatus(ctx context.Context, owner uuid, status string) ([]uuid, error) {
	return a.svc.EntryIDsByStatus(ctx, owner, status)
}
func (a schedCardAdapter) BulkCreateForDeck(ctx context.Context, owner uuid, ids []uuid) (int, error) {
	return a.svc.BulkCreateForDeck(ctx, owner, ids)
}
```

> **Sửa kiểu:** `uuid` ở trên là viết tắt minh họa. Thay `uuid` bằng `uuid.UUID` và thêm import `github.com/google/uuid`. Bản đúng hoàn chỉnh dưới đây (ghi đè cả file).

Replace `cmd/api/main.go` (bản đúng hoàn chỉnh):
```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/platform/config"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/platform/jobs"
	"github.com/memorix/memorix/internal/platform/logger"
	schedrepo "github.com/memorix/memorix/internal/scheduling/repo"
	schedsvc "github.com/memorix/memorix/internal/scheduling/service"
	vocabhandler "github.com/memorix/memorix/internal/vocabulary/handler"
	vocabjobs "github.com/memorix/memorix/internal/vocabulary/jobs"
	vocabports "github.com/memorix/memorix/internal/vocabulary/ports"
	vocabrepo "github.com/memorix/memorix/internal/vocabulary/repo"
	vocabsvc "github.com/memorix/memorix/internal/vocabulary/service"
)

type schedCardAdapter struct{ svc *schedsvc.Service }

func (a schedCardAdapter) CreateCardsForEntry(ctx context.Context, in vocabports.CreateCardsInput) error {
	return a.svc.CreateCardsForEntry(ctx, schedsvc.CreateCardsInput(in))
}
func (a schedCardAdapter) CardStatusesByEntry(ctx context.Context, owner uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	return a.svc.CardStatusesByEntry(ctx, owner, ids)
}
func (a schedCardAdapter) EntryIDsByStatus(ctx context.Context, owner uuid.UUID, status string) ([]uuid.UUID, error) {
	return a.svc.EntryIDsByStatus(ctx, owner, status)
}
func (a schedCardAdapter) BulkCreateForDeck(ctx context.Context, owner uuid.UUID, ids []uuid.UUID) (int, error) {
	return a.svc.BulkCreateForDeck(ctx, owner, ids)
}

func main() {
	log := logger.New(os.Stdout, "info")
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("config load failed", slog.Any("err", err))
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db connect failed", slog.Any("err", err))
		os.Exit(1)
	}
	defer pool.Close()

	// River client (insert-only ở API) + đảm bảo schema River.
	if err := jobs.Migrate(ctx, pool); err != nil {
		log.Error("river migrate failed", slog.Any("err", err))
		os.Exit(1)
	}
	riverClient, err := jobs.NewClient(pool, nil)
	if err != nil {
		log.Error("river client failed", slog.Any("err", err))
		os.Exit(1)
	}

	// scheduling
	schedService := schedsvc.New(schedrepo.New(pool))
	cards := schedCardAdapter{svc: schedService}

	// vocabulary
	vRepo := vocabrepo.New(pool)
	enqueuer := &vocabjobs.Enqueuer{Client: riverClient}
	vService := vocabsvc.New(vRepo, vRepo, cards, enqueuer)
	// Auth Contract (Sprint 1): authmw.UserID trả (string, bool); PrincipalFunc
	// cần (uuid.UUID, error) → wrap + uuid.Parse ở ranh giới.
	principal := func(c *gin.Context) (uuid.UUID, error) {
		uid, ok := authmw.UserID(c)
		if !ok {
			return uuid.Nil, errors.New("unauthenticated")
		}
		return uuid.Parse(uid)
	}
	vHandler := vocabhandler.New(vService, principal)

	r := httpx.NewRouter()
	v1 := r.Group("/api/v1")
	v1.Use(authmw.RequireAuth(jwtManager)) // bảo vệ route cần principal (Sprint 1)
	vocabhandler.RegisterRoutes(v1, vHandler)

	log.Info("api starting", "port", cfg.HTTPPort, "env", cfg.AppEnv)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, r); err != nil {
		log.Error("server stopped", slog.Any("err", err))
		os.Exit(1)
	}
}
```

> **Lưu ý httpx.NewRouter:** Sprint 0 trả `*gin.Engine`; `r.Group("/api/v1")` đã tồn tại route health. `RegisterRoutes` nhận `*gin.RouterGroup` — `r.Group("/api/v1")` trả đúng kiểu đó. Nếu Sprint 0 đã tạo group v1 nội bộ, expose 1 hàm `httpx.V1Group(r)` hoặc chấp nhận group trùng path (Gin cho phép). Giữ đơn giản: tạo group mới `/api/v1` ở đây cho route vocabulary.

- [ ] **Step 3: Wire worker (đăng ký EnrollWorker)**

Replace `cmd/worker/main.go`:
```go
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/riverqueue/river"

	"github.com/memorix/memorix/internal/platform/config"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/memorix/memorix/internal/platform/jobs"
	"github.com/memorix/memorix/internal/platform/logger"
	schedrepo "github.com/memorix/memorix/internal/scheduling/repo"
	schedsvc "github.com/memorix/memorix/internal/scheduling/service"
	vocabjobs "github.com/memorix/memorix/internal/vocabulary/jobs"
	vocabrepo "github.com/memorix/memorix/internal/vocabulary/repo"
)

func main() {
	log := logger.New(os.Stdout, "info")
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("config load failed", slog.Any("err", err))
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db connect failed", slog.Any("err", err))
		os.Exit(1)
	}
	defer pool.Close()

	if err := jobs.Migrate(ctx, pool); err != nil {
		log.Error("river migrate failed", slog.Any("err", err))
		os.Exit(1)
	}

	vRepo := vocabrepo.New(pool)
	schedService := schedsvc.New(schedrepo.New(pool))

	workers := river.NewWorkers()
	river.AddWorker(workers, &vocabjobs.EnrollWorker{Store: vRepo, Cards: schedService})

	client, err := jobs.NewClient(pool, workers)
	if err != nil {
		log.Error("river client failed", slog.Any("err", err))
		os.Exit(1)
	}
	log.Info("worker starting", "env", cfg.AppEnv)
	if err := client.Start(ctx); err != nil {
		log.Error("worker start failed", slog.Any("err", err))
		os.Exit(1)
	}
	select {} // giữ tiến trình sống
}
```

- [ ] **Step 4: Verify build**

Run:
```bash
goimports -w cmd/ 2>/dev/null || true
go build ./...
```
Expected: no error (adapter thỏa `vocabports.CardService`; worker deps khớp `EnrollStore`/`BulkCardCreator`).

- [ ] **Step 5: Verify full suite (short + container)**

Run:
```bash
go test ./... -short
go test ./internal/vocabulary/... ./internal/scheduling/... ./internal/platform/jobs/... -v
```
Expected: short suite xanh; container tests xanh (cần Docker).

- [ ] **Step 6: Commit**

```bash
git add cmd/ internal/platform/authmw/principal.go
git commit -m "feat(api,worker): wire vocabulary+scheduling, enroll worker, principal adapter (AD-9)"
```

---

## Self-Review

**1. Spec coverage (Story AC → Task):**

| Story | AC chính | Task |
| --- | --- | --- |
| 2.1 Thêm từ term-only + tự tạo card | entries + term_normalized (unaccent+lower), owner_id | T1 (schema/generated col), T9 (repo insert), T11 (service Create) |
| 2.1 | tự tạo card New qua port scheduling (AD-9, không ghi thẳng) | T5–T7 (scheduling card), T8 (port), T11 (Create gọi CardService) |
| 2.1 | bảng con meanings/examples/pronunciations/syn-ant + escape HTML | T1 (bảng), T9 (insertChildren), T11 (TestCreate_EscapesHTML) |
| 2.2 Cảnh báo trùng | 409 + gợi ý mở entry cũ (existing_id) | T9 (ExistingID + 23505), T11 (DuplicateError), T15 (409 handler test) |
| 2.3 Xem/sửa/xóa | detail + card status; update giữ FSRS; soft delete; ownership | T9 (FindByID/Update/SoftDelete), T11 (Get/Update/Delete + ownership test), T15 |
| 2.4 List + filter + virtual | cursor pagination; lọc status qua port (no cross-schema join); FTS; empty | T1 (gin FTS + idx), T6 (EntryIDsByStatus/CardStatusesByEntry), T9 (ListPage/ByIDs), T11 (List), T15 |
| 2.5 Starter deck + enroll | seed curated owner_id NULL; enroll → bulk card qua job idempotent; 409 re-enroll | T16 (seed), T10 (enrollment unique), T12 (Enroll), T13–T14 (River job idempotent), T6 (BulkCreateForDeck) |
| 2.6 Onboarding gợi ý enroll | list curated decks để gợi ý; skip → empty | T12 (ListCuratedDecks), T15 (endpoint). Goal-setting (desired retention) = Epic 3 — ghi rõ deferred |

**2. Placeholder scan:** Không có "TBD/implement later". Các "TODO" cố ý: `authmw.RequireAuth()` (Sprint 1 sở hữu middleware; sprint này chỉ đọc principal) và ghi chú deferred goal-setting (Epic 3) — hợp lệ. Hai chỗ có bản "sai rồi ghi đè bản đúng" (T15 handler interface, T17 adapter) là kỹ thuật trình bày để chỉ rõ cạm bẫy kiểu context/uuid — bản đúng hoàn chỉnh đi kèm ngay dưới, engineer dùng bản đúng.

**3. Type consistency:**
- `ports.CreateCardsInput` (vocabulary) và `schedsvc.CreateCardsInput` đồng shape → convert bằng `schedsvc.CreateCardsInput(in)` ở adapter (T7 test + T17 wiring). Nhất quán.
- `CardService` interface (T8) khớp method set của `schedsvc.Service` (T7) và fake (`fakeCards` T11) và adapter (T17). 4 method: CreateCardsForEntry/CardStatusesByEntry/EntryIDsByStatus/BulkCreateForDeck.
- Cursor: `repo.cursorTimeLayout` = `time.RFC3339Nano`; `service.cursorLayout` = literal RFC3339Nano string (T11) — cùng layout, round-trip encode/parse khớp (T9 TestEntryRepo_ListPage dùng `cursorTimeLayout`).
- `EntryRepo`/`DeckRepo` interface (T11 service) khớp `*repo.Repo` method set (T9+T10) — 1 struct implements cả hai; wiring T17 truyền `vRepo` cho cả 2 tham số.
- `EnrollStore`/`BulkCardCreator` (T14) khớp `*repo.Repo` (CuratedEntryIDs/CompleteEnrollment) và `*schedsvc.Service` (BulkCreateForDeck) — T17 truyền đúng.
- `httpx.APIError/Cursor/Page`, `config.Config`, `logger.New`, `db.Migrate/Connect` — reuse nguyên từ Sprint 0, không định nghĩa lại.

**4. Kiến trúc invariants:**
- AD-6: entries.owner_id NULL = curated; cards per-user/per-direction ref entry_id — giữ.
- AD-9: vocabulary → scheduling chỉ qua `CardService` (interface phía gọi, tránh import cycle); ráp ở cmd/api. Không import internal chéo.
- AD-10: FK chỉ trong vocabulary schema (curated_deck_id, entry_id children); cards.entry_id/owner_id là cột id logic không FK; list filter batch-load qua port, không join chéo schema.
- AD-14: envelope lỗi + cursor pagination + `/api/v1` xuyên suốt handler.

**Gaps / deferred (ghi rõ):**
- Onboarding goal-setting (desired retention, daily limits) = Epic 3 (`user_scheduler_prefs`) — sprint này chỉ cung cấp list curated-decks + enroll cho luồng onboarding.
- Card partial-failure: Create commit entry trước rồi gọi CardService; nếu tạo card lỗi trả 500, entry đã tồn tại (bù trừ/retry để Epic sau; MVP chấp nhận, có ExistingID chống trùng khi tạo lại).
- `authmw.RequireAuth()` middleware do Sprint 1 sở hữu; T17 chỉ dùng `authmw.UserID` reader + adapter key (đổi hằng nếu Sprint 1 dùng key khác).
- Virtual scroll (UX-DR6) là frontend; backend cung cấp cursor pagination + status badge đủ để FE ảo hóa.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-07-07-sprint-2-vocabulary.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — dispatch một subagent mới mỗi task, review giữa các task, iterate nhanh.

**2. Inline Execution** — chạy task trong session này qua executing-plans, batch có checkpoint.

**Which approach?**
