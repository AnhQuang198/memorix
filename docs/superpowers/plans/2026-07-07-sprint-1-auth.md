# Sprint 1 — Auth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dựng module `identity` hoàn chỉnh cho Memorix — đăng ký, xác thực email, đăng nhập + phiên (JWT access 15m + refresh rotation/reuse-detect), OAuth (Authorization Code + PKCE + id_token), đặt lại mật khẩu, hồ sơ, GDPR export/delete — phủ Story 1.2–1.8 của Epic 1.

**Architecture:** Hexagonal-nhẹ cho `identity` (service + repo + ports; expose `IdentityPort`). `internal/identity/{domain,service,ports,handler,repo}` + adapters ở `internal/platform/{authmw,security,ratelimit}`. Domain thuần (AD-2, depguard chặn gin/pgx/net/http). Auth theo AD-11: access JWT stateless verify ở `platform/authmw` → `principal` xuống service (không xuống domain); refresh opaque hash trong DB, rotation + reuse-detection revoke cả `family_id`. OAuth verify id_token, không auto-merge email chưa verified. Mọi HTTP lỗi theo envelope AD-14. FK chỉ trong schema `identity` (AD-10).

**Tech Stack:** Go 1.26, Gin v1.10, pgx/v5 v5.10.0, golang-migrate v4, `github.com/alexedwards/argon2id` v1.0.0, `github.com/golang-jwt/jwt/v5` v5.3.1, `golang.org/x/oauth2` v0.36.0, `github.com/coreos/go-oidc/v3` v3.18.0, `github.com/google/uuid` v1.6.0, sqlc CLI v1.31.1, testcontainers-go, testify. Postgres 18 (`citext`, `pgcrypto`).

**Nguồn:** `_bmad-output/planning-artifacts/epics.md` (Story 1.2–1.8) + `ARCHITECTURE-SPINE.md` (AD-2, AD-10, AD-11, AD-14) + `addendum-structure.md` (S1–S7). **Tái dùng nguyên platform types từ Sprint 0** (`2026-07-07-sprint-0-foundation.md`): `httpx.APIError`/`ErrorCode`/`Cursor`/`Page`, `config.Config`, `logger.New/Scrub`, `eventbus.Bus/Event/InProcess`, `db.Migrate`, pattern testcontainers.

**Scope boundary:** CHỈ module `identity` + adapters platform cho auth. KHÔNG động vocabulary/scheduling/review/progress. Rate-limit login dùng in-memory fixed-window (MVP 1-instance; nâng Redis khi multi-instance — deferred). Repo `identity` dùng pgx trực tiếp (SQL tường minh, tránh ma sát pgtype nullable của sqlc cho CRUD auth); sqlc giữ nguyên cho module read-model nặng về sau.

---

### Task 1: Thêm dependencies auth + verify versions

**Files:** sửa `go.mod`, `go.sum`

- [ ] **Step 1: go get các lib đã pin version (đã verify trên web 2026-07)**

Run:
```bash
go get github.com/alexedwards/argon2id@v1.0.0
go get github.com/golang-jwt/jwt/v5@v5.3.1
go get golang.org/x/oauth2@v0.36.0
go get github.com/coreos/go-oidc/v3@v3.18.0
go get github.com/google/uuid@v1.6.0
```
Expected: 5 dòng require mới trong `go.mod`, không lỗi resolve.

- [ ] **Step 2: Verify build vẫn xanh**

Run: `go build ./...`
Expected: no error (chưa dùng lib nào, chỉ tải về).

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(deps): add argon2id, jwt/v5, oauth2, go-oidc, uuid for auth"
```

---

### Task 2: Migration 0002 — bảng schema `identity` (TDD testcontainers)

**Files:**
- Create: `migrations/0002_identity.up.sql`, `migrations/0002_identity.down.sql`
- Test: `internal/identity/repo/schema_test.go`

- [ ] **Step 1: Write migration SQL**

Create `migrations/0002_identity.up.sql`:
```sql
-- Story 1.2/1.3/1.4/1.5/1.8 — bảng module identity. FK CHỈ trong schema identity (AD-10).

CREATE TABLE identity.users (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email             citext NOT NULL,
    password_hash     text NOT NULL DEFAULT '',
    display_name      text NOT NULL DEFAULT '',
    timezone          text NOT NULL DEFAULT 'UTC',
    locale            text NOT NULL DEFAULT 'vi',
    theme             text NOT NULL DEFAULT 'system',
    email_verified_at timestamptz,
    plan              text NOT NULL DEFAULT 'free',
    role              text NOT NULL DEFAULT 'user',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz
);
CREATE UNIQUE INDEX users_email_active_uniq ON identity.users (email) WHERE deleted_at IS NULL;

CREATE TABLE identity.email_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
    kind       text NOT NULL CHECK (kind IN ('verify','reset')),
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_tokens_user_idx ON identity.email_tokens (user_id);

CREATE TABLE identity.sessions (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
    family_id          uuid NOT NULL,
    refresh_token_hash text NOT NULL UNIQUE,
    rotated_to         uuid REFERENCES identity.sessions(id),
    expires_at         timestamptz NOT NULL,
    revoked_at         timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_family_idx ON identity.sessions (family_id);
CREATE INDEX sessions_user_idx ON identity.sessions (user_id);

CREATE TABLE identity.oauth_identities (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
    provider     text NOT NULL,
    provider_uid text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_uid)
);
CREATE INDEX oauth_identities_user_idx ON identity.oauth_identities (user_id);
```

Create `migrations/0002_identity.down.sql`:
```sql
DROP TABLE IF EXISTS identity.oauth_identities;
DROP TABLE IF EXISTS identity.sessions;
DROP TABLE IF EXISTS identity.email_tokens;
DROP TABLE IF EXISTS identity.users;
```

- [ ] **Step 2: Write the failing integration test**

Create `internal/identity/repo/schema_test.go`:
```go
package repo

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/memorix/memorix/internal/platform/db"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startPG(t *testing.T) string {
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
	return dsn
}

func TestIdentitySchema_TablesExist(t *testing.T) {
	dsn := startPG(t)
	ctx := context.Background()
	conn, _ := pgx.Connect(ctx, dsn)
	defer conn.Close(ctx)

	var count int
	err := conn.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables
		 WHERE table_schema = 'identity'
		 AND table_name IN ('users','email_tokens','sessions','oauth_identities')`).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 identity tables, got %d", count)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/identity/repo/ -run TestIdentitySchema -v`
Expected: FAIL — migration 0002 chưa được apply đúng / bảng chưa tồn tại nếu SQL sai (chạy lần đầu để chứng minh test thật sự chạy migration). Nếu SQL đúng ngay, test PASS ở bước sau; đảm bảo test build được trước (imports repo package rỗng → cần file `doc.go`, xem Step 4).

- [ ] **Step 4: Thêm doc.go để package repo build được**

Create `internal/identity/repo/doc.go`:
```go
// Package repo chứa driven adapter Postgres (pgx) cho module identity.
package repo
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/identity/repo/ -run TestIdentitySchema -v`
Expected: PASS (4 bảng identity tồn tại).

- [ ] **Step 6: Commit**

```bash
git add migrations/0002_identity.up.sql migrations/0002_identity.down.sql internal/identity/repo/
git commit -m "feat(identity): migration for users, email_tokens, sessions, oauth_identities"
```

---

### Task 3: Domain — entities, errors, helpers (TDD)

**Files:**
- Create: `internal/identity/domain/user.go`, `internal/identity/domain/errors.go`
- Test: `internal/identity/domain/user_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/identity/domain/user_test.go`:
```go
package domain

import (
	"testing"
	"time"
)

func TestUser_IsVerified(t *testing.T) {
	u := User{}
	if u.IsVerified() {
		t.Error("new user must be unverified")
	}
	now := time.Now()
	u.EmailVerifiedAt = &now
	if !u.IsVerified() {
		t.Error("user with EmailVerifiedAt should be verified")
	}
}

func TestSession_Active(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	s := Session{ExpiresAt: now.Add(time.Hour)}
	if !s.Active(now) {
		t.Error("session should be active before expiry")
	}
	if s.Active(now.Add(2 * time.Hour)) {
		t.Error("session should be inactive after expiry")
	}
	revoked := now
	s.RevokedAt = &revoked
	if s.Active(now) {
		t.Error("revoked session must be inactive")
	}
}

func TestEmailToken_Usable(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	tok := EmailToken{ExpiresAt: now.Add(time.Hour)}
	if !tok.Usable(now) {
		t.Error("fresh token should be usable")
	}
	if tok.Usable(now.Add(2 * time.Hour)) {
		t.Error("expired token unusable")
	}
	used := now
	tok.UsedAt = &used
	if tok.Usable(now) {
		t.Error("used token unusable")
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  Linh@Example.COM "); got != "linh@example.com" {
		t.Errorf("NormalizeEmail = %q", got)
	}
}

func TestValidTimezone(t *testing.T) {
	if !ValidTimezone("Asia/Ho_Chi_Minh") {
		t.Error("valid IANA tz rejected")
	}
	if ValidTimezone("Mars/Phobos") {
		t.Error("bogus tz accepted")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/domain/ -v`
Expected: FAIL (build error — `User`/`Session`/`EmailToken`/`NormalizeEmail`/`ValidTimezone` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/identity/domain/user.go`:
```go
package domain

import (
	"strings"
	"time"
)

type Plan string

const (
	PlanFree Plan = "free"
	PlanPro  Plan = "pro"
)

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type TokenKind string

const (
	KindVerify TokenKind = "verify"
	KindReset  TokenKind = "reset"
)

// User là aggregate tài khoản. password_hash rỗng = tài khoản chỉ-OAuth.
type User struct {
	ID              string
	Email           string
	PasswordHash    string
	DisplayName     string
	Timezone        string
	Locale          string
	Theme           string
	EmailVerifiedAt *time.Time
	Plan            Plan
	Role            Role
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

func (u *User) IsVerified() bool { return u.EmailVerifiedAt != nil }

// Session là refresh session; rotation qua family_id (AD-11).
type Session struct {
	ID               string
	UserID           string
	FamilyID         string
	RefreshTokenHash string
	RotatedTo        *string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
}

func (s *Session) Active(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.ExpiresAt)
}

// EmailToken dùng cho verify (TTL 24h) và reset (TTL 1h); lưu hash, 1-lần.
type EmailToken struct {
	ID        string
	UserID    string
	Kind      TokenKind
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (t *EmailToken) Usable(now time.Time) bool {
	return t.UsedAt == nil && now.Before(t.ExpiresAt)
}

type OAuthIdentity struct {
	ID          string
	UserID      string
	Provider    string
	ProviderUID string
	CreatedAt   time.Time
}

// NormalizeEmail chuẩn hóa cho so khớp logic (DB dùng citext, đây cho service).
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidTimezone kiểm tra IANA name (AD-12 — "ngày học" theo TZ user).
func ValidTimezone(tz string) bool {
	_, err := time.LoadLocation(tz)
	return err == nil
}
```

Create `internal/identity/domain/errors.go`:
```go
package domain

import "errors"

var (
	ErrNotFound           = errors.New("identity: not found")
	ErrEmailTaken         = errors.New("identity: email already registered")
	ErrWeakPassword       = errors.New("identity: password too weak")
	ErrInvalidCredentials = errors.New("identity: invalid credentials")
	ErrTokenInvalid       = errors.New("identity: token invalid or expired")
	ErrReuseDetected      = errors.New("identity: refresh token reuse detected")
	ErrRateLimited        = errors.New("identity: too many attempts")
	ErrInvalidProfile     = errors.New("identity: invalid profile field")
	ErrOAuthFailed        = errors.New("identity: oauth verification failed")
	ErrOAuthNoMerge       = errors.New("identity: cannot merge account on unverified email")
)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/identity/domain/ -v`
Expected: PASS (5 test).

- [ ] **Step 5: Commit**

```bash
git add internal/identity/domain/
git commit -m "feat(identity): domain entities, sentinel errors, email/tz helpers"
```

---

### Task 4: Domain — chính sách độ mạnh mật khẩu (TDD)

**Files:**
- Create: `internal/identity/domain/password.go`
- Test: `internal/identity/domain/password_test.go`

Story 1.2 AC: mật khẩu ≥8 ký tự; zxcvbn score <2 → 400. Ta implement bộ ước lượng score 0–4 thuần (length + đa dạng lớp ký tự + blocklist), ngưỡng chấp nhận ≥2 — không phụ thuộc lib ngoài (giữ domain thuần, AD-2).

- [ ] **Step 1: Write the failing test**

Create `internal/identity/domain/password_test.go`:
```go
package domain

import "testing"

func TestPasswordStrongEnough(t *testing.T) {
	weak := []string{
		"short7A",    // < 8
		"password",   // blocklist
		"aaaaaaaa",   // 8, một lớp ký tự
		"12345678",   // blocklist
	}
	for _, pw := range weak {
		if PasswordStrongEnough(pw) {
			t.Errorf("expected %q weak (score<2)", pw)
		}
	}
	strong := []string{
		"Tr0ub4dour",         // 10, 3 lớp
		"MyPa55word!!",       // 12, 4 lớp
		"correct-horse9Batt", // dài + đa dạng
	}
	for _, pw := range strong {
		if !PasswordStrongEnough(pw) {
			t.Errorf("expected %q strong (score>=2)", pw)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/domain/ -run TestPasswordStrongEnough -v`
Expected: FAIL (`PasswordStrongEnough` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/identity/domain/password.go`:
```go
package domain

import (
	"strings"
	"unicode"
)

var commonPasswords = map[string]bool{
	"password": true, "12345678": true, "qwerty": true, "qwertyui": true,
	"letmein": true, "iloveyou": true, "admin123": true, "memorix": true,
}

// EstimateStrength trả 0–4 (xấp xỉ zxcvbn): độ dài + số lớp ký tự, phạt blocklist.
func EstimateStrength(pw string) int {
	if len(pw) < 8 {
		return 0
	}
	if commonPasswords[strings.ToLower(pw)] {
		return 0
	}
	var lower, upper, digit, sym bool
	for _, r := range pw {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		default:
			sym = true
		}
	}
	classes := 0
	for _, has := range []bool{lower, upper, digit, sym} {
		if has {
			classes++
		}
	}
	score := 0
	switch {
	case len(pw) >= 12:
		score += 2
	case len(pw) >= 10:
		score++
	}
	score += classes - 1
	if score < 0 {
		score = 0
	}
	if score > 4 {
		score = 4
	}
	return score
}

// PasswordStrongEnough: ngưỡng chấp nhận zxcvbn score >= 2 (Story 1.2).
func PasswordStrongEnough(pw string) bool { return EstimateStrength(pw) >= 2 }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/identity/domain/ -run TestPasswordStrongEnough -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/identity/domain/password.go internal/identity/domain/password_test.go
git commit -m "feat(identity): password strength policy (zxcvbn-style score threshold)"
```

---

### Task 5: platform/security — argon2id hasher + opaque token factory (TDD)

**Files:**
- Create: `internal/platform/security/argon2.go`, `internal/platform/security/tokens.go`
- Test: `internal/platform/security/argon2_test.go`, `internal/platform/security/tokens_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/platform/security/argon2_test.go`:
```go
package security

import "testing"

func TestArgon2Hasher_RoundTrip(t *testing.T) {
	h := NewArgon2Hasher()
	hash, err := h.Hash("Tr0ub4dour!")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "Tr0ub4dour!" || hash == "" {
		t.Fatal("hash must not be plaintext/empty")
	}
	ok, err := h.Verify("Tr0ub4dour!", hash)
	if err != nil || !ok {
		t.Errorf("verify correct password failed: ok=%v err=%v", ok, err)
	}
	bad, _ := h.Verify("wrong", hash)
	if bad {
		t.Error("verify must reject wrong password")
	}
}

func TestArgon2Hasher_EmptyHashRejects(t *testing.T) {
	h := NewArgon2Hasher()
	ok, err := h.Verify("anything", "")
	if err != nil || ok {
		t.Errorf("empty hash (oauth-only user) must reject, got ok=%v err=%v", ok, err)
	}
}

func TestArgon2Hasher_SaltedUnique(t *testing.T) {
	h := NewArgon2Hasher()
	a, _ := h.Hash("samePass123")
	b, _ := h.Hash("samePass123")
	if a == b {
		t.Error("same password must yield different hashes (random salt)")
	}
}
```

Create `internal/platform/security/tokens_test.go`:
```go
package security

import "testing"

func TestTokenFactory_NewAndHash(t *testing.T) {
	f := TokenFactory{}
	raw, hash := f.New()
	if raw == "" || hash == "" {
		t.Fatal("New must return non-empty raw and hash")
	}
	if raw == hash {
		t.Error("raw must differ from its hash")
	}
	if got := f.Hash(raw); got != hash {
		t.Errorf("Hash(raw) not stable: %q vs %q", got, hash)
	}
}

func TestTokenFactory_Unpredictable(t *testing.T) {
	f := TokenFactory{}
	r1, _ := f.New()
	r2, _ := f.New()
	if r1 == r2 {
		t.Error("two tokens must differ")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/platform/security/ -v`
Expected: FAIL (`NewArgon2Hasher`/`TokenFactory` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/platform/security/argon2.go`:
```go
package security

import "github.com/alexedwards/argon2id"

// Argon2Hasher implements identity ports.Hasher bằng argon2id (NFR-7).
type Argon2Hasher struct {
	params *argon2id.Params
}

func NewArgon2Hasher() *Argon2Hasher {
	return &Argon2Hasher{params: argon2id.DefaultParams}
}

func (h *Argon2Hasher) Hash(plain string) (string, error) {
	return argon2id.CreateHash(plain, h.params)
}

func (h *Argon2Hasher) Verify(plain, hash string) (bool, error) {
	if hash == "" {
		return false, nil
	}
	return argon2id.ComparePasswordAndHash(plain, hash)
}
```

Create `internal/platform/security/tokens.go`:
```go
package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// TokenFactory tạo token opaque (refresh, verify, reset) và hash SHA-256 để lưu DB.
// Chỉ hash được lưu; raw gửi client 1 lần.
type TokenFactory struct{}

func (TokenFactory) New() (raw, hash string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("security: crypto/rand failed: " + err.Error())
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashHex(raw)
}

func (TokenFactory) Hash(raw string) string { return hashHex(raw) }

func hashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/platform/security/ -v`
Expected: PASS (5 test).

- [ ] **Step 5: Commit**

```bash
git add internal/platform/security/
git commit -m "feat(platform): argon2id hasher and opaque token factory"
```

---

### Task 6: platform/authmw — JWT access token issue/verify (TDD)

**Files:**
- Create: `internal/platform/authmw/jwt.go`
- Test: `internal/platform/authmw/jwt_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/platform/authmw/jwt_test.go`:
```go
package authmw

import (
	"testing"
	"time"
)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestJWTManager_IssueVerifyRoundTrip(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	m := NewJWTManager([]byte("test-secret-please-change"), 15*time.Minute, "memorix")
	m.now = fixedClock(base)

	tok, exp, err := m.Issue("user-1", "user", "free")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !exp.Equal(base.Add(15 * time.Minute)) {
		t.Errorf("exp = %v, want base+15m", exp)
	}
	p, err := m.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if p.UserID != "user-1" || p.Role != "user" || p.Plan != "free" {
		t.Errorf("principal = %+v", p)
	}
}

func TestJWTManager_RejectsExpired(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	m := NewJWTManager([]byte("s3cret"), 15*time.Minute, "memorix")
	m.now = fixedClock(base)
	tok, _, _ := m.Issue("u", "user", "free")
	m.now = fixedClock(base.Add(20 * time.Minute)) // access 15m đã hết hạn
	if _, err := m.Verify(tok); err == nil {
		t.Error("expected expired token rejected")
	}
}

func TestJWTManager_RejectsWrongSecret(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	issuer := NewJWTManager([]byte("secret-A"), 15*time.Minute, "memorix")
	issuer.now = fixedClock(base)
	tok, _, _ := issuer.Issue("u", "user", "free")

	attacker := NewJWTManager([]byte("secret-B"), 15*time.Minute, "memorix")
	attacker.now = fixedClock(base)
	if _, err := attacker.Verify(tok); err == nil {
		t.Error("expected wrong-secret token rejected")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/platform/authmw/ -run TestJWTManager -v`
Expected: FAIL (`NewJWTManager`/`Principal` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/platform/authmw/jwt.go`:
```go
package authmw

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Principal là chủ thể đã xác thực (AD-11) — xuống service, KHÔNG xuống domain.
type Principal struct {
	UserID string
	Role   string
	Plan   string
}

// JWTManager phát/verify access JWT HS256 ngắn hạn (15m).
type JWTManager struct {
	secret []byte
	ttl    time.Duration
	issuer string
	now    func() time.Time
}

func NewJWTManager(secret []byte, ttl time.Duration, issuer string) *JWTManager {
	return &JWTManager{secret: secret, ttl: ttl, issuer: issuer, now: time.Now}
}

func (m *JWTManager) Issue(userID, role, plan string) (string, time.Time, error) {
	now := m.now()
	exp := now.Add(m.ttl)
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"plan": plan,
		"iss":  m.issuer,
		"iat":  now.Unix(),
		"exp":  exp.Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(m.secret)
	return signed, exp, err
}

func (m *JWTManager) Verify(token string) (Principal, error) {
	claims := jwt.MapClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	_, err := parser.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return m.secret, nil
	})
	if err != nil {
		return Principal{}, err
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return Principal{}, errors.New("authmw: missing sub claim")
	}
	role, _ := claims["role"].(string)
	plan, _ := claims["plan"].(string)
	return Principal{UserID: sub, Role: role, Plan: plan}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/platform/authmw/ -run TestJWTManager -v`
Expected: PASS (3 test).

- [ ] **Step 5: Commit**

```bash
git add internal/platform/authmw/jwt.go internal/platform/authmw/jwt_test.go
git commit -m "feat(platform): HS256 JWT access token issue/verify with principal claims"
```

---

### Task 7: platform/authmw — Gin middleware → principal vào context (TDD)

**Files:**
- Create: `internal/platform/authmw/middleware.go`
- Test: `internal/platform/authmw/middleware_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/platform/authmw/middleware_test.go`:
```go
package authmw

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := NewJWTManager([]byte("s3cret"), 15*time.Minute, "memorix")
	tok, _, _ := m.Issue("user-42", "user", "free")

	r := gin.New()
	r.GET("/me", RequireAuth(m), func(c *gin.Context) {
		p, ok := PrincipalFrom(c)
		if !ok {
			c.String(500, "no principal")
			return
		}
		c.String(200, p.UserID)
	})

	cases := []struct {
		name   string
		header string
		want   int
		body   string
	}{
		{"valid bearer", "Bearer " + tok, 200, "user-42"},
		{"missing header", "", 401, ""},
		{"malformed", "Basic xyz", 401, ""},
		{"garbage token", "Bearer not.a.jwt", 401, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
			if tc.body != "" && w.Body.String() != tc.body {
				t.Errorf("body = %q, want %q", w.Body.String(), tc.body)
			}
			if tc.want == 401 && w.Body.String() != "" {
				// envelope AD-14: {"error":{"code":"UNAUTHENTICATED",...}}
				if !containsCode(w.Body.String(), "UNAUTHENTICATED") {
					t.Errorf("401 body missing UNAUTHENTICATED envelope: %s", w.Body.String())
				}
			}
		})
	}
}

func containsCode(body, code string) bool {
	return len(body) > 0 && (indexOf(body, code) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/platform/authmw/ -run TestRequireAuth -v`
Expected: FAIL (`RequireAuth`/`PrincipalFrom` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/platform/authmw/middleware.go`:
```go
package authmw

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/memorix/memorix/internal/platform/httpx"
)

const principalKey = "memorix.principal"

// RequireAuth verify Bearer access JWT; đặt Principal vào context (AD-11).
// Deny-by-default: thiếu/sai token → 401 envelope chuẩn (AD-14).
func RequireAuth(m *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || raw == "" {
			abort401(c)
			return
		}
		p, err := m.Verify(raw)
		if err != nil {
			abort401(c)
			return
		}
		c.Set(principalKey, p)
		c.Next()
	}
}

// PrincipalFrom lấy Principal đã xác thực từ context.
func PrincipalFrom(c *gin.Context) (Principal, bool) {
	v, ok := c.Get(principalKey)
	if !ok {
		return Principal{}, false
	}
	p, ok := v.(Principal)
	return p, ok
}

// UserID là reader tiện lợi cho downstream (Sprint 2-5): trả UserID (uuid dạng
// string) của principal đã xác thực. TZ KHÔNG nằm trong principal/context —
// downstream lấy qua IdentityPort.UserTimezone(ctx, userID) (AD-9, AD-12).
func UserID(c *gin.Context) (string, bool) {
	p, ok := PrincipalFrom(c)
	return p.UserID, ok
}

// SetPrincipal đặt principal vào context — dùng cho wiring middleware và test
// double (thay cho việc set key thô "user_id"). Giữ principalKey đóng gói.
func SetPrincipal(c *gin.Context, p Principal) { c.Set(principalKey, p) }

func abort401(c *gin.Context) {
	e := httpx.NewError(httpx.CodeUnauthenticated, "authentication required").
		WithTrace(c.GetHeader("X-Request-Id"))
	c.AbortWithStatusJSON(e.HTTPStatus(), e)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/platform/authmw/ -run TestRequireAuth -v`
Expected: PASS (4 sub-test).

- [ ] **Step 5: Commit**

```bash
git add internal/platform/authmw/middleware.go internal/platform/authmw/middleware_test.go
git commit -m "feat(platform): RequireAuth Gin middleware injecting principal into context"
```

---

### Task 8: platform/ratelimit — fixed-window login limiter (TDD)

**Files:**
- Create: `internal/platform/ratelimit/window.go`
- Test: `internal/platform/ratelimit/window_test.go`

Chống brute-force login (NFR-10). MVP 1-instance dùng in-memory; nâng Redis khi multi-instance (deferred).

- [ ] **Step 1: Write the failing test**

Create `internal/platform/ratelimit/window_test.go`:
```go
package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestWindow_AllowsUpToLimitThenDenies(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	w := NewWindow(3, time.Minute)
	w.now = func() time.Time { return base }
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		ok, _ := w.Allow(ctx, "login:a@b.com")
		if !ok {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if ok, _ := w.Allow(ctx, "login:a@b.com"); ok {
		t.Error("4th attempt in window must be denied")
	}
	// khóa khác không bị ảnh hưởng
	if ok, _ := w.Allow(ctx, "login:other@b.com"); !ok {
		t.Error("different key must be independent")
	}
}

func TestWindow_ResetClearsKey(t *testing.T) {
	w := NewWindow(1, time.Minute)
	ctx := context.Background()
	w.Allow(ctx, "k")
	if ok, _ := w.Allow(ctx, "k"); ok {
		t.Fatal("should be denied after limit")
	}
	w.Reset(ctx, "k")
	if ok, _ := w.Allow(ctx, "k"); !ok {
		t.Error("Reset must clear the key (login success path)")
	}
}

func TestWindow_ExpiresAfterWindow(t *testing.T) {
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	w := NewWindow(1, time.Minute)
	w.now = func() time.Time { return base }
	ctx := context.Background()
	w.Allow(ctx, "k")
	w.now = func() time.Time { return base.Add(61 * time.Second) }
	if ok, _ := w.Allow(ctx, "k"); !ok {
		t.Error("attempt must be allowed after window elapsed")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/platform/ratelimit/ -v`
Expected: FAIL (`NewWindow` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/platform/ratelimit/window.go`:
```go
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Window là rate limiter fixed-window in-memory theo key (vd "login:<email>").
// Implements identity ports.LoginLimiter.
type Window struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
	now    func() time.Time
}

func NewWindow(limit int, window time.Duration) *Window {
	return &Window{
		hits:   make(map[string][]time.Time),
		limit:  limit,
		window: window,
		now:    time.Now,
	}
}

func (w *Window) Allow(_ context.Context, key string) (bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.now()
	cutoff := now.Add(-w.window)
	kept := w.hits[key][:0]
	for _, t := range w.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= w.limit {
		w.hits[key] = kept
		return false, nil
	}
	w.hits[key] = append(kept, now)
	return true, nil
}

func (w *Window) Reset(_ context.Context, key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.hits, key)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/platform/ratelimit/ -v`
Expected: PASS (3 test).

- [ ] **Step 5: Commit**

```bash
git add internal/platform/ratelimit/
git commit -m "feat(platform): in-memory fixed-window login rate limiter"
```

---

### Task 9: identity/ports + in-memory repos + service.Register (Story 1.2) (TDD)

**Files:**
- Create: `internal/identity/ports/ports.go`
- Create: `internal/identity/repo/memory/memory.go` (real in-memory repos, dùng cho unit test + dev)
- Create: `internal/identity/service/service.go`, `internal/identity/service/register.go`
- Test: `internal/identity/service/fakes_test.go`, `internal/identity/service/register_test.go`

- [ ] **Step 1: Define ports (interface — không test, sẽ compile qua service test)**

Create `internal/identity/ports/ports.go`:
```go
package ports

import (
	"context"
	"time"

	"github.com/memorix/memorix/internal/identity/domain"
)

// --- Driven ports (repo) — chỉ FK trong schema identity (AD-10) ---

type UserRepo interface {
	Create(ctx context.Context, u *domain.User) error
	ByEmail(ctx context.Context, email string) (*domain.User, error)
	ByID(ctx context.Context, id string) (*domain.User, error)
	Update(ctx context.Context, u *domain.User) error
	SoftDelete(ctx context.Context, id string, at time.Time) error
	PurgeDeletedBefore(ctx context.Context, cutoff time.Time) (int, error)
}

type SessionRepo interface {
	Create(ctx context.Context, s *domain.Session) error
	ByTokenHash(ctx context.Context, hash string) (*domain.Session, error)
	MarkRotated(ctx context.Context, id, successorID string, at time.Time) error
	RevokeFamily(ctx context.Context, familyID string, at time.Time) error
	RevokeAllForUser(ctx context.Context, userID string, at time.Time) error
}

type EmailTokenRepo interface {
	Create(ctx context.Context, t *domain.EmailToken) error
	ByTokenHash(ctx context.Context, hash string, kind domain.TokenKind) (*domain.EmailToken, error)
	MarkUsed(ctx context.Context, id string, at time.Time) error
}

type OAuthRepo interface {
	ByProviderUID(ctx context.Context, provider, uid string) (*domain.OAuthIdentity, error)
	Create(ctx context.Context, o *domain.OAuthIdentity) error
}

// --- Security / infra ports ---

type Hasher interface {
	Hash(plain string) (string, error)
	Verify(plain, hash string) (bool, error)
}

type TokenFactory interface {
	New() (raw, hash string)
	Hash(raw string) string
}

type TokenIssuer interface {
	Issue(userID, role, plan string) (accessToken string, expiresAt time.Time, err error)
}

type Clock interface{ Now() time.Time }

type LoginLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
	Reset(ctx context.Context, key string)
}

type Mailer interface {
	SendVerification(ctx context.Context, email, rawToken string) error
	SendPasswordReset(ctx context.Context, email, rawToken string) error
}

// OIDCClaims là kết quả verify id_token (AD-11).
type OIDCClaims struct {
	ProviderUID   string
	Email         string
	EmailVerified bool
}

type OIDCVerifier interface {
	Verify(ctx context.Context, provider, code, codeVerifier, redirectURI, nonce string) (OIDCClaims, error)
}

// --- Driving port expose ra module khác (AD-1) ---

type IdentityPort interface {
	UserExists(ctx context.Context, id string) (bool, error)
	UserTimezone(ctx context.Context, id string) (string, error)
}
```

- [ ] **Step 2: In-memory repos (real code, dùng chung cho test + dev)**

Create `internal/identity/repo/memory/memory.go`:
```go
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/identity/ports"
)

// Stores gom 4 repo in-memory thread-safe cho unit test và dev.
type Stores struct {
	Users    *UserStore
	Sessions *SessionStore
	Tokens   *TokenStore
	OAuth    *OAuthStore
}

func New() *Stores {
	return &Stores{
		Users:    &UserStore{byID: map[string]domain.User{}},
		Sessions: &SessionStore{byID: map[string]domain.Session{}},
		Tokens:   &TokenStore{byID: map[string]domain.EmailToken{}},
		OAuth:    &OAuthStore{byID: map[string]domain.OAuthIdentity{}},
	}
}

// --- Users ---

type UserStore struct {
	mu   sync.Mutex
	byID map[string]domain.User
}

func (s *UserStore) Create(_ context.Context, u *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[u.ID] = *u
	return nil
}

func (s *UserStore) ByEmail(_ context.Context, email string) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	email = domain.NormalizeEmail(email)
	for _, u := range s.byID {
		if u.DeletedAt == nil && domain.NormalizeEmail(u.Email) == email {
			cp := u
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *UserStore) ByID(_ context.Context, id string) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := u
	return &cp, nil
}

func (s *UserStore) Update(_ context.Context, u *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[u.ID]; !ok {
		return domain.ErrNotFound
	}
	s.byID[u.ID] = *u
	return nil
}

func (s *UserStore) SoftDelete(_ context.Context, id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	u.DeletedAt = &at
	u.UpdatedAt = at
	s.byID[id] = u
	return nil
}

func (s *UserStore) PurgeDeletedBefore(_ context.Context, cutoff time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for id, u := range s.byID {
		if u.DeletedAt != nil && u.DeletedAt.Before(cutoff) {
			delete(s.byID, id)
			n++
		}
	}
	return n, nil
}

// --- Sessions ---

type SessionStore struct {
	mu   sync.Mutex
	byID map[string]domain.Session
}

func (s *SessionStore) Create(_ context.Context, sess *domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[sess.ID] = *sess
	return nil
}

func (s *SessionStore) ByTokenHash(_ context.Context, hash string) (*domain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sess := range s.byID {
		if sess.RefreshTokenHash == hash {
			cp := sess
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *SessionStore) MarkRotated(_ context.Context, id, successorID string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	sess.RotatedTo = &successorID
	sess.RevokedAt = &at
	s.byID[id] = sess
	return nil
}

func (s *SessionStore) RevokeFamily(_ context.Context, familyID string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.byID {
		if sess.FamilyID == familyID && sess.RevokedAt == nil {
			sess.RevokedAt = &at
			s.byID[id] = sess
		}
	}
	return nil
}

func (s *SessionStore) RevokeAllForUser(_ context.Context, userID string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.byID {
		if sess.UserID == userID && sess.RevokedAt == nil {
			sess.RevokedAt = &at
			s.byID[id] = sess
		}
	}
	return nil
}

// --- Email tokens ---

type TokenStore struct {
	mu   sync.Mutex
	byID map[string]domain.EmailToken
}

func (s *TokenStore) Create(_ context.Context, t *domain.EmailToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[t.ID] = *t
	return nil
}

func (s *TokenStore) ByTokenHash(_ context.Context, hash string, kind domain.TokenKind) (*domain.EmailToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.byID {
		if t.TokenHash == hash && t.Kind == kind {
			cp := t
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *TokenStore) MarkUsed(_ context.Context, id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	t.UsedAt = &at
	s.byID[id] = t
	return nil
}

// --- OAuth identities ---

type OAuthStore struct {
	mu   sync.Mutex
	byID map[string]domain.OAuthIdentity
}

func (s *OAuthStore) ByProviderUID(_ context.Context, provider, uid string) (*domain.OAuthIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, o := range s.byID {
		if o.Provider == provider && o.ProviderUID == uid {
			cp := o
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *OAuthStore) Create(_ context.Context, o *domain.OAuthIdentity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[o.ID] = *o
	return nil
}

// Compile-time checks: memory stores thỏa ports.
var (
	_ ports.UserRepo       = (*UserStore)(nil)
	_ ports.SessionRepo    = (*SessionStore)(nil)
	_ ports.EmailTokenRepo = (*TokenStore)(nil)
	_ ports.OAuthRepo      = (*OAuthStore)(nil)
)
```

- [ ] **Step 3: Service scaffold**

Create `internal/identity/service/service.go`:
```go
package service

import (
	"context"
	"time"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/identity/ports"
	"github.com/memorix/memorix/internal/platform/eventbus"
)

// Deps gom mọi cộng tác của Service (constructor injection, dễ test bằng fake).
type Deps struct {
	Users      ports.UserRepo
	Sessions   ports.SessionRepo
	Tokens     ports.EmailTokenRepo
	OAuth      ports.OAuthRepo
	Hasher     ports.Hasher
	Issuer     ports.TokenIssuer
	Secrets    ports.TokenFactory
	Clock      ports.Clock
	Limiter    ports.LoginLimiter
	OIDC       ports.OIDCVerifier
	Bus        eventbus.Bus
	RefreshTTL time.Duration
	VerifyTTL  time.Duration
	ResetTTL   time.Duration
}

// Service là use-case layer module identity (hexagonal-nhẹ). Principal đến từ
// handler (authmw), không lộ xuống domain.
type Service struct {
	deps      Deps
	dummyHash string
}

func New(d Deps) *Service {
	// dummyHash cho constant-time compare khi email không tồn tại (chống enumeration).
	dummy, _ := d.Hasher.Hash("memorix-timing-guard")
	return &Service{deps: d, dummyHash: dummy}
}

// TokenPair trả về sau login/register/refresh. RefreshToken là opaque raw,
// handler set vào cookie httpOnly+Secure+SameSite=Strict.
type TokenPair struct {
	AccessToken     string
	AccessExpiresAt time.Time
	RefreshToken    string
}

// issueSession tạo family session mới + access token (dùng chung login/register/oauth).
func (s *Service) issueSession(ctx context.Context, u *domain.User) (TokenPair, error) {
	now := s.deps.Clock.Now()
	raw, hash := s.deps.Secrets.New()
	sess := &domain.Session{
		ID:               newID(),
		UserID:           u.ID,
		FamilyID:         newID(),
		RefreshTokenHash: hash,
		ExpiresAt:        now.Add(s.deps.RefreshTTL),
		CreatedAt:        now,
	}
	if err := s.deps.Sessions.Create(ctx, sess); err != nil {
		return TokenPair{}, err
	}
	access, exp, err := s.deps.Issuer.Issue(u.ID, string(u.Role), string(u.Plan))
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, AccessExpiresAt: exp, RefreshToken: raw}, nil
}
```

Create `internal/identity/service/ids.go`:
```go
package service

import "github.com/google/uuid"

func newID() string { return uuid.NewString() }
```

- [ ] **Step 4: Register use-case**

Create `internal/identity/service/register.go`:
```go
package service

import (
	"context"
	"errors"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/platform/eventbus"
)

type RegisterInput struct {
	Email       string
	Password    string
	DisplayName string
}

type RegisterResult struct {
	UserID      string
	Tokens      TokenPair
	VerifyToken string // raw — handler gửi email; cũng trả cho test
}

// Register tạo tài khoản email+password (Story 1.2): argon2id hash, phát access
// token, phát email-token verify (kind=verify, TTL 24h — Story 1.3).
func (s *Service) Register(ctx context.Context, in RegisterInput) (RegisterResult, error) {
	email := domain.NormalizeEmail(in.Email)
	if !domain.PasswordStrongEnough(in.Password) {
		return RegisterResult{}, domain.ErrWeakPassword
	}

	if _, err := s.deps.Users.ByEmail(ctx, email); err == nil {
		return RegisterResult{}, domain.ErrEmailTaken
	} else if !errors.Is(err, domain.ErrNotFound) {
		return RegisterResult{}, err
	}

	hash, err := s.deps.Hasher.Hash(in.Password)
	if err != nil {
		return RegisterResult{}, err
	}

	now := s.deps.Clock.Now()
	u := &domain.User{
		ID:           newID(),
		Email:        email,
		PasswordHash: hash,
		DisplayName:  in.DisplayName,
		Timezone:     "UTC",
		Locale:       "vi",
		Theme:        "system",
		Plan:         domain.PlanFree,
		Role:         domain.RoleUser,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.deps.Users.Create(ctx, u); err != nil {
		return RegisterResult{}, err
	}

	rawTok, tokHash := s.deps.Secrets.New()
	vt := &domain.EmailToken{
		ID:        newID(),
		UserID:    u.ID,
		Kind:      domain.KindVerify,
		TokenHash: tokHash,
		ExpiresAt: now.Add(s.deps.VerifyTTL),
		CreatedAt: now,
	}
	if err := s.deps.Tokens.Create(ctx, vt); err != nil {
		return RegisterResult{}, err
	}

	tokens, err := s.issueSession(ctx, u)
	if err != nil {
		return RegisterResult{}, err
	}

	s.deps.Bus.Publish(ctx, eventbus.Event{Name: "UserRegistered", Payload: u.ID})
	return RegisterResult{UserID: u.ID, Tokens: tokens, VerifyToken: rawTok}, nil
}
```

- [ ] **Step 5: Shared fakes cho service test**

Create `internal/identity/service/fakes_test.go`:
```go
package service

import (
	"context"
	"time"
)

// fakeHasher: hash tất định "h:"+plain (nhanh, xác định).
type fakeHasher struct{}

func (fakeHasher) Hash(p string) (string, error) { return "h:" + p, nil }
func (fakeHasher) Verify(p, h string) (bool, error) {
	return h != "" && h == "h:"+p, nil
}

// fakeSecrets: raw đếm tăng, hash = "H("+raw+")".
type fakeSecrets struct{ n int }

func (f *fakeSecrets) New() (string, string) {
	f.n++
	raw := "tok-" + itoa(f.n)
	return raw, "H(" + raw + ")"
}
func (f *fakeSecrets) Hash(raw string) string { return "H(" + raw + ")" }

// fakeIssuer: access = "jwt:"+userID.
type fakeIssuer struct{ now func() time.Time }

func (f fakeIssuer) Issue(userID, _, _ string) (string, time.Time, error) {
	return "jwt:" + userID, f.now().Add(15 * time.Minute), nil
}

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }

// fakeLimiter: allow theo cờ; đếm Reset.
type fakeLimiter struct {
	allow  bool
	resets int
}

func (l *fakeLimiter) Allow(context.Context, string) (bool, error) { return l.allow, nil }
func (l *fakeLimiter) Reset(context.Context, string)               { l.resets++ }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
```

Create `internal/identity/service/oidc_fake_test.go` (fake OIDC verifier, tách để dùng ở test OAuth):
```go
package service

import (
	"context"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/identity/ports"
)

type stubOIDC struct {
	claims ports.OIDCClaims
	err    error
}

func (s stubOIDC) Verify(context.Context, string, string, string, string, string) (ports.OIDCClaims, error) {
	if s.err != nil {
		return ports.OIDCClaims{}, s.err
	}
	return s.claims, nil
}

var _ ports.OIDCVerifier = stubOIDC{}
var _ = domain.ErrOAuthFailed
```

Create `internal/identity/service/harness_test.go` (dựng Deps chuẩn cho test):
```go
package service

import (
	"time"

	"github.com/memorix/memorix/internal/identity/repo/memory"
	"github.com/memorix/memorix/internal/platform/eventbus"
)

type harness struct {
	svc     *Service
	stores  *memory.Stores
	clock   *fakeClock
	limiter *fakeLimiter
	bus     *eventbus.InProcess
}

func newHarness() *harness {
	clk := &fakeClock{t: time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)}
	lim := &fakeLimiter{allow: true}
	bus := eventbus.NewInProcess()
	st := memory.New()
	svc := New(Deps{
		Users:      st.Users,
		Sessions:   st.Sessions,
		Tokens:     st.Tokens,
		OAuth:      st.OAuth,
		Hasher:     fakeHasher{},
		Issuer:     fakeIssuer{now: clk.Now},
		Secrets:    &fakeSecrets{},
		Clock:      clk,
		Limiter:    lim,
		OIDC:       stubOIDC{},
		Bus:        bus,
		RefreshTTL: 30 * 24 * time.Hour,
		VerifyTTL:  24 * time.Hour,
		ResetTTL:   time.Hour,
	})
	return &harness{svc: svc, stores: st, clock: clk, limiter: lim, bus: bus}
}
```

Create `internal/identity/service/register_test.go`:
```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/memorix/memorix/internal/identity/domain"
)

func TestRegister_CreatesHashedUserAndTokens(t *testing.T) {
	h := newHarness()
	res, err := h.svc.Register(context.Background(), RegisterInput{
		Email: "Linh@Example.com", Password: "Tr0ub4dour!", DisplayName: "Linh",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if res.Tokens.AccessToken == "" || res.Tokens.RefreshToken == "" {
		t.Error("expected access + refresh tokens")
	}
	if res.VerifyToken == "" {
		t.Error("expected verify email token issued (Story 1.3)")
	}
	u, err := h.stores.Users.ByEmail(context.Background(), "linh@example.com")
	if err != nil {
		t.Fatalf("user not persisted: %v", err)
	}
	if u.PasswordHash == "Tr0ub4dour!" || u.PasswordHash == "" {
		t.Error("password must be hashed, never raw")
	}
	if u.IsVerified() {
		t.Error("new account must be unverified until email confirmed (FR-2)")
	}
}

func TestRegister_DuplicateEmailConflict(t *testing.T) {
	h := newHarness()
	in := RegisterInput{Email: "dup@example.com", Password: "Tr0ub4dour!"}
	if _, err := h.svc.Register(context.Background(), in); err != nil {
		t.Fatalf("first register: %v", err)
	}
	_, err := h.svc.Register(context.Background(), in)
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestRegister_WeakPasswordRejected(t *testing.T) {
	h := newHarness()
	_, err := h.svc.Register(context.Background(), RegisterInput{
		Email: "weak@example.com", Password: "password",
	})
	if !errors.Is(err, domain.ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got %v", err)
	}
}

func TestRegister_EmitsUserRegistered(t *testing.T) {
	h := newHarness()
	got := make(chan string, 1)
	h.bus.Subscribe("UserRegistered", func(_ context.Context, e eventbus.Event) {
		id, _ := e.Payload.(string)
		got <- id
	})
	res, _ := h.svc.Register(context.Background(), RegisterInput{
		Email: "e@example.com", Password: "Tr0ub4dour!",
	})
	h.bus.Wait()
	select {
	case id := <-got:
		if id != res.UserID {
			t.Errorf("event payload = %q, want %q", id, res.UserID)
		}
	default:
		t.Error("UserRegistered not published")
	}
}
```

Executor lưu ý: `register_test.go` import `eventbus`; thêm `"github.com/memorix/memorix/internal/platform/eventbus"` vào import của file test.

- [ ] **Step 6: Run tests to verify they fail**

Run: `go test ./internal/identity/service/ -run TestRegister -v`
Expected: FAIL (build error — `Service`/`Register`/ports chưa đủ; nếu đã tạo hết ở step trên thì chạy để chuyển sang PASS).

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/identity/service/ -run TestRegister -v`
Expected: PASS (4 test).

- [ ] **Step 8: Commit**

```bash
git add internal/identity/ports internal/identity/repo/memory internal/identity/service
git commit -m "feat(identity): ports, in-memory repos, and Register use-case (Story 1.2)"
```

---

### Task 10: service — xác thực email (Story 1.3) (TDD)

**Files:**
- Create: `internal/identity/service/verify_email.go`
- Test: `internal/identity/service/verify_email_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/identity/service/verify_email_test.go`:
```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/memorix/memorix/internal/identity/domain"
)

func TestVerifyEmail_SetsVerifiedAndConsumesToken(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{
		Email: "v@example.com", Password: "Tr0ub4dour!",
	})
	if err := h.svc.VerifyEmail(context.Background(), res.VerifyToken); err != nil {
		t.Fatalf("verify: %v", err)
	}
	u, _ := h.stores.Users.ByEmail(context.Background(), "v@example.com")
	if !u.IsVerified() {
		t.Error("email_verified_at must be set")
	}
	// token 1-lần: dùng lại phải fail
	if err := h.svc.VerifyEmail(context.Background(), res.VerifyToken); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("reused token should be invalid, got %v", err)
	}
}

func TestVerifyEmail_BadTokenRejected(t *testing.T) {
	h := newHarness()
	if err := h.svc.VerifyEmail(context.Background(), "does-not-exist"); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestVerifyEmail_ExpiredTokenRejected(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{
		Email: "exp@example.com", Password: "Tr0ub4dour!",
	})
	h.clock.t = h.clock.t.Add(25 * 60 * 60 * 1e9) // +25h > VerifyTTL 24h
	if err := h.svc.VerifyEmail(context.Background(), res.VerifyToken); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("expired verify token should be invalid, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/service/ -run TestVerifyEmail -v`
Expected: FAIL (`VerifyEmail` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/identity/service/verify_email.go`:
```go
package service

import (
	"context"

	"github.com/memorix/memorix/internal/identity/domain"
)

// VerifyEmail tiêu thụ token kind=verify: set email_verified_at, đánh dấu used
// (1-lần, TTL 24h) — Story 1.3.
func (s *Service) VerifyEmail(ctx context.Context, rawToken string) error {
	hash := s.deps.Secrets.Hash(rawToken)
	tok, err := s.deps.Tokens.ByTokenHash(ctx, hash, domain.KindVerify)
	if err != nil {
		return domain.ErrTokenInvalid
	}
	now := s.deps.Clock.Now()
	if !tok.Usable(now) {
		return domain.ErrTokenInvalid
	}
	if err := s.deps.Tokens.MarkUsed(ctx, tok.ID, now); err != nil {
		return err
	}
	u, err := s.deps.Users.ByID(ctx, tok.UserID)
	if err != nil {
		return err
	}
	u.EmailVerifiedAt = &now
	u.UpdatedAt = now
	return s.deps.Users.Update(ctx, u)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/identity/service/ -run TestVerifyEmail -v`
Expected: PASS (3 test).

- [ ] **Step 5: Commit**

```bash
git add internal/identity/service/verify_email.go internal/identity/service/verify_email_test.go
git commit -m "feat(identity): email verification consuming one-time token (Story 1.3)"
```

---

### Task 11: service — đăng nhập + phiên + rate-limit (Story 1.4a) (TDD)

**Files:**
- Create: `internal/identity/service/login.go`
- Test: `internal/identity/service/login_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/identity/service/login_test.go`:
```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/memorix/memorix/internal/identity/domain"
)

func seedUser(t *testing.T, h *harness, email, pw string) {
	t.Helper()
	if _, err := h.svc.Register(context.Background(), RegisterInput{Email: email, Password: pw}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "l@example.com", "Tr0ub4dour!")
	tok, err := h.svc.Login(context.Background(), "L@Example.com", "Tr0ub4dour!")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if tok.AccessToken == "" || tok.RefreshToken == "" {
		t.Error("expected token pair on success")
	}
	if h.limiter.resets == 0 {
		t.Error("limiter must be reset on successful login")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "l@example.com", "Tr0ub4dour!")
	_, err := h.svc.Login(context.Background(), "l@example.com", "wrong-pass99")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_UnknownEmailSameError(t *testing.T) {
	h := newHarness()
	// không tạo user — phải trả CÙNG lỗi ErrInvalidCredentials (chống enumeration)
	_, err := h.svc.Login(context.Background(), "ghost@example.com", "whatever99A")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("unknown email must return ErrInvalidCredentials (no enumeration), got %v", err)
	}
}

func TestLogin_RateLimited(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "l@example.com", "Tr0ub4dour!")
	h.limiter.allow = false // limiter đã chặn
	_, err := h.svc.Login(context.Background(), "l@example.com", "Tr0ub4dour!")
	if !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/service/ -run TestLogin -v`
Expected: FAIL (`Login` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/identity/service/login.go`:
```go
package service

import (
	"context"
	"errors"

	"github.com/memorix/memorix/internal/identity/domain"
)

// Login xác thực email+password, trả token pair (Story 1.4). Rate-limit theo
// email (NFR-10) + constant-time compare khi email không tồn tại (chống
// enumeration): mọi thất bại credential trả CÙNG ErrInvalidCredentials.
func (s *Service) Login(ctx context.Context, email, password string) (TokenPair, error) {
	email = domain.NormalizeEmail(email)
	key := "login:" + email
	if ok, _ := s.deps.Limiter.Allow(ctx, key); !ok {
		return TokenPair{}, domain.ErrRateLimited
	}

	u, err := s.deps.Users.ByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return TokenPair{}, err
		}
		// user không tồn tại: burn thời gian để không lộ qua timing.
		_, _ = s.deps.Hasher.Verify(password, s.dummyHash)
		return TokenPair{}, domain.ErrInvalidCredentials
	}

	ok, err := s.deps.Hasher.Verify(password, u.PasswordHash)
	if err != nil {
		return TokenPair{}, err
	}
	if !ok {
		return TokenPair{}, domain.ErrInvalidCredentials
	}

	s.deps.Limiter.Reset(ctx, key)
	return s.issueSession(ctx, u)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/identity/service/ -run TestLogin -v`
Expected: PASS (4 test).

- [ ] **Step 5: Commit**

```bash
git add internal/identity/service/login.go internal/identity/service/login_test.go
git commit -m "feat(identity): password login with rate-limit and anti-enumeration (Story 1.4)"
```

---

### Task 12: service — refresh rotation + reuse-detection (Story 1.4b) (TDD)

**Files:**
- Create: `internal/identity/service/refresh.go`
- Test: `internal/identity/service/refresh_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/identity/service/refresh_test.go`:
```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/memorix/memorix/internal/identity/domain"
)

func TestRefresh_RotatesAndInvalidatesOld(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "r@example.com", "Tr0ub4dour!")
	first, _ := h.svc.Login(context.Background(), "r@example.com", "Tr0ub4dour!")

	second, err := h.svc.Refresh(context.Background(), first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Error("refresh must rotate to a NEW opaque token")
	}
	if second.AccessToken == "" {
		t.Error("refresh must mint a new access token")
	}
	// token cũ đã bị xoay → dùng lại phải bị coi là reuse
	if _, err := h.svc.Refresh(context.Background(), first.RefreshToken); !errors.Is(err, domain.ErrReuseDetected) {
		t.Errorf("old token reuse must be detected, got %v", err)
	}
}

func TestRefresh_ReuseRevokesWholeFamily(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "r@example.com", "Tr0ub4dour!")
	first, _ := h.svc.Login(context.Background(), "r@example.com", "Tr0ub4dour!")
	second, _ := h.svc.Refresh(context.Background(), first.RefreshToken)

	// tấn công: dùng lại token đã xoay (first) → revoke cả family
	if _, err := h.svc.Refresh(context.Background(), first.RefreshToken); !errors.Is(err, domain.ErrReuseDetected) {
		t.Fatalf("expected reuse detected, got %v", err)
	}
	// hệ quả: token hợp lệ kế tiếp (second) cũng bị vô hiệu do revoke family
	if _, err := h.svc.Refresh(context.Background(), second.RefreshToken); err == nil {
		t.Error("successor token must be revoked after family compromise")
	}
}

func TestRefresh_UnknownTokenInvalid(t *testing.T) {
	h := newHarness()
	if _, err := h.svc.Refresh(context.Background(), "nope"); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestRefresh_ExpiredTokenInvalid(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "r@example.com", "Tr0ub4dour!")
	first, _ := h.svc.Login(context.Background(), "r@example.com", "Tr0ub4dour!")
	h.clock.t = h.clock.t.Add(31 * 24 * 60 * 60 * 1e9) // +31d > RefreshTTL 30d
	if _, err := h.svc.Refresh(context.Background(), first.RefreshToken); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("expired refresh must be invalid, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/service/ -run TestRefresh -v`
Expected: FAIL (`Refresh` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/identity/service/refresh.go`:
```go
package service

import (
	"context"

	"github.com/memorix/memorix/internal/identity/domain"
)

// Refresh xoay vòng refresh token (Story 1.4, AD-11). Token opaque, tra bằng
// hash. Nếu token đã bị revoke (đã xoay) mà bị dùng lại → reuse-detection:
// revoke toàn bộ family. Token hợp lệ → tạo session kế cùng family, đánh dấu
// token cũ rotated.
func (s *Service) Refresh(ctx context.Context, rawToken string) (TokenPair, error) {
	hash := s.deps.Secrets.Hash(rawToken)
	sess, err := s.deps.Sessions.ByTokenHash(ctx, hash)
	if err != nil {
		return TokenPair{}, domain.ErrTokenInvalid
	}
	now := s.deps.Clock.Now()

	if sess.RevokedAt != nil {
		// Token đã xoay/thu hồi mà xuất hiện lại = bị đánh cắp → nuke family.
		_ = s.deps.Sessions.RevokeFamily(ctx, sess.FamilyID, now)
		return TokenPair{}, domain.ErrReuseDetected
	}
	if !now.Before(sess.ExpiresAt) {
		return TokenPair{}, domain.ErrTokenInvalid
	}

	u, err := s.deps.Users.ByID(ctx, sess.UserID)
	if err != nil || u.DeletedAt != nil {
		return TokenPair{}, domain.ErrTokenInvalid
	}

	raw, newHash := s.deps.Secrets.New()
	next := &domain.Session{
		ID:               newID(),
		UserID:           sess.UserID,
		FamilyID:         sess.FamilyID,
		RefreshTokenHash: newHash,
		ExpiresAt:        now.Add(s.deps.RefreshTTL),
		CreatedAt:        now,
	}
	if err := s.deps.Sessions.Create(ctx, next); err != nil {
		return TokenPair{}, err
	}
	if err := s.deps.Sessions.MarkRotated(ctx, sess.ID, next.ID, now); err != nil {
		return TokenPair{}, err
	}
	access, exp, err := s.deps.Issuer.Issue(u.ID, string(u.Role), string(u.Plan))
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, AccessExpiresAt: exp, RefreshToken: raw}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/identity/service/ -run TestRefresh -v`
Expected: PASS (4 test).

- [ ] **Step 5: Commit**

```bash
git add internal/identity/service/refresh.go internal/identity/service/refresh_test.go
git commit -m "feat(identity): refresh token rotation with family reuse-detection (Story 1.4)"
```

---

### Task 13: service — OAuth login (PKCE + id_token, no auto-merge) (Story 1.5) (TDD)

**Files:**
- Create: `internal/identity/service/oauth.go`
- Test: `internal/identity/service/oauth_test.go`

Verify id_token đã xảy ra trong `ports.OIDCVerifier` (adapter thật ở Task 18). Service quyết định link/tạo user và **không tự-merge bằng email chưa verified** (AD-11).

- [ ] **Step 1: Write the failing test**

Create `internal/identity/service/oauth_test.go`:
```go
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/identity/ports"
	"github.com/memorix/memorix/internal/identity/repo/memory"
	"github.com/memorix/memorix/internal/platform/eventbus"
)

// harness với OIDC verifier tùy biến.
func newOIDCHarness(claims ports.OIDCClaims, verr error) *harness {
	clk := &fakeClock{t: time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)}
	lim := &fakeLimiter{allow: true}
	bus := eventbus.NewInProcess()
	st := memory.New()
	svc := New(Deps{
		Users: st.Users, Sessions: st.Sessions, Tokens: st.Tokens, OAuth: st.OAuth,
		Hasher: fakeHasher{}, Issuer: fakeIssuer{now: clk.Now}, Secrets: &fakeSecrets{},
		Clock: clk, Limiter: lim, OIDC: stubOIDC{claims: claims, err: verr}, Bus: bus,
		RefreshTTL: 30 * 24 * time.Hour, VerifyTTL: 24 * time.Hour, ResetTTL: time.Hour,
	})
	return &harness{svc: svc, stores: st, clock: clk, limiter: lim, bus: bus}
}

func TestOAuth_FirstTimeCreatesUserAndLinks(t *testing.T) {
	h := newOIDCHarness(ports.OIDCClaims{ProviderUID: "g-123", Email: "new@example.com", EmailVerified: true}, nil)
	tok, err := h.svc.OAuthLogin(context.Background(), "google", "code", "verifier", "https://app/cb", "nonce")
	if err != nil {
		t.Fatalf("oauth: %v", err)
	}
	if tok.AccessToken == "" {
		t.Error("expected tokens on first oauth login")
	}
	oid, err := h.stores.OAuth.ByProviderUID(context.Background(), "google", "g-123")
	if err != nil {
		t.Fatalf("identity not linked: %v", err)
	}
	u, _ := h.stores.Users.ByID(context.Background(), oid.UserID)
	if !u.IsVerified() {
		t.Error("provider-verified email should mark account verified")
	}
}

func TestOAuth_ExistingLinkReused(t *testing.T) {
	h := newOIDCHarness(ports.OIDCClaims{ProviderUID: "g-123", Email: "x@example.com", EmailVerified: true}, nil)
	first, _ := h.svc.OAuthLogin(context.Background(), "google", "c", "v", "cb", "n")
	_ = first
	second, err := h.svc.OAuthLogin(context.Background(), "google", "c2", "v2", "cb", "n2")
	if err != nil {
		t.Fatalf("second oauth: %v", err)
	}
	if second.AccessToken == "" {
		t.Error("relogin via linked identity should succeed")
	}
	// vẫn chỉ 1 user
	if _, err := h.stores.Users.ByEmail(context.Background(), "x@example.com"); err != nil {
		t.Errorf("user should persist: %v", err)
	}
}

func TestOAuth_NoMergeOnUnverifiedEmail(t *testing.T) {
	// user email/password đã tồn tại nhưng chưa verify; provider trả cùng email
	h := newOIDCHarness(ports.OIDCClaims{ProviderUID: "g-999", Email: "dup@example.com", EmailVerified: true}, nil)
	seedUser(t, h, "dup@example.com", "Tr0ub4dour!") // account CHƯA verify
	_, err := h.svc.OAuthLogin(context.Background(), "google", "c", "v", "cb", "n")
	if !errors.Is(err, domain.ErrOAuthNoMerge) {
		t.Errorf("must not auto-merge into unverified account, got %v", err)
	}
}

func TestOAuth_VerifierFailure(t *testing.T) {
	h := newOIDCHarness(ports.OIDCClaims{}, errors.New("bad signature"))
	if _, err := h.svc.OAuthLogin(context.Background(), "google", "c", "v", "cb", "n"); !errors.Is(err, domain.ErrOAuthFailed) {
		t.Errorf("expected ErrOAuthFailed, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/service/ -run TestOAuth -v`
Expected: FAIL (`OAuthLogin` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/identity/service/oauth.go`:
```go
package service

import (
	"context"
	"errors"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/platform/eventbus"
)

// OAuthLogin xử lý Authorization Code + PKCE sau khi verifier đã xác minh
// id_token (sig/aud/iss/nonce) — Story 1.5, AD-11.
//   - provider_uid đã link → đăng nhập user đó.
//   - chưa link + email trùng user hiện có → CHỈ link khi cả provider-email
//     verified LẪN account đã verified; ngược lại từ chối (không auto-merge).
//   - chưa link + email mới → tạo user mới (+ link), phát UserRegistered.
func (s *Service) OAuthLogin(ctx context.Context, provider, code, codeVerifier, redirectURI, nonce string) (TokenPair, error) {
	claims, err := s.deps.OIDC.Verify(ctx, provider, code, codeVerifier, redirectURI, nonce)
	if err != nil {
		return TokenPair{}, domain.ErrOAuthFailed
	}
	now := s.deps.Clock.Now()

	if oid, err := s.deps.OAuth.ByProviderUID(ctx, provider, claims.ProviderUID); err == nil {
		u, err := s.deps.Users.ByID(ctx, oid.UserID)
		if err != nil {
			return TokenPair{}, err
		}
		return s.issueSession(ctx, u)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return TokenPair{}, err
	}

	email := domain.NormalizeEmail(claims.Email)
	existing, err := s.deps.Users.ByEmail(ctx, email)
	switch {
	case err == nil:
		if !claims.EmailVerified || !existing.IsVerified() {
			return TokenPair{}, domain.ErrOAuthNoMerge
		}
		if err := s.deps.OAuth.Create(ctx, &domain.OAuthIdentity{
			ID: newID(), UserID: existing.ID, Provider: provider,
			ProviderUID: claims.ProviderUID, CreatedAt: now,
		}); err != nil {
			return TokenPair{}, err
		}
		return s.issueSession(ctx, existing)

	case errors.Is(err, domain.ErrNotFound):
		u := &domain.User{
			ID: newID(), Email: email, Timezone: "UTC", Locale: "vi", Theme: "system",
			Plan: domain.PlanFree, Role: domain.RoleUser, CreatedAt: now, UpdatedAt: now,
		}
		if claims.EmailVerified {
			u.EmailVerifiedAt = &now
		}
		if err := s.deps.Users.Create(ctx, u); err != nil {
			return TokenPair{}, err
		}
		if err := s.deps.OAuth.Create(ctx, &domain.OAuthIdentity{
			ID: newID(), UserID: u.ID, Provider: provider,
			ProviderUID: claims.ProviderUID, CreatedAt: now,
		}); err != nil {
			return TokenPair{}, err
		}
		s.deps.Bus.Publish(ctx, eventbus.Event{Name: "UserRegistered", Payload: u.ID})
		return s.issueSession(ctx, u)

	default:
		return TokenPair{}, err
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/identity/service/ -run TestOAuth -v`
Expected: PASS (4 test).

- [ ] **Step 5: Commit**

```bash
git add internal/identity/service/oauth.go internal/identity/service/oauth_test.go
git commit -m "feat(identity): OAuth login with id_token verify and no unverified auto-merge (Story 1.5)"
```

---

### Task 14: service — đặt lại mật khẩu (Story 1.6) (TDD)

**Files:**
- Create: `internal/identity/service/reset.go`
- Test: `internal/identity/service/reset_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/identity/service/reset_test.go`:
```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/memorix/memorix/internal/identity/domain"
)

func TestRequestReset_KnownEmailIssuesToken(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "p@example.com", "Tr0ub4dour!")
	raw, err := h.svc.RequestReset(context.Background(), "P@Example.com")
	if err != nil {
		t.Fatalf("request reset: %v", err)
	}
	if raw == "" {
		t.Error("known email must produce a reset token to email")
	}
}

func TestRequestReset_UnknownEmailNoError(t *testing.T) {
	h := newHarness()
	raw, err := h.svc.RequestReset(context.Background(), "ghost@example.com")
	if err != nil {
		t.Fatalf("unknown email must not error (no enumeration): %v", err)
	}
	if raw != "" {
		t.Error("unknown email must not produce a token")
	}
}

func TestResetPassword_UpdatesAndRevokesSessions(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "p@example.com", "Tr0ub4dour!")
	login, _ := h.svc.Login(context.Background(), "p@example.com", "Tr0ub4dour!")
	raw, _ := h.svc.RequestReset(context.Background(), "p@example.com")

	if err := h.svc.ResetPassword(context.Background(), raw, "N3wStr0ng!Pass"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	// mật khẩu mới dùng được
	if _, err := h.svc.Login(context.Background(), "p@example.com", "N3wStr0ng!Pass"); err != nil {
		t.Errorf("login with new password failed: %v", err)
	}
	// mọi session cũ bị thu hồi
	if _, err := h.svc.Refresh(context.Background(), login.RefreshToken); err == nil {
		t.Error("existing sessions must be revoked after reset (Story 1.6)")
	}
	// token 1-lần
	if err := h.svc.ResetPassword(context.Background(), raw, "An0ther!Pass9"); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("reused reset token must be invalid, got %v", err)
	}
}

func TestResetPassword_WeakRejected(t *testing.T) {
	h := newHarness()
	seedUser(t, h, "p@example.com", "Tr0ub4dour!")
	raw, _ := h.svc.RequestReset(context.Background(), "p@example.com")
	if err := h.svc.ResetPassword(context.Background(), raw, "weak"); !errors.Is(err, domain.ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/service/ -run "TestRequestReset|TestResetPassword" -v`
Expected: FAIL (`RequestReset`/`ResetPassword` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/identity/service/reset.go`:
```go
package service

import (
	"context"
	"errors"

	"github.com/memorix/memorix/internal/identity/domain"
)

// RequestReset phát token kind=reset (TTL 1h). Response phía handler GIỐNG NHAU
// dù email tồn tại hay không (Story 1.6, chống enumeration); raw chỉ để gửi mail.
func (s *Service) RequestReset(ctx context.Context, email string) (string, error) {
	email = domain.NormalizeEmail(email)
	u, err := s.deps.Users.ByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	now := s.deps.Clock.Now()
	raw, hash := s.deps.Secrets.New()
	tok := &domain.EmailToken{
		ID: newID(), UserID: u.ID, Kind: domain.KindReset,
		TokenHash: hash, ExpiresAt: now.Add(s.deps.ResetTTL), CreatedAt: now,
	}
	if err := s.deps.Tokens.Create(ctx, tok); err != nil {
		return "", err
	}
	return raw, nil
}

// ResetPassword đặt mật khẩu mới, tiêu thụ token, và THU HỒI mọi session (Story 1.6).
func (s *Service) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	if !domain.PasswordStrongEnough(newPassword) {
		return domain.ErrWeakPassword
	}
	hash := s.deps.Secrets.Hash(rawToken)
	tok, err := s.deps.Tokens.ByTokenHash(ctx, hash, domain.KindReset)
	if err != nil {
		return domain.ErrTokenInvalid
	}
	now := s.deps.Clock.Now()
	if !tok.Usable(now) {
		return domain.ErrTokenInvalid
	}
	ph, err := s.deps.Hasher.Hash(newPassword)
	if err != nil {
		return err
	}
	u, err := s.deps.Users.ByID(ctx, tok.UserID)
	if err != nil {
		return err
	}
	u.PasswordHash = ph
	u.UpdatedAt = now
	if err := s.deps.Users.Update(ctx, u); err != nil {
		return err
	}
	if err := s.deps.Tokens.MarkUsed(ctx, tok.ID, now); err != nil {
		return err
	}
	return s.deps.Sessions.RevokeAllForUser(ctx, u.ID, now)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/identity/service/ -run "TestRequestReset|TestResetPassword" -v`
Expected: PASS (4 test).

- [ ] **Step 5: Commit**

```bash
git add internal/identity/service/reset.go internal/identity/service/reset_test.go
git commit -m "feat(identity): password reset revoking all sessions (Story 1.6)"
```

---

### Task 15: service — cập nhật hồ sơ (Story 1.7) (TDD)

**Files:**
- Create: `internal/identity/service/profile.go`
- Test: `internal/identity/service/profile_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/identity/service/profile_test.go`:
```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/memorix/memorix/internal/identity/domain"
)

func ptr(s string) *string { return &s }

func TestUpdateProfile_AppliesFields(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{Email: "u@example.com", Password: "Tr0ub4dour!"})
	u, err := h.svc.UpdateProfile(context.Background(), res.UserID, ProfileInput{
		DisplayName: ptr("Minh"), Timezone: ptr("Asia/Ho_Chi_Minh"),
		Locale: ptr("en"), Theme: ptr("dark"),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if u.DisplayName != "Minh" || u.Timezone != "Asia/Ho_Chi_Minh" || u.Locale != "en" || u.Theme != "dark" {
		t.Errorf("profile not applied: %+v", u)
	}
	// persisted
	got, _ := h.stores.Users.ByID(context.Background(), res.UserID)
	if got.Timezone != "Asia/Ho_Chi_Minh" {
		t.Error("timezone not persisted (needed for study-day AD-12)")
	}
}

func TestUpdateProfile_PartialUpdate(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{Email: "u@example.com", Password: "Tr0ub4dour!"})
	u, err := h.svc.UpdateProfile(context.Background(), res.UserID, ProfileInput{Theme: ptr("light")})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if u.Theme != "light" || u.Locale != "vi" {
		t.Errorf("partial update should keep defaults: %+v", u)
	}
}

func TestUpdateProfile_InvalidValuesRejected(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{Email: "u@example.com", Password: "Tr0ub4dour!"})
	for _, in := range []ProfileInput{
		{Timezone: ptr("Mars/Phobos")},
		{Locale: ptr("fr")},
		{Theme: ptr("neon")},
	} {
		if _, err := h.svc.UpdateProfile(context.Background(), res.UserID, in); !errors.Is(err, domain.ErrInvalidProfile) {
			t.Errorf("expected ErrInvalidProfile for %+v, got %v", in, err)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/service/ -run TestUpdateProfile -v`
Expected: FAIL (`UpdateProfile`/`ProfileInput` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/identity/service/profile.go`:
```go
package service

import (
	"context"

	"github.com/memorix/memorix/internal/identity/domain"
)

// ProfileInput: field nil = giữ nguyên (partial update).
type ProfileInput struct {
	DisplayName *string
	Timezone    *string
	Locale      *string
	Theme       *string
}

// UpdateProfile cập nhật tên/múi giờ/ngôn ngữ/theme (Story 1.7). Timezone dùng
// cho "ngày học" downstream (AD-12); validate whitelist deny-by-default.
func (s *Service) UpdateProfile(ctx context.Context, userID string, in ProfileInput) (*domain.User, error) {
	u, err := s.deps.Users.ByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if in.DisplayName != nil {
		u.DisplayName = *in.DisplayName
	}
	if in.Timezone != nil {
		if !domain.ValidTimezone(*in.Timezone) {
			return nil, domain.ErrInvalidProfile
		}
		u.Timezone = *in.Timezone
	}
	if in.Locale != nil {
		if *in.Locale != "vi" && *in.Locale != "en" {
			return nil, domain.ErrInvalidProfile
		}
		u.Locale = *in.Locale
	}
	if in.Theme != nil {
		switch *in.Theme {
		case "light", "dark", "system":
			u.Theme = *in.Theme
		default:
			return nil, domain.ErrInvalidProfile
		}
	}
	u.UpdatedAt = s.deps.Clock.Now()
	if err := s.deps.Users.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/identity/service/ -run TestUpdateProfile -v`
Expected: PASS (3 test).

- [ ] **Step 5: Commit**

```bash
git add internal/identity/service/profile.go internal/identity/service/profile_test.go
git commit -m "feat(identity): profile update with timezone/locale/theme validation (Story 1.7)"
```

---

### Task 16: service — GDPR export + xóa tài khoản (Story 1.8) (TDD)

**Files:**
- Create: `internal/identity/service/gdpr.go`
- Test: `internal/identity/service/gdpr_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/identity/service/gdpr_test.go`:
```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/memorix/memorix/internal/identity/domain"
)

func TestExportData_RequiresReauth(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{
		Email: "g@example.com", Password: "Tr0ub4dour!", DisplayName: "G",
	})
	// sai mật khẩu → từ chối (re-auth, NFR-14)
	if _, err := h.svc.ExportData(context.Background(), res.UserID, "wrong-pass"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("export must re-auth, got %v", err)
	}
	exp, err := h.svc.ExportData(context.Background(), res.UserID, "Tr0ub4dour!")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if exp.User.Email != "g@example.com" || exp.User.DisplayName != "G" {
		t.Errorf("export payload incomplete: %+v", exp.User)
	}
	if exp.ExportedAt.IsZero() {
		t.Error("export must be timestamped")
	}
}

func TestDeleteAccount_SoftDeletesAndRevokes(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{Email: "d@example.com", Password: "Tr0ub4dour!"})
	login, _ := h.svc.Login(context.Background(), "d@example.com", "Tr0ub4dour!")

	if err := h.svc.DeleteAccount(context.Background(), res.UserID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// user biến khỏi lookup theo email (soft-delete)
	if _, err := h.stores.Users.ByEmail(context.Background(), "d@example.com"); !errors.Is(err, domain.ErrNotFound) {
		t.Error("soft-deleted user must not resolve by email")
	}
	// session bị thu hồi
	if _, err := h.svc.Refresh(context.Background(), login.RefreshToken); err == nil {
		t.Error("sessions must be revoked on account deletion (Story 1.8)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/service/ -run "TestExportData|TestDeleteAccount" -v`
Expected: FAIL (`ExportData`/`DeleteAccount` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/identity/service/gdpr.go`:
```go
package service

import (
	"context"
	"time"

	"github.com/memorix/memorix/internal/identity/domain"
)

// ExportUser là bản chụp dữ liệu identity của user cho GDPR export (JSON).
type ExportUser struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	DisplayName     string     `json:"display_name"`
	Timezone        string     `json:"timezone"`
	Locale          string     `json:"locale"`
	Theme           string     `json:"theme"`
	Plan            string     `json:"plan"`
	Role            string     `json:"role"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Export struct {
	User       ExportUser `json:"user"`
	ExportedAt time.Time  `json:"exported_at"`
}

// ExportData trả toàn bộ dữ liệu identity sau khi re-auth bằng mật khẩu (Story
// 1.8, NFR-14). Dữ liệu module khác (vocabulary/review...) ghép qua port của
// chúng ở V1 — MVP export identity scope.
func (s *Service) ExportData(ctx context.Context, userID, password string) (Export, error) {
	u, err := s.deps.Users.ByID(ctx, userID)
	if err != nil {
		return Export{}, err
	}
	ok, err := s.deps.Hasher.Verify(password, u.PasswordHash)
	if err != nil {
		return Export{}, err
	}
	if !ok {
		return Export{}, domain.ErrInvalidCredentials
	}
	return Export{
		User: ExportUser{
			ID: u.ID, Email: u.Email, DisplayName: u.DisplayName,
			Timezone: u.Timezone, Locale: u.Locale, Theme: u.Theme,
			Plan: string(u.Plan), Role: string(u.Role),
			EmailVerifiedAt: u.EmailVerifiedAt, CreatedAt: u.CreatedAt,
		},
		ExportedAt: s.deps.Clock.Now(),
	}, nil
}

// DeleteAccount soft-delete tài khoản + thu hồi mọi session ngay (Story 1.8).
// Purge cứng theo lịch do worker (Task 20). Không log PII/token.
func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	now := s.deps.Clock.Now()
	if err := s.deps.Users.SoftDelete(ctx, userID, now); err != nil {
		return err
	}
	return s.deps.Sessions.RevokeAllForUser(ctx, userID, now)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/identity/service/ -run "TestExportData|TestDeleteAccount" -v`
Expected: PASS (2 test).

- [ ] **Step 5: Commit**

```bash
git add internal/identity/service/gdpr.go internal/identity/service/gdpr_test.go
git commit -m "feat(identity): GDPR export with re-auth and account soft-delete (Story 1.8)"
```

---

### Task 17: repo — pgx implementations + integration test (TDD)

**Files:**
- Create: `internal/identity/repo/pg.go`
- Test: `internal/identity/repo/pg_test.go` (testcontainers, reuse `startPG` từ Task 2)

Repo dùng pgx trực tiếp (SQL tường minh). Nullable columns scan qua con trỏ. Chỉ FK trong schema `identity` (AD-10).

- [ ] **Step 1: Write pgx repositories**

Create `internal/identity/repo/pg.go`:
```go
package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/identity/ports"
)

// Repos gom 4 repo pgx cho module identity.
type Repos struct {
	Users    *UserRepo
	Sessions *SessionRepo
	Tokens   *EmailTokenRepo
	OAuth    *OAuthRepo
}

func New(pool *pgxpool.Pool) *Repos {
	return &Repos{
		Users:    &UserRepo{pool: pool},
		Sessions: &SessionRepo{pool: pool},
		Tokens:   &EmailTokenRepo{pool: pool},
		OAuth:    &OAuthRepo{pool: pool},
	}
}

func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// --- Users ---

type UserRepo struct{ pool *pgxpool.Pool }

const userCols = `id, email, password_hash, display_name, timezone, locale, theme,
	email_verified_at, plan, role, created_at, updated_at, deleted_at`

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	var plan, role string
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.Timezone,
		&u.Locale, &u.Theme, &u.EmailVerifiedAt, &plan, &role,
		&u.CreatedAt, &u.UpdatedAt, &u.DeletedAt); err != nil {
		return nil, mapNotFound(err)
	}
	u.Plan = domain.Plan(plan)
	u.Role = domain.Role(role)
	return &u, nil
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO identity.users
		(id, email, password_hash, display_name, timezone, locale, theme,
		 email_verified_at, plan, role, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		u.ID, u.Email, u.PasswordHash, u.DisplayName, u.Timezone, u.Locale, u.Theme,
		u.EmailVerifiedAt, string(u.Plan), string(u.Role), u.CreatedAt, u.UpdatedAt)
	return err
}

func (r *UserRepo) ByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userCols+`
		FROM identity.users WHERE email = $1 AND deleted_at IS NULL`, email)
	return scanUser(row)
}

func (r *UserRepo) ByID(ctx context.Context, id string) (*domain.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM identity.users WHERE id = $1`, id)
	return scanUser(row)
}

func (r *UserRepo) Update(ctx context.Context, u *domain.User) error {
	ct, err := r.pool.Exec(ctx, `UPDATE identity.users SET
		email=$2, password_hash=$3, display_name=$4, timezone=$5, locale=$6,
		theme=$7, email_verified_at=$8, plan=$9, role=$10, updated_at=$11
		WHERE id=$1`,
		u.ID, u.Email, u.PasswordHash, u.DisplayName, u.Timezone, u.Locale, u.Theme,
		u.EmailVerifiedAt, string(u.Plan), string(u.Role), u.UpdatedAt)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepo) SoftDelete(ctx context.Context, id string, at time.Time) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE identity.users SET deleted_at=$2, updated_at=$2 WHERE id=$1 AND deleted_at IS NULL`, id, at)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepo) PurgeDeletedBefore(ctx context.Context, cutoff time.Time) (int, error) {
	ct, err := r.pool.Exec(ctx,
		`DELETE FROM identity.users WHERE deleted_at IS NOT NULL AND deleted_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return int(ct.RowsAffected()), nil
}

// --- Sessions ---

type SessionRepo struct{ pool *pgxpool.Pool }

func (r *SessionRepo) Create(ctx context.Context, s *domain.Session) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO identity.sessions
		(id, user_id, family_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		s.ID, s.UserID, s.FamilyID, s.RefreshTokenHash, s.ExpiresAt, s.CreatedAt)
	return err
}

func (r *SessionRepo) ByTokenHash(ctx context.Context, hash string) (*domain.Session, error) {
	var s domain.Session
	err := r.pool.QueryRow(ctx, `SELECT id, user_id, family_id, refresh_token_hash,
		rotated_to, expires_at, revoked_at, created_at
		FROM identity.sessions WHERE refresh_token_hash = $1`, hash).
		Scan(&s.ID, &s.UserID, &s.FamilyID, &s.RefreshTokenHash,
			&s.RotatedTo, &s.ExpiresAt, &s.RevokedAt, &s.CreatedAt)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &s, nil
}

func (r *SessionRepo) MarkRotated(ctx context.Context, id, successorID string, at time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE identity.sessions SET rotated_to=$2, revoked_at=$3 WHERE id=$1`, id, successorID, at)
	return err
}

func (r *SessionRepo) RevokeFamily(ctx context.Context, familyID string, at time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE identity.sessions SET revoked_at=$2 WHERE family_id=$1 AND revoked_at IS NULL`, familyID, at)
	return err
}

func (r *SessionRepo) RevokeAllForUser(ctx context.Context, userID string, at time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE identity.sessions SET revoked_at=$2 WHERE user_id=$1 AND revoked_at IS NULL`, userID, at)
	return err
}

// --- Email tokens ---

type EmailTokenRepo struct{ pool *pgxpool.Pool }

func (r *EmailTokenRepo) Create(ctx context.Context, t *domain.EmailToken) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO identity.email_tokens
		(id, user_id, kind, token_hash, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		t.ID, t.UserID, string(t.Kind), t.TokenHash, t.ExpiresAt, t.CreatedAt)
	return err
}

func (r *EmailTokenRepo) ByTokenHash(ctx context.Context, hash string, kind domain.TokenKind) (*domain.EmailToken, error) {
	var t domain.EmailToken
	var k string
	err := r.pool.QueryRow(ctx, `SELECT id, user_id, kind, token_hash, expires_at, used_at, created_at
		FROM identity.email_tokens WHERE token_hash=$1 AND kind=$2`, hash, string(kind)).
		Scan(&t.ID, &t.UserID, &k, &t.TokenHash, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if err != nil {
		return nil, mapNotFound(err)
	}
	t.Kind = domain.TokenKind(k)
	return &t, nil
}

func (r *EmailTokenRepo) MarkUsed(ctx context.Context, id string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE identity.email_tokens SET used_at=$2 WHERE id=$1`, id, at)
	return err
}

// --- OAuth identities ---

type OAuthRepo struct{ pool *pgxpool.Pool }

func (r *OAuthRepo) ByProviderUID(ctx context.Context, provider, uid string) (*domain.OAuthIdentity, error) {
	var o domain.OAuthIdentity
	err := r.pool.QueryRow(ctx, `SELECT id, user_id, provider, provider_uid, created_at
		FROM identity.oauth_identities WHERE provider=$1 AND provider_uid=$2`, provider, uid).
		Scan(&o.ID, &o.UserID, &o.Provider, &o.ProviderUID, &o.CreatedAt)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &o, nil
}

func (r *OAuthRepo) Create(ctx context.Context, o *domain.OAuthIdentity) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO identity.oauth_identities
		(id, user_id, provider, provider_uid, created_at) VALUES ($1,$2,$3,$4,$5)`,
		o.ID, o.UserID, o.Provider, o.ProviderUID, o.CreatedAt)
	return err
}

// Compile-time checks.
var (
	_ ports.UserRepo       = (*UserRepo)(nil)
	_ ports.SessionRepo    = (*SessionRepo)(nil)
	_ ports.EmailTokenRepo = (*EmailTokenRepo)(nil)
	_ ports.OAuthRepo      = (*OAuthRepo)(nil)
)
```

- [ ] **Step 2: Write the failing integration test**

Create `internal/identity/repo/pg_test.go`:
```go
package repo

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/google/uuid"
)

func TestPgRepos_UserRoundTripAndSessionRotation(t *testing.T) {
	dsn := startPG(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	repos := New(pool)

	now := time.Now().UTC().Truncate(time.Second)
	u := &domain.User{
		ID: uuid.NewString(), Email: "int@example.com", PasswordHash: "h:x",
		Timezone: "UTC", Locale: "vi", Theme: "system",
		Plan: domain.PlanFree, Role: domain.RoleUser, CreatedAt: now, UpdatedAt: now,
	}
	if err := repos.Users.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	got, err := repos.Users.ByEmail(ctx, "INT@example.com") // citext case-insensitive
	if err != nil || got.ID != u.ID {
		t.Fatalf("ByEmail citext: got=%+v err=%v", got, err)
	}

	// email verified update
	got.EmailVerifiedAt = &now
	got.UpdatedAt = now
	if err := repos.Users.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reloaded, _ := repos.Users.ByID(ctx, u.ID)
	if !reloaded.IsVerified() {
		t.Error("email_verified_at not persisted")
	}

	// session create + rotate
	fam := uuid.NewString()
	s1 := &domain.Session{ID: uuid.NewString(), UserID: u.ID, FamilyID: fam,
		RefreshTokenHash: "hash-1", ExpiresAt: now.Add(time.Hour), CreatedAt: now}
	s2 := &domain.Session{ID: uuid.NewString(), UserID: u.ID, FamilyID: fam,
		RefreshTokenHash: "hash-2", ExpiresAt: now.Add(time.Hour), CreatedAt: now}
	if err := repos.Sessions.Create(ctx, s1); err != nil {
		t.Fatalf("session1: %v", err)
	}
	if err := repos.Sessions.Create(ctx, s2); err != nil {
		t.Fatalf("session2: %v", err)
	}
	if err := repos.Sessions.MarkRotated(ctx, s1.ID, s2.ID, now); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	back, _ := repos.Sessions.ByTokenHash(ctx, "hash-1")
	if back.RevokedAt == nil || back.RotatedTo == nil || *back.RotatedTo != s2.ID {
		t.Errorf("rotation not persisted: %+v", back)
	}
	// revoke family → s2 revoked
	if err := repos.Sessions.RevokeFamily(ctx, fam, now); err != nil {
		t.Fatalf("revoke family: %v", err)
	}
	s2back, _ := repos.Sessions.ByTokenHash(ctx, "hash-2")
	if s2back.RevokedAt == nil {
		t.Error("family revoke did not reach successor session")
	}
}

func TestPgRepos_EmailTokenAndPurge(t *testing.T) {
	dsn := startPG(t)
	ctx := context.Background()
	pool, _ := pgxpool.New(ctx, dsn)
	defer pool.Close()
	repos := New(pool)

	now := time.Now().UTC().Truncate(time.Second)
	u := &domain.User{ID: uuid.NewString(), Email: "tok@example.com",
		Timezone: "UTC", Locale: "vi", Theme: "system",
		Plan: domain.PlanFree, Role: domain.RoleUser, CreatedAt: now, UpdatedAt: now}
	_ = repos.Users.Create(ctx, u)

	tok := &domain.EmailToken{ID: uuid.NewString(), UserID: u.ID, Kind: domain.KindVerify,
		TokenHash: "th-1", ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now}
	if err := repos.Tokens.Create(ctx, tok); err != nil {
		t.Fatalf("token create: %v", err)
	}
	got, err := repos.Tokens.ByTokenHash(ctx, "th-1", domain.KindVerify)
	if err != nil || got.ID != tok.ID {
		t.Fatalf("token lookup: %v", err)
	}
	if err := repos.Tokens.MarkUsed(ctx, tok.ID, now); err != nil {
		t.Fatalf("mark used: %v", err)
	}

	// purge: soft-delete rồi purge trước cutoff
	_ = repos.Users.SoftDelete(ctx, u.ID, now.Add(-48*time.Hour))
	n, err := repos.Users.PurgeDeletedBefore(ctx, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 purged, got %d", n)
	}
}
```

- [ ] **Step 3: Run test to verify it fails then passes**

Run: `go test ./internal/identity/repo/ -run TestPgRepos -v`
Expected: FAIL trước khi tạo `pg.go` (undefined `New`); sau khi tạo → PASS (2 test, cần Docker).

- [ ] **Step 4: Commit**

```bash
git add internal/identity/repo/pg.go internal/identity/repo/pg_test.go
git commit -m "feat(identity): pgx repositories with integration tests (testcontainers)"
```

---

### Task 18: OAuth adapter — go-oidc + oauth2 PKCE (Story 1.5) (TDD)

**Files:**
- Create: `internal/identity/oauthx/verifier.go`
- Test: `internal/identity/oauthx/verifier_test.go`

Driven adapter implement `ports.OIDCVerifier`: đổi code lấy id_token (PKCE), verify chữ ký/aud/iss/nonce qua go-oidc. Đặt trong module identity (được import `ports`), Wire ở `cmd/api`. Phần discovery/JWKS cần mạng → kiểm ở staging; unit test phủ sinh PKCE + AuthURL (thuần, không mạng).

- [ ] **Step 1: Write the failing test**

Create `internal/identity/oauthx/verifier_test.go`:
```go
package oauthx

import (
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestBeginAuth_BuildsPKCEAuthURL(t *testing.T) {
	v := &Verifier{providers: map[string]*provider{
		"google": {oauth2: &oauth2.Config{
			ClientID:    "client-x",
			Endpoint:    oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth"},
			RedirectURL: "https://app/cb",
			Scopes:      []string{"openid", "email"},
		}},
	}}
	req, err := v.BeginAuth("google")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if req.State == "" || req.Nonce == "" || req.Verifier == "" {
		t.Fatal("state/nonce/verifier must be generated")
	}
	u, err := url.Parse(req.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	q := u.Query()
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		t.Error("auth URL must carry S256 PKCE challenge")
	}
	if q.Get("state") != req.State || q.Get("nonce") != req.Nonce {
		t.Error("auth URL must carry state and nonce")
	}
	if !strings.Contains(req.URL, "client-x") {
		t.Error("auth URL must carry client id")
	}
}

func TestBeginAuth_UnknownProvider(t *testing.T) {
	v := &Verifier{providers: map[string]*provider{}}
	if _, err := v.BeginAuth("apple"); err == nil {
		t.Error("unknown provider must error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/oauthx/ -v`
Expected: FAIL (`Verifier`/`provider`/`BeginAuth` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/identity/oauthx/verifier.go`:
```go
package oauthx

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/memorix/memorix/internal/identity/ports"
	"golang.org/x/oauth2"
)

type provider struct {
	oauth2   *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// Verifier implements ports.OIDCVerifier cho nhiều provider (google, apple).
type Verifier struct {
	providers map[string]*provider
}

// ProviderConfig cấu hình 1 provider OIDC.
type ProviderConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// New khám phá OIDC discovery cho từng provider (cần mạng — gọi lúc bootstrap).
func New(ctx context.Context, cfgs map[string]ProviderConfig) (*Verifier, error) {
	v := &Verifier{providers: map[string]*provider{}}
	for name, c := range cfgs {
		op, err := oidc.NewProvider(ctx, c.IssuerURL)
		if err != nil {
			return nil, fmt.Errorf("oidc discovery %s: %w", name, err)
		}
		scopes := c.Scopes
		if len(scopes) == 0 {
			scopes = []string{oidc.ScopeOpenID, "email", "profile"}
		}
		v.providers[name] = &provider{
			oauth2: &oauth2.Config{
				ClientID:     c.ClientID,
				ClientSecret: c.ClientSecret,
				RedirectURL:  c.RedirectURL,
				Endpoint:     op.Endpoint(),
				Scopes:       scopes,
			},
			verifier: op.Verifier(&oidc.Config{ClientID: c.ClientID}),
		}
	}
	return v, nil
}

// AuthRequest là dữ liệu 1 lần server lưu (state/nonce/verifier) để verify callback.
type AuthRequest struct {
	URL      string
	State    string
	Nonce    string
	Verifier string
}

// BeginAuth sinh PKCE verifier + state + nonce và AuthURL (Authorization Code + PKCE).
func (v *Verifier) BeginAuth(provider string) (AuthRequest, error) {
	p, ok := v.providers[provider]
	if !ok {
		return AuthRequest{}, fmt.Errorf("oauthx: unknown provider %q", provider)
	}
	codeVerifier := oauth2.GenerateVerifier()
	state := randString()
	nonce := randString()
	url := p.oauth2.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(codeVerifier),
		oidc.Nonce(nonce),
	)
	return AuthRequest{URL: url, State: state, Nonce: nonce, Verifier: codeVerifier}, nil
}

// Verify đổi code (PKCE) lấy token, xác minh id_token (sig/aud/iss) + nonce (AD-11).
func (v *Verifier) Verify(ctx context.Context, provider, code, codeVerifier, redirectURI, nonce string) (ports.OIDCClaims, error) {
	p, ok := v.providers[provider]
	if !ok {
		return ports.OIDCClaims{}, fmt.Errorf("oauthx: unknown provider %q", provider)
	}
	oauthTok, err := p.oauth2.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return ports.OIDCClaims{}, fmt.Errorf("oauthx: code exchange: %w", err)
	}
	rawID, ok := oauthTok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return ports.OIDCClaims{}, fmt.Errorf("oauthx: missing id_token")
	}
	idTok, err := p.verifier.Verify(ctx, rawID)
	if err != nil {
		return ports.OIDCClaims{}, fmt.Errorf("oauthx: id_token verify: %w", err)
	}
	if idTok.Nonce != nonce {
		return ports.OIDCClaims{}, fmt.Errorf("oauthx: nonce mismatch")
	}
	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idTok.Claims(&claims); err != nil {
		return ports.OIDCClaims{}, fmt.Errorf("oauthx: claims: %w", err)
	}
	return ports.OIDCClaims{
		ProviderUID:   claims.Sub,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
	}, nil
}

func randString() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic("oauthx: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

var _ ports.OIDCVerifier = (*Verifier)(nil)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/identity/oauthx/ -v`
Expected: PASS (2 test — không cần mạng vì chỉ test BeginAuth).

- [ ] **Step 5: Commit**

```bash
git add internal/identity/oauthx/
git commit -m "feat(identity): OIDC verifier adapter with PKCE Authorization Code flow (Story 1.5)"
```

---

### Task 19: handler — Gin routes, error mapping, IdentityPort, wire cmd/api (TDD)

**Files:**
- Create: `internal/identity/mailer/log.go`
- Create: `internal/identity/service/identity_port.go`
- Create: `internal/identity/handler/handler.go`, `internal/identity/handler/errors.go`
- Test: `internal/identity/handler/handler_test.go`
- Edit: `internal/platform/config/config.go` (thêm JWT/TTL/OAuth field)
- Edit: `cmd/api/main.go` (Wire module identity)

- [ ] **Step 1: Mailer log adapter (MVP không gửi mail thật; log, scrub token)**

Create `internal/identity/mailer/log.go`:
```go
package mailer

import (
	"context"
	"log/slog"
)

// LogMailer ghi log sự kiện gửi mail (MVP). KHÔNG log raw token (NFR-14) — chỉ
// độ dài để chẩn đoán. Thay bằng SMTP/provider adapter ở prod.
type LogMailer struct{ log *slog.Logger }

func NewLogMailer(log *slog.Logger) *LogMailer { return &LogMailer{log: log} }

func (m *LogMailer) SendVerification(_ context.Context, email, rawToken string) error {
	m.log.Info("send verification email", "email", email, "token_len", len(rawToken))
	return nil
}

func (m *LogMailer) SendPasswordReset(_ context.Context, email, rawToken string) error {
	m.log.Info("send password reset email", "email", email, "token_len", len(rawToken))
	return nil
}
```

- [ ] **Step 2: IdentityPort implementation (expose ra module khác — AD-1)**

Create `internal/identity/service/identity_port.go`:
```go
package service

import (
	"context"
	"errors"

	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/identity/ports"
)

// Port implements ports.IdentityPort — mặt tiền để module khác (vd scheduling
// cần TZ user tính "ngày học" AD-12) hỏi identity mà không chạm bảng của nó.
type Port struct{ users ports.UserRepo }

func NewPort(users ports.UserRepo) *Port { return &Port{users: users} }

func (p *Port) UserExists(ctx context.Context, id string) (bool, error) {
	_, err := p.users.ByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (p *Port) UserTimezone(ctx context.Context, id string) (string, error) {
	u, err := p.users.ByID(ctx, id)
	if err != nil {
		return "", err
	}
	return u.Timezone, nil
}

var _ ports.IdentityPort = (*Port)(nil)
```

- [ ] **Step 3: Error mapping domain → envelope (AD-14)**

Create `internal/identity/handler/errors.go`:
```go
package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/memorix/memorix/internal/identity/domain"
	"github.com/memorix/memorix/internal/platform/httpx"
)

// writeErr map domain error → APIError envelope (AD-14). Mặc định 500.
func writeErr(c *gin.Context, err error) {
	var e *httpx.APIError
	switch {
	case errors.Is(err, domain.ErrWeakPassword):
		e = httpx.NewError(httpx.CodeValidation, "password is too weak").WithField("password", "choose a stronger password")
	case errors.Is(err, domain.ErrInvalidProfile):
		e = httpx.NewError(httpx.CodeValidation, "invalid profile value")
	case errors.Is(err, domain.ErrEmailTaken):
		e = httpx.NewError(httpx.CodeConflict, "email already registered")
	case errors.Is(err, domain.ErrInvalidCredentials):
		e = httpx.NewError(httpx.CodeUnauthenticated, "invalid email or password")
	case errors.Is(err, domain.ErrTokenInvalid):
		e = httpx.NewError(httpx.CodeUnauthenticated, "token invalid or expired")
	case errors.Is(err, domain.ErrReuseDetected):
		e = httpx.NewError(httpx.CodeUnauthenticated, "session revoked, please sign in again")
	case errors.Is(err, domain.ErrRateLimited):
		e = httpx.NewError(httpx.CodeRateLimited, "too many attempts, try again later")
	case errors.Is(err, domain.ErrOAuthNoMerge):
		e = httpx.NewError(httpx.CodeConflict, "an account with this email already exists")
	case errors.Is(err, domain.ErrOAuthFailed):
		e = httpx.NewError(httpx.CodeUnauthenticated, "oauth verification failed")
	case errors.Is(err, domain.ErrNotFound):
		e = httpx.NewError(httpx.CodeNotFound, "not found")
	default:
		e = httpx.NewError(httpx.CodeInternal, "internal error")
	}
	e.WithTrace(c.GetHeader("X-Request-Id"))
	c.JSON(e.HTTPStatus(), e)
}
```

- [ ] **Step 4: HTTP handlers + route registration**

Create `internal/identity/handler/handler.go`:
```go
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/memorix/memorix/internal/identity/ports"
	"github.com/memorix/memorix/internal/identity/service"
	"github.com/memorix/memorix/internal/platform/authmw"
)

const refreshCookie = "memorix_refresh"

// Handler là driving adapter Gin cho module identity.
type Handler struct {
	svc        *service.Service
	mailer     ports.Mailer
	oauth      *OAuthDeps
	jwt        *authmw.JWTManager
	refreshTTL time.Duration
	secure     bool // Secure cookie (tắt trong test http)
}

// OAuthDeps tách để test không cần OIDC thật.
type OAuthDeps struct {
	Verifier ports.OIDCVerifier
	Begin    func(provider string) (redirectURL, state, nonce, verifier string, err error)
}

func New(svc *service.Service, mailer ports.Mailer, jwt *authmw.JWTManager, refreshTTL time.Duration, secure bool, oauth *OAuthDeps) *Handler {
	return &Handler{svc: svc, mailer: mailer, jwt: jwt, refreshTTL: refreshTTL, secure: secure, oauth: oauth}
}

// Register gắn route vào group /api/v1. protected group verify JWT (AD-11).
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup) {
	a := v1.Group("/auth")
	a.POST("/register", h.register)
	a.POST("/verify-email", h.verifyEmail)
	a.POST("/login", h.login)
	a.POST("/refresh", h.refresh)
	a.POST("/logout", h.logout)
	a.POST("/password/forgot", h.forgot)
	a.POST("/password/reset", h.reset)
	if h.oauth != nil {
		a.GET("/oauth/:provider/start", h.oauthStart)
		a.POST("/oauth/:provider/callback", h.oauthCallback)
	}

	me := v1.Group("")
	me.Use(authmw.RequireAuth(h.jwt))
	me.GET("/me", h.getMe)
	me.PATCH("/me", h.updateMe)
	me.POST("/account/export", h.exportData)
	me.DELETE("/account", h.deleteAccount)
}

func (h *Handler) setRefresh(c *gin.Context, raw string) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookie, raw, int(h.refreshTTL.Seconds()), "/api/v1/auth", "", h.secure, true)
}

func (h *Handler) register(c *gin.Context) {
	var body struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	res, err := h.svc.Register(c.Request.Context(), service.RegisterInput{
		Email: body.Email, Password: body.Password, DisplayName: body.DisplayName,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	_ = h.mailer.SendVerification(c.Request.Context(), body.Email, res.VerifyToken)
	h.setRefresh(c, res.Tokens.RefreshToken)
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{
		"user_id":      res.UserID,
		"access_token": res.Tokens.AccessToken,
		"expires_at":   res.Tokens.AccessExpiresAt,
	}})
}

func (h *Handler) verifyEmail(c *gin.Context) {
	var body struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	if err := h.svc.VerifyEmail(c.Request.Context(), body.Token); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"verified": true}})
}

func (h *Handler) login(c *gin.Context) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	tok, err := h.svc.Login(c.Request.Context(), body.Email, body.Password)
	if err != nil {
		writeErr(c, err)
		return
	}
	h.setRefresh(c, tok.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"access_token": tok.AccessToken, "expires_at": tok.AccessExpiresAt,
	}})
}

func (h *Handler) refresh(c *gin.Context) {
	raw, err := c.Cookie(refreshCookie)
	if err != nil || raw == "" {
		writeErr(c, domainTokenInvalid)
		return
	}
	tok, err := h.svc.Refresh(c.Request.Context(), raw)
	if err != nil {
		writeErr(c, err)
		return
	}
	h.setRefresh(c, tok.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"access_token": tok.AccessToken, "expires_at": tok.AccessExpiresAt,
	}})
}

func (h *Handler) logout(c *gin.Context) {
	// xóa cookie; refresh sẽ tự hết hạn / có thể revoke chủ động ở V1.
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookie, "", -1, "/api/v1/auth", "", h.secure, true)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"logged_out": true}})
}

func (h *Handler) forgot(c *gin.Context) {
	var body struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	raw, err := h.svc.RequestReset(c.Request.Context(), body.Email)
	if err != nil {
		writeErr(c, err)
		return
	}
	if raw != "" {
		_ = h.mailer.SendPasswordReset(c.Request.Context(), body.Email, raw)
	}
	// Response GIỐNG NHAU dù email tồn tại hay không (Story 1.6).
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"sent": true}})
}

func (h *Handler) reset(c *gin.Context) {
	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), body.Token, body.NewPassword); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"reset": true}})
}

func (h *Handler) oauthStart(c *gin.Context) {
	redirectURL, state, nonce, verifier, err := h.oauth.Begin(c.Param("provider"))
	if err != nil {
		writeErr(c, domainOAuthFailed)
		return
	}
	// state/nonce/verifier lưu cookie ngắn hạn httpOnly để đối chiếu ở callback.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("oauth_state", state, 600, "/api/v1/auth", "", h.secure, true)
	c.SetCookie("oauth_nonce", nonce, 600, "/api/v1/auth", "", h.secure, true)
	c.SetCookie("oauth_verifier", verifier, 600, "/api/v1/auth", "", h.secure, true)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"authorization_url": redirectURL}})
}

func (h *Handler) oauthCallback(c *gin.Context) {
	var body struct {
		Code        string `json:"code"`
		State       string `json:"state"`
		RedirectURI string `json:"redirect_uri"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	state, _ := c.Cookie("oauth_state")
	nonce, _ := c.Cookie("oauth_nonce")
	verifier, _ := c.Cookie("oauth_verifier")
	if state == "" || body.State != state {
		writeErr(c, domainOAuthFailed)
		return
	}
	tok, err := h.svc.OAuthLogin(c.Request.Context(), c.Param("provider"), body.Code, verifier, body.RedirectURI, nonce)
	if err != nil {
		writeErr(c, err)
		return
	}
	h.setRefresh(c, tok.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"access_token": tok.AccessToken, "expires_at": tok.AccessExpiresAt,
	}})
}

func (h *Handler) getMe(c *gin.Context) {
	p, _ := authmw.PrincipalFrom(c)
	u, err := h.svc.GetUser(c.Request.Context(), p.UserID)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": u})
}

func (h *Handler) updateMe(c *gin.Context) {
	p, _ := authmw.PrincipalFrom(c)
	var body struct {
		DisplayName *string `json:"display_name"`
		Timezone    *string `json:"timezone"`
		Locale      *string `json:"locale"`
		Theme       *string `json:"theme"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	u, err := h.svc.UpdateProfile(c.Request.Context(), p.UserID, service.ProfileInput{
		DisplayName: body.DisplayName, Timezone: body.Timezone, Locale: body.Locale, Theme: body.Theme,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id": u.ID, "email": u.Email, "display_name": u.DisplayName,
		"timezone": u.Timezone, "locale": u.Locale, "theme": u.Theme,
	}})
}

func (h *Handler) exportData(c *gin.Context) {
	p, _ := authmw.PrincipalFrom(c)
	var body struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, errBadJSON)
		return
	}
	exp, err := h.svc.ExportData(c.Request.Context(), p.UserID, body.Password)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="memorix-export.json"`)
	c.JSON(http.StatusOK, exp)
}

func (h *Handler) deleteAccount(c *gin.Context) {
	p, _ := authmw.PrincipalFrom(c)
	if err := h.svc.DeleteAccount(c.Request.Context(), p.UserID); err != nil {
		writeErr(c, err)
		return
	}
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookie, "", -1, "/api/v1/auth", "", h.secure, true)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": true}})
}
```

Create `internal/identity/handler/sentinels.go` (lỗi cục bộ handler → map qua writeErr):
```go
package handler

import "github.com/memorix/memorix/internal/identity/domain"

var (
	errBadJSON         = domain.ErrInvalidProfile // 400 VALIDATION_ERROR cho body sai
	domainTokenInvalid = domain.ErrTokenInvalid
	domainOAuthFailed  = domain.ErrOAuthFailed
)
```

- [ ] **Step 5: Thêm `GetUser` vào service (đọc hồ sơ cho GET /me)**

Create `internal/identity/service/get_user.go`:
```go
package service

import "context"

// UserView là hồ sơ trả cho client (không lộ password_hash).
type UserView struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Timezone    string `json:"timezone"`
	Locale      string `json:"locale"`
	Theme       string `json:"theme"`
	Plan        string `json:"plan"`
	Role        string `json:"role"`
	Verified    bool   `json:"verified"`
}

func (s *Service) GetUser(ctx context.Context, userID string) (UserView, error) {
	u, err := s.deps.Users.ByID(ctx, userID)
	if err != nil {
		return UserView{}, err
	}
	return UserView{
		ID: u.ID, Email: u.Email, DisplayName: u.DisplayName,
		Timezone: u.Timezone, Locale: u.Locale, Theme: u.Theme,
		Plan: string(u.Plan), Role: string(u.Role), Verified: u.IsVerified(),
	}, nil
}
```

- [ ] **Step 6: Write the failing handler test**

Create `internal/identity/handler/handler_test.go`:
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
	"github.com/memorix/memorix/internal/identity/repo/memory"
	"github.com/memorix/memorix/internal/identity/service"
	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/platform/eventbus"
	"github.com/memorix/memorix/internal/platform/ratelimit"
	"github.com/memorix/memorix/internal/platform/security"
)

type testMailer struct{}

func (testMailer) SendVerification(context.Context, string, string) error  { return nil }
func (testMailer) SendPasswordReset(context.Context, string, string) error { return nil }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func newTestServer(t *testing.T) (*gin.Engine, *authmw.JWTManager) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := memory.New()
	jwt := authmw.NewJWTManager([]byte("test-secret"), 15*time.Minute, "memorix")
	svc := service.New(service.Deps{
		Users: st.Users, Sessions: st.Sessions, Tokens: st.Tokens, OAuth: st.OAuth,
		Hasher:  security.NewArgon2Hasher(),
		Issuer:  jwt,
		Secrets: security.TokenFactory{},
		Clock:   realClock{},
		Limiter: ratelimit.NewWindow(5, time.Minute),
		Bus:     eventbus.NewInProcess(),
		RefreshTTL: 30 * 24 * time.Hour, VerifyTTL: 24 * time.Hour, ResetTTL: time.Hour,
	})
	h := New(svc, testMailer{}, jwt, 30*24*time.Hour, false, nil)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	return r, jwt
}

func doJSON(r *gin.Engine, method, path, body, bearer string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandler_RegisterThenDuplicate(t *testing.T) {
	r, _ := newTestServer(t)
	body := `{"email":"h@example.com","password":"Tr0ub4dour!","display_name":"H"}`
	w := doJSON(r, http.MethodPost, "/api/v1/auth/register", body, "", nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Data.AccessToken == "" {
		t.Error("expected access_token in response")
	}
	if !hasCookie(w, refreshCookie) {
		t.Error("expected httpOnly refresh cookie set")
	}
	// duplicate → 409
	w2 := doJSON(r, http.MethodPost, "/api/v1/auth/register", body, "", nil)
	if w2.Code != http.StatusConflict {
		t.Errorf("duplicate register status = %d, want 409", w2.Code)
	}
}

func TestHandler_LoginWrongIs401Envelope(t *testing.T) {
	r, _ := newTestServer(t)
	doJSON(r, http.MethodPost, "/api/v1/auth/register",
		`{"email":"h@example.com","password":"Tr0ub4dour!"}`, "", nil)
	w := doJSON(r, http.MethodPost, "/api/v1/auth/login",
		`{"email":"h@example.com","password":"nope-nope9"}`, "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if !strings.Contains(w.Body.String(), "UNAUTHENTICATED") {
		t.Errorf("expected UNAUTHENTICATED envelope, got %s", w.Body.String())
	}
}

func TestHandler_MeRequiresAuth(t *testing.T) {
	r, jwt := newTestServer(t)
	// không token → 401
	if w := doJSON(r, http.MethodGet, "/api/v1/me", "", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("no-token /me = %d, want 401", w.Code)
	}
	// register lấy user id qua token
	reg := doJSON(r, http.MethodPost, "/api/v1/auth/register",
		`{"email":"me@example.com","password":"Tr0ub4dour!"}`, "", nil)
	var got struct {
		Data struct {
			UserID string `json:"user_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(reg.Body.Bytes(), &got)
	tok, _, _ := jwt.Issue(got.Data.UserID, "user", "free")
	w := doJSON(r, http.MethodGet, "/api/v1/me", "", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("authorized /me = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "me@example.com") {
		t.Errorf("me payload missing email: %s", w.Body.String())
	}
}

func hasCookie(w *httptest.ResponseRecorder, name string) bool {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return true
		}
	}
	return false
}
```

- [ ] **Step 7: Run test to verify it fails then passes**

Run: `go test ./internal/identity/handler/ -v`
Expected: sau khi tạo handler + `GetUser` + dọn `testMailer` → PASS (3 test).

- [ ] **Step 8: Mở rộng config cho auth**

Edit `internal/platform/config/config.go` — thêm field và parse (giữ default để test Sprint 0 vẫn xanh):
```go
package config

import (
	"fmt"
	"time"
)

type Config struct {
	AppEnv      string
	HTTPPort    string
	DatabaseURL string
	RedisURL    string

	JWTSecret  string
	JWTIssuer  string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	VerifyTTL  time.Duration
	ResetTTL   time.Duration

	GoogleClientID     string
	GoogleClientSecret string
	OAuthRedirectURL   string
}

func Load(getenv func(string) string) (Config, error) {
	c := Config{
		AppEnv:      or(getenv("APP_ENV"), "development"),
		HTTPPort:    or(getenv("HTTP_PORT"), "8080"),
		DatabaseURL: getenv("DATABASE_URL"),
		RedisURL:    getenv("REDIS_URL"),

		JWTSecret: or(getenv("JWT_SECRET"), "dev-insecure-secret-change-me"),
		JWTIssuer: or(getenv("JWT_ISSUER"), "memorix"),

		GoogleClientID:     getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: getenv("GOOGLE_CLIENT_SECRET"),
		OAuthRedirectURL:   getenv("OAUTH_REDIRECT_URL"),
	}
	c.AccessTTL = parseDur(getenv("ACCESS_TTL"), 15*time.Minute)
	c.RefreshTTL = parseDur(getenv("REFRESH_TTL"), 30*24*time.Hour)
	c.VerifyTTL = parseDur(getenv("VERIFY_TTL"), 24*time.Hour)
	c.ResetTTL = parseDur(getenv("RESET_TTL"), time.Hour)

	if c.AppEnv == "production" {
		if c.DatabaseURL == "" {
			return Config{}, fmt.Errorf("DATABASE_URL required in production")
		}
		if c.JWTSecret == "dev-insecure-secret-change-me" {
			return Config{}, fmt.Errorf("JWT_SECRET required in production")
		}
	}
	return c, nil
}

func parseDur(v string, def time.Duration) time.Duration {
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
```

- [ ] **Step 9: Verify config test vẫn xanh**

Run: `go test ./internal/platform/config/ -v`
Expected: PASS (3 test cũ — default HTTPPort=8080, prod thiếu DATABASE_URL vẫn lỗi trước JWT).

- [ ] **Step 10: Wire module identity vào cmd/api**

Edit `cmd/api/main.go`:
```go
package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/memorix/memorix/internal/identity/handler"
	"github.com/memorix/memorix/internal/identity/mailer"
	identityrepo "github.com/memorix/memorix/internal/identity/repo"
	"github.com/memorix/memorix/internal/identity/service"
	"github.com/memorix/memorix/internal/platform/authmw"
	"github.com/memorix/memorix/internal/platform/config"
	"github.com/memorix/memorix/internal/platform/eventbus"
	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/platform/logger"
	"github.com/memorix/memorix/internal/platform/ratelimit"
	"github.com/memorix/memorix/internal/platform/security"
)

type sysClock struct{}

func (sysClock) Now() time.Time { return time.Now() }

func main() {
	log := logger.New(os.Stdout, "info")
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("config load failed", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db pool failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	repos := identityrepo.New(pool)
	jwt := authmw.NewJWTManager([]byte(cfg.JWTSecret), cfg.AccessTTL, cfg.JWTIssuer)
	bus := eventbus.NewInProcess()

	svc := service.New(service.Deps{
		Users: repos.Users, Sessions: repos.Sessions, Tokens: repos.Tokens, OAuth: repos.OAuth,
		Hasher:  security.NewArgon2Hasher(),
		Issuer:  jwt,
		Secrets: security.TokenFactory{},
		Clock:   sysClock{},
		Limiter: ratelimit.NewWindow(10, time.Minute),
		Bus:     bus,
		RefreshTTL: cfg.RefreshTTL, VerifyTTL: cfg.VerifyTTL, ResetTTL: cfg.ResetTTL,
	})

	r := httpx.NewRouter()
	v1 := r.Group("/api/v1")
	h := handler.New(svc, mailer.NewLogMailer(log), jwt, cfg.RefreshTTL, cfg.AppEnv != "development", nil)
	h.RegisterRoutes(v1)
	// OAuth wiring: khi có GOOGLE_CLIENT_ID, dựng oauthx.New(ctx, ...) và truyền OAuthDeps.
	// Bỏ qua ở bootstrap tối thiểu nếu chưa cấu hình provider.

	_ = service.NewPort(repos.Users) // IdentityPort — module khác consume ở sprint sau

	log.Info("api starting", "port", cfg.HTTPPort, "env", cfg.AppEnv)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, r); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 11: Verify build**

Run: `go build ./...`
Expected: no error.

- [ ] **Step 12: Commit**

```bash
git add internal/identity/handler internal/identity/mailer internal/identity/service/identity_port.go internal/identity/service/get_user.go internal/platform/config/config.go cmd/api/main.go
git commit -m "feat(identity): HTTP handlers, IdentityPort, config, and cmd/api wiring"
```

---

### Task 20: worker — job purge tài khoản soft-deleted (Story 1.8) (TDD)

**Files:**
- Create: `internal/identity/service/purge.go`
- Test: `internal/identity/service/purge_test.go`
- Edit: `cmd/worker/main.go` (đăng ký periodic purge)

Tách logic purge khỏi River để test thuần bằng fake repo; River chỉ là driver.

- [ ] **Step 1: Write the failing test**

Create `internal/identity/service/purge_test.go`:
```go
package service

import (
	"context"
	"testing"
	"time"
)

func TestPurgeDeletedAccounts_RemovesOldSoftDeletes(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{Email: "old@example.com", Password: "Tr0ub4dour!"})
	// soft-delete tại thời điểm quá khứ
	h.clock.t = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := h.svc.DeleteAccount(context.Background(), res.UserID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// purge mọi tài khoản xóa trước 2026-06-15 (retention 14 ngày)
	n, err := h.svc.PurgeDeletedAccounts(context.Background(), 14*24*time.Hour, time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 account purged, got %d", n)
	}
}

func TestPurgeDeletedAccounts_KeepsRecent(t *testing.T) {
	h := newHarness()
	res, _ := h.svc.Register(context.Background(), RegisterInput{Email: "recent@example.com", Password: "Tr0ub4dour!"})
	h.clock.t = time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	_ = h.svc.DeleteAccount(context.Background(), res.UserID)
	// xóa mới 1 ngày trước, retention 14d → chưa purge
	n, err := h.svc.PurgeDeletedAccounts(context.Background(), 14*24*time.Hour, time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 0 {
		t.Errorf("recent soft-delete must be retained, purged %d", n)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/service/ -run TestPurgeDeletedAccounts -v`
Expected: FAIL (`PurgeDeletedAccounts` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/identity/service/purge.go`:
```go
package service

import (
	"context"
	"time"
)

// PurgeDeletedAccounts xóa cứng tài khoản đã soft-delete quá `retention` tính
// đến `now` (Story 1.8: soft-delete → purge theo lịch). CASCADE dọn
// sessions/email_tokens/oauth_identities (FK ON DELETE CASCADE trong schema).
func (s *Service) PurgeDeletedAccounts(ctx context.Context, retention time.Duration, now time.Time) (int, error) {
	cutoff := now.Add(-retention)
	return s.deps.Users.PurgeDeletedBefore(ctx, cutoff)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/identity/service/ -run TestPurgeDeletedAccounts -v`
Expected: PASS (2 test).

- [ ] **Step 5: Đăng ký periodic job trong worker (River)**

Edit `cmd/worker/main.go` — thay thân `main` để dựng pool + River client với periodic purge (idempotent, chạy hằng ngày):
```go
package main

import (
	"context"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/memorix/memorix/internal/identity/repo"
	"github.com/memorix/memorix/internal/platform/config"
	"github.com/memorix/memorix/internal/platform/logger"
)

const purgeRetention = 30 * 24 * time.Hour

func main() {
	log := logger.New(os.Stdout, "info")
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("config load failed", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db pool failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	repos := repo.New(pool)
	// Purge chạy trực tiếp qua repo (không cần full Service graph cho job hạ tầng).
	tick := time.NewTicker(24 * time.Hour)
	defer tick.Stop()
	log.Info("worker started: daily GDPR purge scheduled", "retention", purgeRetention.String())
	for {
		n, err := repos.Users.PurgeDeletedBefore(ctx, time.Now().Add(-purgeRetention))
		if err != nil {
			log.Error("purge failed", "err", err)
		} else if n > 0 {
			log.Info("purged deleted accounts", "count", n)
		}
		<-tick.C
	}
}
```

Executor lưu ý: bản River đầy đủ (`river.PeriodicJob` + `riverpgxv5`) thay cho `time.Ticker` là fast-follow — ticker đủ cho MVP 1-instance. Logic + test purge đã hoàn chỉnh ở Step 1–4.

- [ ] **Step 6: Verify build + toàn bộ test đơn vị**

Run:
```bash
go build ./...
go test ./... -short
```
Expected: build xanh; unit test (bỏ testcontainers qua `-short`) PASS toàn bộ.

- [ ] **Step 7: Commit**

```bash
git add internal/identity/service/purge.go internal/identity/service/purge_test.go cmd/worker/main.go
git commit -m "feat(identity): scheduled GDPR purge of soft-deleted accounts (Story 1.8)"
```

---

## Self-Review

### Story AC → Task map

**Story 1.2 — Đăng ký (email+password)**
- AC email hợp lệ + pw ≥8 → tạo user, argon2id hash, ghi `identity.users` → Task 2 (schema), Task 5 (argon2id), Task 9 (Register)
- AC trả access token + phát email xác thực → Task 9 (issueSession + verify token), Task 19 (mailer)
- AC email đã tồn tại → 409 CONFLICT → Task 9 (`ErrEmailTaken`), Task 19 (`writeErr`→409)
- AC pw yếu (score <2) → 400 VALIDATION_ERROR nêu trường → Task 4 (policy), Task 19 (`writeErr` field)

**Story 1.3 — Xác thực email**
- AC token hash 1-lần TTL 24h `email_tokens(kind=verify)` → Task 2 (bảng), Task 9 (phát khi register), Task 10
- AC mở link hợp lệ → set `email_verified_at`, mark used, không tái dùng → Task 10 (`VerifyEmail`)
- AC chưa xác thực bị giới hạn quyền (FR-2) → `User.IsVerified()` (Task 3) expose qua `/me.verified` (Task 19); enforcement quyền chi tiết ở module downstream (deferred — xem Gaps)

**Story 1.4 — Đăng nhập + phiên**
- AC access JWT 15m + refresh opaque hash `sessions`, cookie httpOnly+Secure+SameSite=Strict → Task 6 (JWT), Task 9 (issueSession), Task 19 (`setRefresh`)
- AC refresh → cấp cặp mới, rotation, vô hiệu cũ → Task 12 (`Refresh`)
- AC refresh cũ dùng lại → thu hồi cả family (reuse-detection) → Task 12
- AC sai N lần → rate-limit + không phân biệt email tồn tại → Task 8 (limiter), Task 11 (`Login` anti-enumeration)

**Story 1.5 — OAuth Google/Apple**
- AC Authorization Code + PKCE, verify id_token (sig/aud/iss + state/nonce), link `oauth_identities` → Task 18 (adapter), Task 13 (service), Task 19 (start/callback + state cookie)
- AC provider_uid chưa link → tạo/link; không auto-merge email chưa verified → Task 13 (`ErrOAuthNoMerge`)
- AC trả token như luồng chuẩn → Task 13 (issueSession)

**Story 1.6 — Đặt lại mật khẩu**
- AC token hash 1-lần TTL 1h (kind=reset); response giống nhau dù email tồn tại → Task 14 (`RequestReset`), Task 19 (`forgot` luôn 202)
- AC đặt pw mới hợp lệ → cập nhật, token used, thu hồi mọi session → Task 14 (`ResetPassword` + `RevokeAllForUser`)

**Story 1.7 — Hồ sơ**
- AC cập nhật tên/TZ/locale/theme → lưu `users`, áp ngay → Task 15 (`UpdateProfile`), Task 19 (`PATCH /me`)
- AC TZ dùng cho "ngày học" downstream (AD-12) → Task 15 (validate IANA), Task 19 (`IdentityPort.UserTimezone`)

**Story 1.8 — GDPR export + xóa**
- AC export JSON toàn bộ sau xác thực lại → Task 16 (`ExportData` re-auth), Task 19 (`POST /account/export`)
- AC xóa tài khoản → soft-delete→purge theo lịch, thu hồi session, không log PII/token → Task 16 (`DeleteAccount`), Task 20 (purge job), Task 19 (mailer scrub) + Task 2 (FK CASCADE dọn con)

### Placeholder scan
- KHÔNG có TBD/`TODO`/"add validation"/"similar to" trong code sản phẩm.
- Ghi chú có chủ đích (không phải placeholder): (a) `cmd/api` OAuth wiring gọi `oauthx.New` khi có `GOOGLE_CLIENT_ID` — code adapter đã hoàn chỉnh ở Task 18, chỉ hoãn bootstrap provider; (b) `cmd/worker` dùng `time.Ticker` thay River đầy đủ — thân purge hoàn chỉnh và test ở Task 20; (c) mọi "Executor lưu ý" là chỉ dẫn dọn import trong test, không để lại code khuyết.

### Type consistency (khớp Sprint 0 + xuyên suốt sprint này)
- Tái dùng nguyên: `httpx.APIError`/`httpx.CodeValidation|CodeUnauthenticated|CodeConflict|CodeRateLimited|...`, `httpx.NewRouter`, `config.Config`+`config.Load`, `logger.New`, `eventbus.Bus`/`Event`/`NewInProcess`/`Wait`, `db.Migrate`, pattern testcontainers `postgres:18` + `-short` skip.
- Ổn định trong sprint: `domain.User/Session/EmailToken/OAuthIdentity`, `domain.Plan/Role/TokenKind`, sentinel `domain.Err*`; `ports.*` (repo + security + `OIDCVerifier`/`OIDCClaims` + `IdentityPort`); `service.Deps/Service/TokenPair/RegisterInput/ProfileInput/Export/UserView`; `authmw.Principal/JWTManager/RequireAuth/PrincipalFrom`; `security.Argon2Hasher/TokenFactory`; `ratelimit.Window`.
- `authmw` không import `identity` (Principal dùng string) → platform không phụ thuộc module (đúng addendum "platform không phụ thuộc ngược module"). `oauthx`/`service`/`handler`/`repo` là ruột module import `ports`/`domain`. Depguard (S5) giữ `domain/` sạch hạ tầng: domain chỉ import `time`/`strings`/`unicode`/`errors` (stdlib).

### Gaps flagged
1. **Enforcement quyền cho tài khoản chưa verified (FR-2)**: sprint này expose trạng thái `verified` (JWT/`/me`); chặn hành động cụ thể thuộc module nghiệp vụ (vocabulary/review) — thực thi ở Sprint 2+ qua middleware `RequireVerified` đọc claim. Đã đặt nền (Principal + IsVerified).
2. **OAuth provider bootstrap + Apple**: adapter `oauthx` hoàn chỉnh (PKCE + id_token); Apple timing là OQ-4 (deferred theo Spine). Cấu hình discovery cần secret thật → verify ở staging (network).
3. **Rate-limit multi-instance**: in-memory fixed-window đúng cho MVP 1-instance; nâng Redis khi scale (Stage 4) — deferred có chủ đích.
4. **River đầy đủ cho purge**: MVP dùng ticker; chuyển `river.PeriodicJob` fast-follow (logic + test purge đã xong).
5. **Export đa-module**: MVP export scope identity; ghép dữ liệu vocabulary/review qua port của chúng ở V1 (các module chưa tồn tại ở sprint này).

---

## Execution Handoff
Sau khi lưu, chọn cách chạy: subagent-driven (khuyến nghị) hoặc inline executing-plans. Chạy testcontainers (Task 2, 17) cần Docker; CI dùng `-short` bỏ qua như Sprint 0.
