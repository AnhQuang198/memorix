# Addendum — Memorix PRD (MVP)

> Chiều sâu kỹ thuật & "how" tách khỏi PRD (PRD giữ capability-level). Dành cho architecture/solution design downstream. Chi tiết đầy đủ ở `../../memorix-spec/`.

## Tech stack (quyết định của user)
- **Backend**: Go + **Gin** (handler chỉ bind/validate → gọi service module Clean; domain không biết Gin). sqlc/pgx, golang-migrate, River (job trong Postgres), slog, OpenTelemetry.
- **FSRS**: thư viện `go-fsrs` chính thức, **bọc sau port `Scheduler`** — không tự viết toán. Cho phép A/B FSRS-5/6/optimizer trên cùng log.
- **Frontend**: React + TS + Vite, TanStack Query (optimistic hợp FR-21), shadcn/Radix/Tailwind (a11y, khớp token design), Zod.
- **Mobile**: **PWA trước → React Native (Expo)** ở V1.5 (không Flutter — tái-dùng TS là đòn bẩy chính; đánh giá RN-vs-Flutter đầy đủ ở spec Phase 10).
- **DB**: Postgres (schema-per-module) + Redis (cache queue/stats, rate-limit) + S3-compatible (export/audio) + FTS (→OpenSearch sau).
- **Infra**: Docker + Compose (MVP, 1 VPS) → Swarm/K8s sau. Caddy (TLS), Cloudflare CDN, Prometheus/Grafana/Loki, Vault/SOPS.

## Mapping FR → cơ chế (how)
- **FR-15 idempotent grade** → `unique(card_id, client_review_id)` trên `review_logs`.
- **FR-16 replay** → `review_logs` append-only, partition theo tháng, tính lại S/D/Due từ chuỗi log theo server-ts.
- **NFR-5 nguyên tử** → 1 transaction: update `cards` + insert `review_logs`.
- **FR-25 queue priority** → `priority = w_overdue*daysOverdue + w_lowR*(1-R) + w_lapse*isRelearning + w_new*isNew`; index nóng `cards(owner_id, due_at) partial`.
- **FR-28 chống nổ** → policy trong Scheduling context: giới hạn hiển thị + rải overdue qua nhiều ngày, ưu tiên R thấp nhất.
- **NFR-9 chống gian lận** → API `POST /review/grade` chỉ nhận `{card_id, grade, client_review_id}`; server tính S/D/Due (không nhận state từ client).
- **FR-14 interval kế** → `GET /review/queue` trả `next_intervals:{again,hard,good,easy}` (client mỏng, không tự tính FSRS).

## Kiến trúc (tóm)
Modular Monolith, Clean trong core (Scheduling/Review), event bus in-process, CQRS nhẹ (read model queue/stats). Bounded context = đường cắt service khi 1M+. Chi tiết: `../../memorix-spec/07-system-architecture.md`, `08-database-design.md`, `09-api-design.md`.

## Domain (tóm)
- **Entry (nội dung) tách Card (đơn vị học/user + FSRS)** — curated dùng chung, FSRS riêng/user.
- Ubiquitous language, aggregate, state machine: `../../memorix-spec/03-domain-model.md`.

## Rejected alternatives (rationale)
- **Framework**: chi/Echo/Fiber → chọn Gin (user quyết; binding/validate sẵn, tuyển dev quen). Fiber loại (fasthttp lệch chuẩn).
- **Mobile**: Flutter loại — phá thế tái-dùng-TS; RN thắng nhờ chung stack + OTA.
- **Kiến trúc**: microservices loại ở MVP — giao dịch phân tán + ops đắt cho team nhỏ.
- **ORM**: GORM loại — query ẩn, N+1; chọn sqlc (SQL tường minh).
- **PWA trước RN**: validate rẻ trước; trần cứng = push iOS yếu → RN ở V1.5.

## Success metric — công thức North Star
`words_retained_week = count(cards recall đúng trong tuần AND next_interval ≥ N ngày)`, **N=7 (giả định, OQ-3 — chốt lại nếu dữ liệu beta gợi ý khác)**. Nguồn: `daily_stats.retained` (rebuild-được từ `review_logs`).
