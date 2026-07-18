# Phase 10 — Khuyến nghị Công nghệ

> Chọn theo: hot-path nhanh, 1 binary deploy rẻ, thư viện FSRS có sẵn. Không theo hype.

## Backend

### Ngôn ngữ — **Go**
1 binary tĩnh, concurrency native (worker/cron), p95<150ms dễ, có `go-fsrs`. Ưu: nhanh, ít RAM, ops đơn giản. Nhược: verbose. Alt: Node/TS (chung FE nhưng chậm hơn), Rust (nhanh nhất nhưng cong học dốc), Java/Spring (nặng).

### Framework — **Gin**
Framework Go phổ biến nhất; binding + validation tích hợp; nhiều ví dụ, tuyển dev quen; router radix nhanh. Nhược: `gin.Context` hơi lock-in (lệch net/http chuẩn), dễ nhét logic vào handler nếu thiếu kỷ luật. Alt: chi (thuần net/http), Echo (giống Gin), Fiber (fasthttp lệch chuẩn — tránh).
**Kỷ luật**: handler Gin **chỉ** bind/validate → gọi service module (Clean). Domain/service **không** biết Gin (test + tách dễ).
```go
func (h *Handler) Grade(c *gin.Context) {
    var req GradeRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, errEnvelope("VALIDATION_ERROR", err)); return
    }
    res, err := h.svc.GradeCard(c.Request.Context(), principal(c), req.toCmd())
    c.JSON(200, res)
}
```

### Cấu trúc & mảng backend
```
/cmd/api /cmd/worker
/internal/<module>/{domain,service,ports,handler,repo}  # module = bounded context
/internal/platform/{db,eventbus,config,logger}
/migrations
```
| Mảng | Chọn | Alt |
|---|---|---|
| DI | Wire (compile-time) / thủ công | fx (runtime) |
| Config | Viper / koanf | — |
| Validation | go-playground/validator | — |
| Auth | golang-jwt + argon2id | — |
| DB/SQL | **sqlc** (gen type-safe) + pgx | GORM (query ẩn, N+1) |
| Migration | golang-migrate / goose | — |
| Background jobs | **River** (Postgres-backed) | asynq (Redis), Temporal (nặng) |
| Logging | slog (stdlib JSON) | — |
| Metrics/Tracing | Prometheus + OpenTelemetry | — |
| API docs | OpenAPI 3 (huma/gen) | — |
| Testing | testing+testify + **testcontainers** | — |
| FSRS | **go-fsrs** bọc port | không tự viết |

## Frontend — **React + TypeScript + Vite**
Prototype đã React; hệ sinh thái lớn nhất; tuyển dev dễ; mobile sau (RN) tái dùng logic. Alt: SvelteKit (gọn nhưng cộng đồng nhỏ), Vue/Nuxt, Solid (non).
| Mảng | Chọn |
|---|---|
| Build | Vite (Next.js nếu SSR landing) |
| Routing | TanStack Router / React Router |
| Server state | **TanStack Query** (cache, optimistic — hợp grade) |
| Client state | Zustand (Redux thừa) |
| UI | **shadcn/ui + Radix + Tailwind** (a11y sẵn, khớp token) |
| Form | React Hook Form |
| Validation | **Zod** (chia sẻ schema với API) |
| Charts | Recharts / visx |
| Table | TanStack Table + Virtual |
| Icons | Lucide |
| i18n | react-i18next (ICU) |
| PWA/offline | Workbox + service worker |

## Mobile — PWA → React Native (Expo)

### PWA là gì
Web app + Service Worker (offline, cache) + Manifest (cài home screen, toàn màn hình) + Web APIs (push, IndexedDB). Cùng 1 codebase web, cài như app, ôn offline, không qua store.

### RN vs Flutter (đánh giá trực diện)
| Tiêu chí | React Native (Expo) | Flutter | Thắng |
|---|---|---|---|
| **Tái dùng code web** | Cao (chung TS, Zod, types, logic) | 0 (Dart, viết lại) | **RN** ⭐ |
| Chung team FE | 1 team lo cả | cần dev Dart | RN |
| Hiệu năng UI | Tốt (list lớn cần FlashList) | Xuất sắc mặc định | Flutter |
| Nhất quán UI 2 nền | khác nhỏ theo OS | pixel-perfect | Flutter |
| Offline SQLite | expo-sqlite/WatermelonDB | drift/sqflite | hòa |
| OTA update | **Expo EAS OTA** | không (Shorebird 3rd) | RN |
| Cong học (team) | thấp (đã React) | cao (Dart) | RN |

**Chốt: React Native (Expo)** — tái dùng TS/web là đòn bẩy lớn nhất; 1 team/1 ngôn ngữ; OTA vá không chờ store. Flutter mạnh hơn về mượt/đồng nhất nhưng **không bù** mất tái-dùng-code cho app này. Lật sang Flutter chỉ khi: team mạnh Dart, đồ họa nặng, hoặc bỏ web.

### Vì sao PWA trước, không RN luôn
PWA rẻ ~0 (đã có web), validate MVP nhanh, sửa đẩy tức thì. Trần cứng: **push iOS yếu** (mà app học sống nhờ nhắc), thiếu widget/store. → PWA cho MVP validate; RN/Expo ở **V1.5** khi retention chứng minh push quan trọng (không đợi V2).

## Database
| Loại | Chọn | Scale |
|---|---|---|
| Quan hệ | **PostgreSQL** | ACID grade; JSONB; FTS; partition; citext/unaccent |
| Cache/queue | **Redis** | cache queue/stats, rate-limit, session |
| Search | pg FTS → **OpenSearch** | tách khi phức tạp tăng |
| Object storage | S3-compatible (MinIO/R2/S3) | audio, backup |
Postgres HA/backup: read replica (đọc), Patroni/managed (failover), WAL + pgBackRest PITR (RPO<5m), partition review_logs (drop cũ O(1)). Sharding chỉ khi ~1M ép.

## Infrastructure
| Mảng | MVP | Production | Large-scale |
|---|---|---|---|
| Orchestration | Docker Compose (1 VPS) | Swarm/managed | **K8s** khi autoscale ép |
| Reverse proxy | Caddy (TLS auto) | Caddy/Nginx | Nginx/Envoy + LB |
| CDN | — | Cloudflare | Cloudflare/Fastly |
| SMTP | Resend/SES | SES/Postmark | SES |
| Monitoring | — | Prometheus+Grafana | + alertmanager |
| Logging | slog→stdout | Loki/ELK | Loki tiered |
| Tracing | — | Tempo/Jaeger (OTel) | + sampling |
| Secrets | env/SOPS | Vault/cloud | Vault |
| CI/CD | GitHub Actions | GHA + staging | GHA progressive |
**K8s KHÔNG phải MVP** — 1 VPS + Compose chạy tới hàng chục nghìn user.

## Chốt stack
| Tầng | Chốt |
|---|---|
| Backend | **Go + Gin** + sqlc/pgx + River + slog + OTel |
| FSRS | go-fsrs (bọc port) |
| Frontend | React + TS + Vite + TanStack + shadcn/Tailwind + Zod |
| Mobile | **PWA → React Native (Expo)** — không Flutter |
| DB | Postgres + Redis + S3 + (FTS→OpenSearch) |
| Infra | Docker + Compose→Swarm→K8s · Caddy · Cloudflare · Prometheus/Grafana/Loki · Vault |

Tránh: microservices sớm, K8s sớm, Flutter (phá tái-dùng-TS), GORM (query ẩn).
