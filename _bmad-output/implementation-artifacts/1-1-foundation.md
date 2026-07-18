# Story 1.1: Nền tảng dự án & khung ứng dụng

Status: ready-for-dev

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a nhà phát triển,
I want một bộ khung Go monolith + frontend chạy được với CI/CD và deploy staging,
so that mọi tính năng sau xây trên nền nhất quán, ranh giới module rõ (enforce bằng compiler), không phải dựng lại hạ tầng.

## Acceptance Criteria

1. **Build & Health**: `go build ./...` xanh; server Gin chạy; `GET /api/v1/health` trả 200 JSON `{status:"ok"}`.
2. **Cấu trúc module**: repo theo `addendum-structure.md` — `cmd/{api,worker}`, `internal/<module>/{domain,service,ports,handler,repo,internal}`, `internal/platform`, `internal/shared`, `migrations`, `db/queries`. 6 module skeleton: identity, vocabulary, scheduling, review, progress, notification.
3. **Hạ tầng local**: Docker Compose chạy Postgres 18 + Redis 8 + api + worker; golang-migrate áp migration khởi tạo (tạo **schema-per-module** rỗng + extension, **KHÔNG** tạo bảng nghiệp vụ nào).
4. **Platform primitives**: có sẵn `slog` JSON structured (scrub PII/token), envelope lỗi chuẩn `{error:{code,message,fields,trace_id}}`, cursor pagination helper, config 12-factor (env), pgxpool, event bus in-process (interface), auth middleware skeleton (AD-14, AD-1).
5. **Enforce ranh giới**: `.golangci.yml` cấu hình depguard chặn `domain` import hạ tầng (Gin/pgx) — vi phạm = **fail CI** (AD-2, quy tắc S5); ruột module dưới `internal/<module>/internal/` (S1).
6. **Frontend shell**: React 19 + Vite 7 + TS render app shell — bottom-nav 4 tab (Trang chủ/Ôn/Thư viện/Thống kê) + theme light/dark/system + design tokens (accent `#5b5bd6`) + responsive (mobile <640 / tablet / desktop sidebar) (UX-DR1, UX-DR2, UX-DR15).
7. **CI/CD**: GitHub Actions chạy lint (golangci-lint + eslint) + test + build; deploy staging thành công (Docker image tag = commit SHA).

## Tasks / Subtasks

- [ ] **T1: Khởi tạo repo & module skeleton** (AC: #1, #2)
  - [ ] `go mod init` (Go 1.26); tạo cây thư mục theo addendum-structure
  - [ ] `cmd/api/main.go` (Gin + graceful shutdown), `cmd/worker/main.go` (River runner skeleton)
  - [ ] 6 module skeleton với thư mục `{domain,service,ports,handler,repo,internal}` (rỗng có `.gitkeep`/doc.go)
  - [ ] `internal/shared` (ids uuid, errors, pagination cursor, time helpers)
- [ ] **T2: Platform primitives** (AC: #4)
  - [ ] `platform/config` — load env (koanf/viper), typed config struct
  - [ ] `platform/logger` — slog JSON handler, middleware gán trace_id, scrub field nhạy cảm
  - [ ] `platform/httpx` — error envelope + mã lỗi chuẩn (VALIDATION_ERROR/UNAUTHENTICATED/…), cursor pagination encode/decode, error→HTTP mapping
  - [ ] `platform/db` — pgxpool init, tx/unit-of-work helper
  - [ ] `platform/eventbus` — interface `Bus{Publish/Subscribe}` + impl in-process (nền cho outbox V1 — AD-8)
  - [ ] `platform/authmw` — skeleton verify Bearer JWT → gán principal vào context (impl đầy đủ ở Story 1.4)
- [ ] **T3: Migration & schema-per-module** (AC: #3)
  - [ ] golang-migrate wiring (chạy lúc khởi động api hoặc lệnh riêng)
  - [ ] Migration `0001_init`: `CREATE SCHEMA` cho 6 module; enable extension `pgcrypto` (gen_random_uuid), `citext`, `unaccent`; **không** tạo bảng nghiệp vụ
  - [ ] Quy ước: file migration prefix theo schema (AD-13 expand-contract)
- [ ] **T4: Enforce ranh giới** (AC: #5)
  - [ ] `.golangci.yml`: bật depguard — rule cấm import `net/http`/`gin`/`pgx` trong `**/domain/**` (S5); bật các linter chuẩn (govet, staticcheck, errcheck)
  - [ ] `sqlc.yaml` per-module (gen vào `repo/gen`)
  - [ ] Wire skeleton ráp adapter→port ở `cmd/api/main.go`
- [ ] **T5: Frontend shell** (AC: #6)
  - [ ] Vite 7 + React 19 + TS scaffold trong `web/`
  - [ ] Design tokens (CSS vars light/dark, accent #5b5bd6, spacing/radius/type) — khớp prototype `design/prototype/`
  - [ ] App shell: bottom-nav 4 tab + theme switch (light/dark/system, prefers-color-scheme) + responsive breakpoints
  - [ ] Route skeleton (Home/Review/Library/Stats) render placeholder; TanStack Query + Router setup
- [ ] **T6: CI/CD & deploy** (AC: #7)
  - [ ] GitHub Actions: job lint (golangci-lint, eslint) + test (`go test ./...`, vitest) + build (go + vite) + build Docker image (tag SHA)
  - [ ] Docker Compose (dev) + Dockerfile (api, worker); deploy staging (script/manual gate)
  - [ ] README quickstart (chạy local, migrate, test)
- [ ] **T7: Verify end-to-end**
  - [ ] `docker compose up` → migrate chạy → `curl /api/v1/health` = 200
  - [ ] Frontend shell render, đổi theme hoạt động, responsive OK
  - [ ] CI pipeline xanh trên PR

## Dev Notes

### Kiến trúc phải tuân (từ Spine)
- **Paradigm**: Modular Monolith + Hexagonal core. Module = bounded context, giao tiếp **chỉ** qua interface public + event bus (AD-1). `domain` không import framework/hạ tầng (AD-2).
- **Layout & enforce**: theo `addendum-structure.md` — `internal/<module>/internal/` để **compiler cấm import chéo** (S1); depguard chặn domain import hạ tầng (S5, fail CI); Wire ráp adapter→port (S6); sqlc gen vào `repo/gen` (S7).
- **Hexagonal chọn lọc**: story sau — scheduling/review full hexagonal; identity/vocabulary/progress nhẹ. Story 1.1 chỉ dựng **skeleton đồng nhất** cho cả 6.
- **DB**: schema-per-module; FK chỉ trong cùng schema, chéo module = ref logic không FK (AD-10). Story 1.1 **chỉ tạo schema rỗng + extension**, KHÔNG tạo bảng (bảng tạo theo nhu cầu từng story sau: users@1.2, sessions@1.4…).
- **API**: envelope lỗi `{error:{code,message,fields,trace_id}}`, cursor pagination, path `/api/v1` (AD-14).
- **Event bus**: in-process interface ở platform, sẵn để nâng transactional outbox ở V1 (AD-8) — story 1.1 chỉ định nghĩa interface + impl in-process.

### Stack (đã verify July 2026 — dùng đúng version)
| Thành phần | Version | Ghi chú |
|---|---|---|
| Go | 1.26 | latest stable |
| Gin | v1.10+ | router; handler chỉ bind/validate |
| pgx / sqlc | pgx v5 / sqlc v1 | type-safe SQL, không GORM |
| golang-migrate | v4 | migration |
| River | v0 | job queue Postgres-backed (worker skeleton) |
| PostgreSQL | 18 | latest stable (18.4) |
| Redis | 8 | latest stable |
| React / Vite / TS | React 19 / Vite 7 | frontend shell |
| TanStack Query/Router · Tailwind · shadcn | latest | UI, optimistic (dùng story sau) |

### Testing standards
- Unit: `testing` + testify cho platform helpers (httpx envelope, cursor pagination, config).
- Integration: **testcontainers** (Postgres thật) — verify migration áp được + health. Đặt nền cho test repo/idempotency ở story sau.
- Frontend: vitest cho shell (theme toggle, nav render).
- CI chạy toàn bộ; fail-fast.

### Project Structure Notes
- Bám `addendum-structure.md` **chính xác** (đường dẫn, tên module, `internal/` con). Không sáng tạo layout mới.
- Greenfield — không có starter template; đây là story dựng nền (đúng nguyên tắc "foundation = Epic 1 Story 1").
- **Không** tạo bảng nghiệp vụ, **không** implement auth logic (chỉ skeleton middleware) — tránh lấn story sau, tránh forward-dep.
- Xung đột/biến thể: không có; nếu phát sinh, ghi rationale vào Dev Agent Record.

### References
- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.1] — AC gốc
- [Source: _bmad-output/planning-artifacts/architecture/architecture-memorix-2026-07-07/ARCHITECTURE-SPINE.md#AD-1,AD-2,AD-8,AD-10,AD-13,AD-14] — invariant
- [Source: _bmad-output/planning-artifacts/architecture/architecture-memorix-2026-07-07/addendum-structure.md] — layout + S1-S7 enforce
- [Source: _bmad-output/planning-artifacts/prds/prd-memorix-2026-07-07/prd.md#NFR-11,12,13,15] — a11y/i18n/observability
- [Source: design/prototype/] — design tokens, nav shell, theme tham chiếu
- [Source: memorix-spec/08-database-design.md] — schema-per-module (bảng tạo story sau)

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

### Completion Notes List

Ultimate context engine analysis completed - comprehensive developer guide created.

### File List
