# Phase 13 — Hiệu năng & Mở rộng

> Đừng tối ưu sớm cho 1M khi có 100. Nhưng data không được chặn scale. Kiến trúc đúng từ đầu (index, idempotent, read model tách), tối ưu *thêm* theo giai đoạn khi đo được.

## Caching
| Cache | Ở đâu | Invalidate |
|---|---|---|
| Queue đến hạn/user | Redis TTL 30-60s | khi grade → xóa key |
| Stats/heatmap/forecast | Redis TTL vài phút | khi ReviewSessionCompleted |
| Curated deck | Redis/CDN dài | khi publish |
| Entry curated | app+HTTP cache | ít đổi |
| Session/rate-limit | Redis | tự hết hạn |
Không cache write path (grade). Cache-aside + single-flight chống thundering herd.

## Pagination / Lazy / Batch
- **Cursor-based** (không OFFSET). Cursor = (sort_key, id), index-friendly.
- Lazy: route-split, ảnh/audio lazy, **prefetch thẻ kế** khi ôn (0 loading).
- Batch: enroll/import insert batch (COPY), rải theo daily new; stats agg batch+incremental; notification batch.

## Connection Pooling
pgxpool tuning (core×~4, min idle). **PgBouncer** (transaction mode) khi nhiều instance. Worker pool riêng (không chia conn với API hot path).

## Indexing / Query
- `cards(owner_id, due_at) partial` (nóng nhất), review_logs idempotency + partition, entries FTS, daily_stats, notifications.
- Covering index (INCLUDE) cho queue. `EXPLAIN ANALYZE`, tránh seq scan bảng nóng.
- Chống N+1 (batch load IN). Queue = 1 index scan + limit (không join nặng). Stats từ read model. Partition pruning cho review_logs.

## Background / Scheduler
- **River** (Postgres) MVP → asynq/Redis khi tải lớn. Job: enroll, import, AI-fill, notification, stats rebuild, forecast, purge. Retry+backoff+dead-letter, idempotent.
- Cron worker tách API, quét theo index due_at (không full scan), chia lô. **Load balancing lịch** (fuzz) làm phẳng tải due.

## CDN / Static
Web SPA + assets qua CDN (Cloudflare): immutable hash filename, gzip/brotli. Audio object storage + CDN edge. API không qua CDN (trừ curated public).

## Scaling Strategy
Stateless API → scale ngang sau LB. State ở Postgres/Redis (scale riêng). Đọc nóng → replica + cache. Ghi nóng (grade) → 1 primary đủ lâu (partition+index). Tách service chỉ khi đo được bottleneck.

## Tiến hóa 6 giai đoạn
| Stage | User | Hạ tầng | Data | Tối ưu | Bottleneck |
|---|---|---|---|---|---|
| 1 Local | dev | Compose, 1 process | 1 PG seed | — | không |
| 2 MVP | <1k | 1 VPS monolith+worker, Caddy | PG+Redis, backup | index nóng, cursor | không |
| 3 Prod | 1-10k | +CDN, monitoring, staging | +replica, WAL PITR, cache | cache đọc, pool | DB conn, cache hit |
| 4 10K | 10k | 2-3 API sau LB, PgBouncer | replica read, partition logs | covering index, batch | write contention nhẹ |
| 5 100K | 100k | autoscale Swarm/K8s, Redis cluster | shard logs theo time, archive, OpenSearch | precompute queue, materialized stats | PG primary write, notif fanout |
| 6 1M+ | 1M+ | **tách service** (Review-write, Notif, Stats) + broker | shard theo user_id, replica đa vùng, tiered log | CQRS đầy đủ, event-sourced, regional | cross-shard, global consistency |
**Không nhảy cóc**. Stage 2-4 vẫn 1 monolith + 1 PG primary — đủ tới ~100k. Chỉ 1M+ mới tách+shard.

## SLO
grade p95<150ms · queue(10k) p95<300ms · stats p95<200ms · uptime 99.9% · enroll 10k <30s nền.

## Scale dễ vs cần công
Dễ: API stateless, đọc (replica+cache), review_logs (partition), static/audio (CDN). Cần công: PG primary write (shard cuối), notification fanout, search (FTS→OpenSearch), global consistency multi-region.

## Cơ hội ẩn
1. read model tách sẵn → đọc scale bằng replica không đụng write.
2. review_logs partition tháng → drop/archive O(1), 1M user quản được.
3. Queue cache + single-flight → hot user không đập DB.
4. Load-balancing lịch (fuzz) → phẳng spike không thêm hạ tầng.
5. Grade nguyên tử+idempotent từ MVP → không refactor write path khi 1M.

**Chốt**: data đúng từ MVP (index nóng, cursor, idempotent, read model tách) → scale bằng thêm lớp theo 6 giai đoạn. 1 monolith + 1 PG primary phủ ~100k. Tách service+shard chỉ 1M+.
