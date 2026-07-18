# Sprint 0 — Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dựng nền tảng Memorix (Go modular monolith + React shell + CI/CD) để mọi story sau xây trên đó — Story 1.1.

**Architecture:** Modular Monolith + Hexagonal core. `cmd/{api,worker}` + `internal/<module>/{domain,service,ports,handler,repo,internal}` + `internal/platform` + `internal/shared`. Ranh giới enforce bằng `internal/` con (compiler) + depguard (domain không import hạ tầng). DB schema-per-module (chỉ tạo schema rỗng ở sprint này). Envelope lỗi + cursor pagination chuẩn từ đầu.

**Tech Stack:** Go 1.26, Gin v1.10, pgx v5, golang-migrate v4, River, slog, Postgres 18, Redis 8, React 19 + Vite 7 + TS, testcontainers-go, testify.

**Nguồn:** `_bmad-output/planning-artifacts/architecture/architecture-memorix-2026-07-07/ARCHITECTURE-SPINE.md` (AD-1,2,8,10,13,14) + `addendum-structure.md` (S1-S7) + `_bmad-output/implementation-artifacts/1-1-foundation.md`.

**Scope boundary:** KHÔNG tạo bảng nghiệp vụ (chỉ schema rỗng + extension). KHÔNG implement auth logic (chỉ middleware skeleton). Auth thật = Sprint 1.

---

### Task 1: Khởi tạo repo, Go module, cấu trúc thư mục

**Files:**
- Create: `go.mod`, `.gitignore`, `cmd/api/main.go`, `cmd/worker/main.go`
- Create: cây `internal/<module>/…` (doc.go mỗi package)

- [ ] **Step 1: Init git + go module**

Run:
```bash
git init
go mod init github.com/memorix/memorix
```
Expected: tạo `.git/` + `go.mod` với `go 1.26`.

- [ ] **Step 2: Tạo cấu trúc thư mục theo addendum-structure**

Run:
```bash
mkdir -p cmd/api cmd/worker \
  internal/{identity,vocabulary,scheduling,review,progress,notification}/{domain,service,ports,handler,repo,internal} \
  internal/platform/{config,logger,httpx,db,eventbus,authmw} \
  internal/shared \
  migrations db/queries web
```

- [ ] **Step 3: Thêm doc.go cho mỗi module (đánh dấu bounded context)**

Create `internal/identity/doc.go` (lặp tương tự cho 6 module, đổi tên + mô tả):
```go
// Package identity là bounded context xác thực & tài khoản.
// Ruột module nằm dưới internal/; module khác chỉ dùng qua ports/.
package identity
```

- [ ] **Step 4: .gitignore**

Create `.gitignore`:
```
/bin/
/tmp/
*.env
.env
/web/node_modules/
/web/dist/
coverage.out
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore: init go module and module skeleton structure"
```

---

### Task 2: platform/httpx — Error envelope (TDD)

**Files:**
- Create: `internal/platform/httpx/errors.go`
- Test: `internal/platform/httpx/errors_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/platform/httpx/errors_test.go`:
```go
package httpx

import (
	"encoding/json"
	"testing"
)

func TestErrorEnvelope_Shape(t *testing.T) {
	e := NewError(CodeValidation, "email không hợp lệ").WithField("email", "bắt buộc").WithTrace("trace-123")
	b, _ := json.Marshal(e)
	var got map[string]map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	inner := got["error"]
	if inner["code"] != "VALIDATION_ERROR" {
		t.Errorf("code = %v, want VALIDATION_ERROR", inner["code"])
	}
	if inner["message"] != "email không hợp lệ" {
		t.Errorf("message = %v", inner["message"])
	}
	if inner["trace_id"] != "trace-123" {
		t.Errorf("trace_id = %v", inner["trace_id"])
	}
	if fields, ok := inner["fields"].(map[string]any); !ok || fields["email"] != "bắt buộc" {
		t.Errorf("fields = %v", inner["fields"])
	}
}

func TestErrorEnvelope_HTTPStatus(t *testing.T) {
	cases := map[ErrorCode]int{
		CodeValidation: 400, CodeUnauthenticated: 401, CodeForbidden: 403,
		CodeNotFound: 404, CodeConflict: 409, CodeRateLimited: 429, CodeInternal: 500,
	}
	for code, want := range cases {
		if got := NewError(code, "x").HTTPStatus(); got != want {
			t.Errorf("%s HTTPStatus = %d, want %d", code, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/platform/httpx/ -run TestErrorEnvelope -v`
Expected: FAIL (build error — `NewError`/`CodeValidation` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/platform/httpx/errors.go`:
```go
package httpx

type ErrorCode string

const (
	CodeValidation      ErrorCode = "VALIDATION_ERROR"
	CodeUnauthenticated ErrorCode = "UNAUTHENTICATED"
	CodeForbidden       ErrorCode = "FORBIDDEN"
	CodeNotFound        ErrorCode = "NOT_FOUND"
	CodeConflict        ErrorCode = "CONFLICT"
	CodeUnprocessable   ErrorCode = "UNPROCESSABLE"
	CodeRateLimited     ErrorCode = "RATE_LIMITED"
	CodeInternal        ErrorCode = "INTERNAL"
)

// APIError là envelope lỗi chuẩn (AD-14). Marshal thành {"error":{...}}.
type APIError struct {
	Code    ErrorCode         `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	TraceID string            `json:"trace_id,omitempty"`
}

func NewError(code ErrorCode, msg string) *APIError {
	return &APIError{Code: code, Message: msg}
}

func (e *APIError) WithField(k, v string) *APIError {
	if e.Fields == nil {
		e.Fields = map[string]string{}
	}
	e.Fields[k] = v
	return e
}

func (e *APIError) WithTrace(id string) *APIError { e.TraceID = id; return e }

func (e *APIError) Error() string { return string(e.Code) + ": " + e.Message }

func (e *APIError) MarshalJSON() ([]byte, error) {
	type inner APIError
	return jsonMarshal(map[string]inner{"error": inner(*e)})
}

func (e *APIError) HTTPStatus() int {
	switch e.Code {
	case CodeValidation, CodeUnprocessable:
		if e.Code == CodeUnprocessable {
			return 422
		}
		return 400
	case CodeUnauthenticated:
		return 401
	case CodeForbidden:
		return 403
	case CodeNotFound:
		return 404
	case CodeConflict:
		return 409
	case CodeRateLimited:
		return 429
	default:
		return 500
	}
}
```

Create `internal/platform/httpx/json.go`:
```go
package httpx

import "encoding/json"

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/platform/httpx/ -run TestErrorEnvelope -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add internal/platform/httpx/
git commit -m "feat(platform): standard API error envelope with HTTP status mapping"
```

---

### Task 3: platform/httpx — Cursor pagination (TDD)

**Files:**
- Create: `internal/platform/httpx/cursor.go`
- Test: `internal/platform/httpx/cursor_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/platform/httpx/cursor_test.go`:
```go
package httpx

import "testing"

func TestCursor_RoundTrip(t *testing.T) {
	c := Cursor{SortKey: "2026-07-07T10:00:00Z", ID: "abc-123"}
	enc := c.Encode()
	if enc == "" {
		t.Fatal("encode empty")
	}
	got, err := DecodeCursor(enc)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.SortKey != c.SortKey || got.ID != c.ID {
		t.Errorf("round-trip mismatch: %+v vs %+v", got, c)
	}
}

func TestCursor_DecodeInvalid(t *testing.T) {
	if _, err := DecodeCursor("!!!not-base64!!!"); err == nil {
		t.Error("expected error on invalid cursor")
	}
}

func TestCursor_EmptyDecodesToZero(t *testing.T) {
	got, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("empty cursor should be valid start: %v", err)
	}
	if got.ID != "" {
		t.Errorf("empty cursor should be zero value, got %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/platform/httpx/ -run TestCursor -v`
Expected: FAIL (`Cursor`/`DecodeCursor` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/platform/httpx/cursor.go`:
```go
package httpx

import (
	"encoding/base64"
	"encoding/json"
)

// Cursor cho pagination ổn định (AD-14). Encode = base64(JSON).
type Cursor struct {
	SortKey string `json:"k"`
	ID      string `json:"i"`
}

func (c Cursor) Encode() string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func DecodeCursor(s string) (Cursor, error) {
	if s == "" {
		return Cursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, err
	}
	var c Cursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return Cursor{}, err
	}
	return c, nil
}

// Page là envelope phân trang trả về cho client.
type Page struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Limit      int    `json:"limit"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/platform/httpx/ -run TestCursor -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/platform/httpx/cursor.go internal/platform/httpx/cursor_test.go
git commit -m "feat(platform): cursor-based pagination helper"
```

---

### Task 4: platform/config — Env config (TDD)

**Files:**
- Create: `internal/platform/config/config.go`
- Test: `internal/platform/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/platform/config/config_test.go`:
```go
package config

import "testing"

func TestLoad_Defaults(t *testing.T) {
	c, err := Load(func(k string) string { return "" }) // no env set
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.HTTPPort != "8080" {
		t.Errorf("default HTTPPort = %q, want 8080", c.HTTPPort)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	env := map[string]string{
		"HTTP_PORT":    "9000",
		"DATABASE_URL": "postgres://x",
		"REDIS_URL":    "redis://y",
	}
	c, err := Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.HTTPPort != "9000" || c.DatabaseURL != "postgres://x" || c.RedisURL != "redis://y" {
		t.Errorf("config not read from env: %+v", c)
	}
}

func TestLoad_MissingRequiredInProd(t *testing.T) {
	env := map[string]string{"APP_ENV": "production"} // DATABASE_URL missing
	if _, err := Load(func(k string) string { return env[k] }); err == nil {
		t.Error("expected error: DATABASE_URL required in production")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/platform/config/ -v`
Expected: FAIL (`Load` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/platform/config/config.go`:
```go
package config

import "fmt"

type Config struct {
	AppEnv      string
	HTTPPort    string
	DatabaseURL string
	RedisURL    string
}

// Load đọc config theo 12-factor. getenv được inject để test.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		AppEnv:      or(getenv("APP_ENV"), "development"),
		HTTPPort:    or(getenv("HTTP_PORT"), "8080"),
		DatabaseURL: getenv("DATABASE_URL"),
		RedisURL:    getenv("REDIS_URL"),
	}
	if c.AppEnv == "production" && c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL required in production")
	}
	return c, nil
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/platform/config/ -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/platform/config/
git commit -m "feat(platform): 12-factor env config loader"
```

---

### Task 5: platform/eventbus — In-process bus (TDD)

**Files:**
- Create: `internal/platform/eventbus/bus.go`
- Test: `internal/platform/eventbus/bus_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/platform/eventbus/bus_test.go`:
```go
package eventbus

import (
	"context"
	"sync"
	"testing"
)

func TestInProcessBus_PublishDelivers(t *testing.T) {
	bus := NewInProcess()
	var mu sync.Mutex
	got := []string{}
	bus.Subscribe("CardGraded", func(_ context.Context, e Event) {
		mu.Lock()
		got = append(got, e.Name)
		mu.Unlock()
	})
	bus.Publish(context.Background(), Event{Name: "CardGraded", Payload: nil})
	bus.Wait()
	if len(got) != 1 || got[0] != "CardGraded" {
		t.Errorf("handler not called correctly: %v", got)
	}
}

func TestInProcessBus_IgnoresUnsubscribed(t *testing.T) {
	bus := NewInProcess()
	called := false
	bus.Subscribe("A", func(context.Context, Event) { called = true })
	bus.Publish(context.Background(), Event{Name: "B"})
	bus.Wait()
	if called {
		t.Error("handler for A should not fire on B")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/platform/eventbus/ -v`
Expected: FAIL (`NewInProcess`/`Event` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/platform/eventbus/bus.go`:
```go
package eventbus

import (
	"context"
	"sync"
)

// Event là domain event (tên PascalCase quá khứ, vd CardGraded).
type Event struct {
	Name    string
	Payload any
}

type Handler func(context.Context, Event)

// Bus là port; MVP dùng InProcess fire-and-forget (AD-8).
// Interface sẵn để nâng transactional outbox ở V1.
type Bus interface {
	Publish(ctx context.Context, e Event)
	Subscribe(name string, h Handler)
}

type InProcess struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	wg       sync.WaitGroup
}

func NewInProcess() *InProcess {
	return &InProcess{handlers: map[string][]Handler{}}
}

func (b *InProcess) Subscribe(name string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = append(b.handlers[name], h)
}

func (b *InProcess) Publish(ctx context.Context, e Event) {
	b.mu.RLock()
	hs := b.handlers[e.Name]
	b.mu.RUnlock()
	for _, h := range hs {
		b.wg.Add(1)
		h := h
		go func() {
			defer b.wg.Done()
			h(ctx, e)
		}()
	}
}

// Wait chờ mọi handler async xong (dùng trong test/shutdown).
func (b *InProcess) Wait() { b.wg.Wait() }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/platform/eventbus/ -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/platform/eventbus/
git commit -m "feat(platform): in-process event bus (foundation for outbox in V1)"
```

---

### Task 6: platform/logger — slog JSON + scrub

**Files:**
- Create: `internal/platform/logger/logger.go`
- Test: `internal/platform/logger/logger_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/platform/logger/logger_test.go`:
```go
package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestNew_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, "info")
	l.Info("hello", "user", "linh")
	out := buf.String()
	if !strings.Contains(out, `"msg":"hello"`) || !strings.Contains(out, `"user":"linh"`) {
		t.Errorf("expected JSON log, got %q", out)
	}
}

func TestScrub_RedactsSensitive(t *testing.T) {
	for _, k := range []string{"password", "token", "refresh_token", "authorization"} {
		if Scrub(k, "secret") != "[REDACTED]" {
			t.Errorf("key %q not scrubbed", k)
		}
	}
	if Scrub("email", "a@b.com") != "a@b.com" {
		t.Error("non-sensitive key should pass through")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/platform/logger/ -v`
Expected: FAIL (`New`/`Scrub` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `internal/platform/logger/logger.go`:
```go
package logger

import (
	"io"
	"log/slog"
	"strings"
)

var sensitive = map[string]bool{
	"password": true, "token": true, "refresh_token": true,
	"access_token": true, "authorization": true, "secret": true,
}

// Scrub thay giá trị field nhạy cảm bằng [REDACTED] (NFR-14).
func Scrub(key, val string) string {
	if sensitive[strings.ToLower(key)] {
		return "[REDACTED]"
	}
	return val
}

func New(w io.Writer, level string) *slog.Logger {
	var lv slog.Level
	_ = lv.UnmarshalText([]byte(level))
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lv}))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/platform/logger/ -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/platform/logger/
git commit -m "feat(platform): structured JSON logger with PII scrubbing"
```

---

### Task 7: Gin server + /api/v1/health (TDD via httptest)

**Files:**
- Create: `internal/platform/httpx/router.go`
- Test: `internal/platform/httpx/router_test.go`
- Create: `cmd/api/main.go`

- [ ] **Step 1: Add dependencies**

Run:
```bash
go get github.com/gin-gonic/gin@latest
```
Expected: gin thêm vào go.mod.

- [ ] **Step 2: Write the failing test**

Create `internal/platform/httpx/router_test.go`:
```go
package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	r := NewRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`status field = %q, want "ok"`, body["status"])
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/platform/httpx/ -run TestHealth -v`
Expected: FAIL (`NewRouter` undefined).

- [ ] **Step 4: Write minimal implementation**

Create `internal/platform/httpx/router.go`:
```go
package httpx

import "github.com/gin-gonic/gin"

// NewRouter dựng Gin engine với route nền tảng (/api/v1). Module handler
// đăng ký thêm route qua RegisterModule ở cmd/api.
func NewRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	v1 := r.Group("/api/v1")
	v1.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	return r
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/platform/httpx/ -run TestHealth -v`
Expected: PASS.

- [ ] **Step 6: Wire cmd/api/main.go**

Create `cmd/api/main.go`:
```go
package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/memorix/memorix/internal/platform/config"
	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/platform/logger"
)

func main() {
	log := logger.New(os.Stdout, "info")
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("config load failed", "err", err)
		os.Exit(1)
	}
	r := httpx.NewRouter()
	log.Info("api starting", "port", cfg.HTTPPort, "env", cfg.AppEnv)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, r); err != nil {
		log.Error("server stopped", slog.Any("err", err))
		os.Exit(1)
	}
}
```

- [ ] **Step 7: Verify build + run**

Run:
```bash
go build ./...
HTTP_PORT=8080 go run ./cmd/api &
sleep 1 && curl -s localhost:8080/api/v1/health
```
Expected: `{"status":"ok"}`. Then `kill %1`.

- [ ] **Step 8: Commit**

```bash
git add cmd/api internal/platform/httpx/router.go internal/platform/httpx/router_test.go go.mod go.sum
git commit -m "feat(api): Gin server with /api/v1/health endpoint"
```

---

### Task 8: Migration 0001 — schema-per-module + extensions

**Files:**
- Create: `migrations/0001_init.up.sql`, `migrations/0001_init.down.sql`
- Create: `internal/platform/db/migrate.go`
- Test: `internal/platform/db/migrate_test.go` (testcontainers)

- [ ] **Step 1: Add deps**

Run:
```bash
go get github.com/jackc/pgx/v5@latest
go get github.com/golang-migrate/migrate/v4@latest
go get github.com/testcontainers/testcontainers-go@latest
go get github.com/testcontainers/testcontainers-go/modules/postgres@latest
```

- [ ] **Step 2: Write migration SQL**

Create `migrations/0001_init.up.sql`:
```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS unaccent;

CREATE SCHEMA IF NOT EXISTS identity;
CREATE SCHEMA IF NOT EXISTS vocabulary;
CREATE SCHEMA IF NOT EXISTS scheduling;
CREATE SCHEMA IF NOT EXISTS review;
CREATE SCHEMA IF NOT EXISTS progress;
CREATE SCHEMA IF NOT EXISTS notification;
-- Không tạo bảng nghiệp vụ ở đây; bảng tạo theo nhu cầu từng story (AD-10).
```

Create `migrations/0001_init.down.sql`:
```sql
DROP SCHEMA IF EXISTS notification CASCADE;
DROP SCHEMA IF EXISTS progress CASCADE;
DROP SCHEMA IF EXISTS review CASCADE;
DROP SCHEMA IF EXISTS scheduling CASCADE;
DROP SCHEMA IF EXISTS vocabulary CASCADE;
DROP SCHEMA IF EXISTS identity CASCADE;
```

- [ ] **Step 3: Write migrate runner**

Create `internal/platform/db/migrate.go`:
```go
package db

import (
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Migrate áp mọi migration up. migrationsURL vd "file://migrations".
func Migrate(migrationsURL, databaseURL string) error {
	m, err := migrate.New(migrationsURL, databaseURL)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
```

- [ ] **Step 4: Write the failing integration test**

Create `internal/platform/db/migrate_test.go`:
```go
package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go"
	"time"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMigrate_CreatesSchemas(t *testing.T) {
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
	var count int
	err = conn.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.schemata
		 WHERE schema_name IN ('identity','vocabulary','scheduling','review','progress','notification')`).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 6 {
		t.Errorf("expected 6 module schemas, got %d", count)
	}
}
```

- [ ] **Step 5: Run test (needs Docker running)**

Run: `go test ./internal/platform/db/ -run TestMigrate -v`
Expected: PASS (pulls postgres:18, applies migration, finds 6 schemas). First run slow (image pull).

- [ ] **Step 6: Commit**

```bash
git add migrations internal/platform/db go.mod go.sum
git commit -m "feat(db): schema-per-module migration with testcontainers verification"
```

---

### Task 9: cmd/worker skeleton (River)

**Files:**
- Create: `cmd/worker/main.go`

- [ ] **Step 1: Add River**

Run:
```bash
go get github.com/riverqueue/river@latest
go get github.com/riverqueue/river/riverdriver/riverpgxv5@latest
```

- [ ] **Step 2: Write worker skeleton**

Create `cmd/worker/main.go`:
```go
package main

import (
	"log/slog"
	"os"

	"github.com/memorix/memorix/internal/platform/config"
	"github.com/memorix/memorix/internal/platform/logger"
)

// Worker chạy job nền (reconcile daily_stats, forecast, purge) — AD-8, ARCH-12.
// Story sau đăng ký River workers. Sprint 0 chỉ dựng skeleton chạy được.
func main() {
	log := logger.New(os.Stdout, "info")
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("config load failed", slog.Any("err", err))
		os.Exit(1)
	}
	log.Info("worker starting (no jobs registered yet)", "env", cfg.AppEnv)
	// TODO(story sau): khởi tạo river.Client với riverpgxv5 + đăng ký workers.
	select {} // giữ tiến trình sống cho container
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./cmd/worker`
Expected: no error.

- [ ] **Step 4: Commit**

```bash
git add cmd/worker go.mod go.sum
git commit -m "chore(worker): River-based worker skeleton"
```

---

### Task 10: Enforce ranh giới — .golangci.yml depguard (S5)

**Files:**
- Create: `.golangci.yml`
- Create: `internal/scheduling/domain/doc.go` (để lint quét)

- [ ] **Step 1: Write golangci config với depguard**

Create `.golangci.yml`:
```yaml
version: "2"
linters:
  enable:
    - govet
    - staticcheck
    - errcheck
    - ineffassign
    - depguard
  settings:
    depguard:
      rules:
        domain-pure:
          files:
            - "**/domain/**"
          deny:
            - pkg: "github.com/gin-gonic/gin"
              desc: "domain phải độc lập framework (AD-2)"
            - pkg: "github.com/jackc/pgx/v5"
              desc: "domain không import hạ tầng DB (AD-2)"
            - pkg: "net/http"
              desc: "domain không biết HTTP (AD-2)"
```

- [ ] **Step 2: Verify lint chạy sạch trên code hiện có**

Run:
```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
golangci-lint run ./...
```
Expected: no issues (domain packages hiện rỗng, không vi phạm).

- [ ] **Step 3: Verify rule THỰC SỰ chặn — tạo vi phạm tạm rồi kỳ vọng fail**

Run:
```bash
printf 'package domain\nimport _ "net/http"\n' > internal/scheduling/domain/bad_tmp.go
golangci-lint run ./internal/scheduling/domain/ ; echo "exit=$?"
rm internal/scheduling/domain/bad_tmp.go
```
Expected: lint BÁO LỖI (depguard chặn net/http trong domain), `exit=1`. Sau khi xóa file tạm, lint sạch lại.

- [ ] **Step 4: Commit**

```bash
git add .golangci.yml
git commit -m "chore(lint): depguard enforces domain purity (AD-2)"
```

---

### Task 11: sqlc config

**Files:**
- Create: `sqlc.yaml`

- [ ] **Step 1: Write sqlc.yaml (per-module, gen vào repo/gen)**

Create `sqlc.yaml`:
```yaml
version: "2"
sql:
  - engine: postgresql
    schema: migrations
    queries: db/queries/identity
    gen:
      go:
        package: gen
        out: internal/identity/repo/gen
        sql_package: pgx/v5
# Thêm block tương tự cho vocabulary/scheduling/review/progress khi story cần query.
```

- [ ] **Step 2: Verify sqlc parse (chưa có query → chỉ validate config)**

Run:
```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
mkdir -p db/queries/identity
sqlc version
```
Expected: in version; config hợp lệ (bỏ generate khi chưa có .sql).

- [ ] **Step 3: Commit**

```bash
git add sqlc.yaml db/queries
git commit -m "chore(db): sqlc config, per-module codegen into repo/gen"
```

---

### Task 12: Docker Compose + Dockerfile

**Files:**
- Create: `Dockerfile`, `docker-compose.yml`, `.dockerignore`

- [ ] **Step 1: Dockerfile (multi-stage)**

Create `Dockerfile`:
```dockerfile
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/api /api
COPY --from=build /out/worker /worker
COPY migrations /migrations
EXPOSE 8080
ENTRYPOINT ["/api"]
```

Create `.dockerignore`:
```
web/node_modules
web/dist
.git
*.md
```

- [ ] **Step 2: docker-compose.yml**

Create `docker-compose.yml`:
```yaml
services:
  postgres:
    image: postgres:18
    environment:
      POSTGRES_DB: memorix
      POSTGRES_USER: memorix
      POSTGRES_PASSWORD: memorix
    ports: ["5432:5432"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U memorix"]
      interval: 5s
      retries: 5
  redis:
    image: redis:8
    ports: ["6379:6379"]
  api:
    build: .
    environment:
      APP_ENV: development
      HTTP_PORT: "8080"
      DATABASE_URL: postgres://memorix:memorix@postgres:5432/memorix?sslmode=disable
      REDIS_URL: redis://redis:6379
    ports: ["8080:8080"]
    depends_on:
      postgres: { condition: service_healthy }
  worker:
    build: .
    entrypoint: ["/worker"]
    environment:
      APP_ENV: development
      DATABASE_URL: postgres://memorix:memorix@postgres:5432/memorix?sslmode=disable
      REDIS_URL: redis://redis:6379
    depends_on:
      postgres: { condition: service_healthy }
```

- [ ] **Step 3: Verify stack up + health + migrate**

Run:
```bash
docker compose up -d --build
sleep 8
docker compose exec -T api /api --help 2>/dev/null || true
curl -s localhost:8080/api/v1/health
```
Expected: `{"status":"ok"}`.

Run cleanup: `docker compose down -v`

- [ ] **Step 4: Commit**

```bash
git add Dockerfile docker-compose.yml .dockerignore
git commit -m "chore(infra): Docker Compose stack (api, worker, postgres 18, redis 8)"
```

---

### Task 13: Frontend shell — Vite + React + tokens + nav + theme

**Files:**
- Create: `web/` (Vite scaffold)
- Create: `web/src/tokens.css`, `web/src/App.tsx`, `web/src/App.test.tsx`

- [ ] **Step 1: Scaffold Vite React-TS**

Run:
```bash
cd web && npm create vite@latest . -- --template react-ts && npm install
npm install -D vitest @testing-library/react @testing-library/jest-dom jsdom
```

- [ ] **Step 2: Design tokens (khớp prototype design/prototype/)**

Create `web/src/tokens.css`:
```css
:root{
  --accent:#5b5bd6; --ground:#f4f4f7; --surface:#fff; --ink:#1c1c22; --muted:#6b6b76; --line:#e6e6ea;
  --again:#d8452f; --hard:#d98a1e; --good:#2f9d5f; --easy:#3e76d6;
  --radius:14px; --tap:44px;
}
:root[data-theme="dark"]{
  --ground:#131318; --surface:#1c1c22; --ink:#ececf2; --muted:#9a9aa8; --line:#2c2c36;
}
@media (prefers-color-scheme:dark){
  :root:not([data-theme]){ --ground:#131318; --surface:#1c1c22; --ink:#ececf2; --muted:#9a9aa8; --line:#2c2c36; }
}
*{box-sizing:border-box}
body{margin:0;background:var(--ground);color:var(--ink);font-family:'Helvetica Neue',Helvetica,Arial,sans-serif}
```

- [ ] **Step 3: App shell (bottom-nav 4 tab + theme + responsive)**

Create `web/src/App.tsx`:
```tsx
import { useState } from "react";
import "./tokens.css";

const TABS = ["Trang chủ", "Ôn", "Thư viện", "Thống kê"];

export default function App() {
  const [tab, setTab] = useState(0);
  const [dark, setDark] = useState(false);
  const toggle = () => {
    const next = !dark;
    setDark(next);
    document.documentElement.setAttribute("data-theme", next ? "dark" : "light");
  };
  return (
    <div style={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      <header style={{ padding: 16, display: "flex", justifyContent: "space-between" }}>
        <strong style={{ color: "var(--accent)" }}>MEMORIX</strong>
        <button aria-label="Đổi giao diện" onClick={toggle}>◑</button>
      </header>
      <main style={{ flex: 1, padding: 16 }}>
        <h1>{TABS[tab]}</h1>
        <p style={{ color: "var(--muted)" }}>Màn placeholder — story sau triển khai.</p>
      </main>
      <nav
        role="navigation"
        style={{ display: "flex", borderTop: "1px solid var(--line)", background: "var(--surface)" }}
      >
        {TABS.map((t, i) => (
          <button
            key={t}
            onClick={() => setTab(i)}
            aria-current={tab === i}
            style={{ flex: 1, minHeight: "var(--tap)", border: "none", background: "none", color: tab === i ? "var(--accent)" : "var(--muted)" }}
          >
            {t}
          </button>
        ))}
      </nav>
    </div>
  );
}
```

- [ ] **Step 4: Write shell test**

Create `web/src/App.test.tsx`:
```tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import App from "./App";

describe("App shell", () => {
  it("renders 4 bottom-nav tabs", () => {
    render(<App />);
    ["Trang chủ", "Ôn", "Thư viện", "Thống kê"].forEach((t) =>
      expect(screen.getByRole("button", { name: t })).toBeInTheDocument()
    );
  });
  it("switches active tab on click", () => {
    render(<App />);
    fireEvent.click(screen.getByRole("button", { name: "Thư viện" }));
    expect(screen.getByRole("heading", { name: "Thư viện" })).toBeInTheDocument();
  });
  it("toggles theme attribute", () => {
    render(<App />);
    fireEvent.click(screen.getByLabelText("Đổi giao diện"));
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
  });
});
```

Add to `web/vite.config.ts` test block:
```ts
/// <reference types="vitest" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
export default defineConfig({
  plugins: [react()],
  test: { environment: "jsdom", globals: true, setupFiles: [] },
});
```

- [ ] **Step 5: Run frontend tests**

Run: `cd web && npx vitest run`
Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
cd .. && git add web
git commit -m "feat(web): app shell with 4-tab nav, theme toggle, design tokens"
```

---

### Task 14: GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write CI workflow**

Create `.github/workflows/ci.yml`:
```yaml
name: CI
on:
  pull_request:
  push:
    branches: [main]
jobs:
  backend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.26" }
      - run: go build ./...
      - run: go test ./... -short
      - uses: golangci/golangci-lint-action@v6
        with: { version: latest }
  frontend:
    runs-on: ubuntu-latest
    defaults: { run: { working-directory: web } }
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: "22" }
      - run: npm ci
      - run: npx vitest run
      - run: npm run build
```

- [ ] **Step 2: Add -short guard to testcontainers test**

Modify `internal/platform/db/migrate_test.go` — add at top of `TestMigrate_CreatesSchemas`:
```go
	if testing.Short() {
		t.Skip("skip container test in -short mode")
	}
```
(CI backend job dùng `-short` để bỏ test container; test container chạy local/nightly.)

- [ ] **Step 3: Verify workflow YAML hợp lệ**

Run: `cat .github/workflows/ci.yml | python3 -c "import sys,yaml; yaml.safe_load(sys.stdin); print('valid')"`
Expected: `valid`.

- [ ] **Step 4: Commit**

```bash
git add .github internal/platform/db/migrate_test.go
git commit -m "ci: GitHub Actions build+test+lint for backend and frontend"
```

---

### Task 15: README + verify toàn bộ

**Files:**
- Create: `README.md`

- [ ] **Step 1: README quickstart**

Create `README.md`:
```markdown
# Memorix

Web app học từ vựng tiếng Anh bằng FSRS. Go modular monolith + React.

## Chạy local
```bash
docker compose up -d --build
curl localhost:8080/api/v1/health   # {"status":"ok"}
```

## Test
```bash
go test ./...            # backend (bỏ -short để chạy testcontainers)
cd web && npx vitest run # frontend
golangci-lint run ./...  # lint + enforce ranh giới (depguard)
```

## Cấu trúc
Xem `_bmad-output/planning-artifacts/architecture/.../addendum-structure.md`.
Ranh giới module enforce bằng `internal/<module>/internal/` + depguard.
```

- [ ] **Step 2: Full verification sweep**

Run:
```bash
go build ./... && go test ./... -short && golangci-lint run ./...
cd web && npx vitest run && npm run build && cd ..
```
Expected: tất cả xanh.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: README quickstart and project layout"
```

---

## Self-Review

**Spec coverage (Story 1.1 AC → task):**
- AC#1 build/health → Task 7
- AC#2 module structure → Task 1
- AC#3 Docker + migrate schemas → Task 8, 12
- AC#4 platform primitives (logger/envelope/cursor/config/db/eventbus/authmw) → Task 2,3,4,5,6 (authmw skeleton: thêm ở Sprint 1 khi có JWT — ghi rõ deferred)
- AC#5 enforce depguard + internal/ → Task 1, 10
- AC#6 FE shell tokens/nav/theme/responsive → Task 13
- AC#7 CI/CD → Task 14

**Gap:** `platform/authmw` chỉ có thư mục ở Task 1; middleware verify JWT thực = Sprint 1 (không có JWT ở Sprint 0). Ghi chú: AC#4 nói "skeleton" nên chấp nhận thư mục + doc.go; verify thật ở Story 1.4.

**Placeholder scan:** không có TBD/TODO ngoài `cmd/worker` TODO có chủ đích (job đăng ký ở story sau) — hợp lệ vì worker skeleton.

**Type consistency:** `httpx.APIError`, `httpx.Cursor/Page`, `eventbus.Event/Bus/InProcess`, `config.Config`, `logger.New/Scrub` — dùng nhất quán qua các task.

---

## Execution Handoff
Sau khi lưu, chọn cách chạy: subagent-driven (khuyến nghị) hoặc inline executing-plans.
