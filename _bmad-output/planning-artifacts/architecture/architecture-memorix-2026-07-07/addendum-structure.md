# Addendum — Structural Mapping (Go Monolith)

> Companion cho `ARCHITECTURE-SPINE.md`. Spine giữ seed tối thiểu; đây là mapping thư mục chi tiết + cách enforce ranh giới (SEED — code sở hữu sau khi tồn tại). Governs AD-1, AD-2, AD-9, AD-10.

## Layout chốt: `cmd/internal/platform` khung + by-module + hexagonal cho lõi

```
memorix/
  cmd/
    api/main.go              # Gin server; Wire ráp adapter→port
    worker/main.go           # River jobs (reconcile, forecast, purge)
  internal/
    identity/                # auth, users, sessions, oauth  (CRUD-ish, nhẹ)
      domain/                # User, Session, VO — thuần, không import ngoài
      service/               # usecase: Register, Login, RotateRefresh, ResetPassword
      ports/                 # IdentityPort (expose ra module khác) + repo iface
      handler/               # adapter Gin (bind/validate → service)
      repo/                  # adapter pgx/sqlc
        gen/                 # sqlc-generated (không export khỏi module)
      internal/              # ⬅ ruột module: Go compiler CẤM module khác import
    vocabulary/              # entries, meanings, curated seed  (CRUD-ish, nhẹ)
      domain/ service/ ports/ handler/ repo/{gen}/ internal/
    scheduling/              # LÕI — full hexagonal
      domain/                # Card, FsrsState, Due, queue priority (thuần)
      ports/                 # SchedulerPort (bọc go-fsrs), CardRepo, VocabularyPort
      service/  handler/  repo/{gen}/  internal/
    review/                  # LÕI — full hexagonal
      domain/                # grade flow, ReviewLog (append-only)
      ports/ service/ handler/ repo/{gen}/ internal/
    progress/                # read model (nhẹ): daily_stats, streak, North Star
      domain/ service/ ports/ handler/ repo/{gen}/
    notification/            # thin ở MVP
    platform/
      db/                    # pgxpool, tx helper, unit-of-work
      eventbus/              # in-process bus (interface sẵn cho outbox V1)
      config/  logger/       # 12-factor env; slog JSON (scrub PII/token)
      authmw/                # verify JWT → principal vào context
      httpx/                 # envelope lỗi chuẩn, cursor pagination, error mapping
    shared/                  # kernel tối thiểu: ids, errors, pagination, time
  migrations/                # golang-migrate; file prefix theo schema module
  db/queries/                # sqlc .sql nguồn (theo module)
  sqlc.yaml
  web/                       # React+TS+Vite (spine FE riêng nếu cần)
```

## Quy tắc enforce (biến AD thành compiler-check, không chỉ quy ước)

| # | Quy tắc | Enforce bởi | Governs |
|---|---|---|---|
| S1 | Ruột module đặt dưới `internal/<module>/internal/` | **Go compiler** cấm import chéo | AD-1 |
| S2 | Module chỉ expose `ports/` (interface + DTO) ra ngoài | quy ước + review | AD-1, AD-9 |
| S3 | Lấy dữ liệu chéo module qua port module chủ (vd `VocabularyPort`), không join bảng khác | review + không có repo chéo | AD-9 |
| S4 | FK chỉ trong cùng schema; chéo module = cột id, không FK vật lý | migration review | AD-10 |
| S5 | `domain/` không import `service`/`repo`/`handler` hay lib hạ tầng | import-linter (depguard) | AD-2 |
| S6 | Adapter (repo/handler) ráp vào port ở `cmd/*/main.go` qua **Wire** | Wire compile-time | AD-1, AD-2 |
| S7 | sqlc gen vào `repo/gen/`, không export khỏi module | sqlc.yaml per-module | AD-1 |

## Hexagonal có chọn lọc (không đều tay)

- **Full hexagonal**: `scheduling`, `review` — nơi đổi thuật toán/hạ tầng đắt, cần A/B FSRS (AD-7), test domain nặng.
- **Nhẹ** (service + repo, port chỉ nơi expose ra ngoài): `identity`, `vocabulary`, `progress`, `notification` — CRUD-ish, boilerplate đầy đủ không đáng.
- Lý do: hexagonal đều tay = boilerplate thừa; áp đúng chỗ trade-off thật.

## Chống import cycle
- Chiều: `handler → service → {ports, domain}`; `repo → ports` (implements) `→ domain`; `domain → ∅`.
- Cross-module chỉ qua **port interface** (định nghĩa ở module gọi hoặc `shared`) + **event bus**. Không có struct module A trong module B.
- `platform` và `shared` được mọi module dùng; **không** phụ thuộc ngược vào module.

## Rejected (rationale)
- **By-technical-layer phẳng** (handlers/services/repos/models): loại — feature rải khắp, không ranh giới module, ngược AD-1.
- **Hexagonal đều tay mọi module**: loại — boilerplate thừa cho CRUD thuần.
- **`pkg/` public**: tránh — dễ thành bãi rác; dùng `internal/` + `shared/` kernel tối thiểu.

## Companion refs
- Depguard/import-linter config đặt ở `.golangci.yml` để CI chặn vi phạm S5 (fail build khi domain import hạ tầng).
- Chi tiết DB schema-per-module: `../../memorix-spec/08-database-design.md`.
