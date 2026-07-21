# CLAUDE.md — Memorix

Bối cảnh codebase cho Claude Code. Đọc trước khi sửa.

## Sản phẩm
Web app học từ vựng tiếng Anh bằng **FSRS** (spaced repetition). Định vị: "não Anki, độ mượt Duolingo". Mục tiêu: tối đa ghi nhớ, tối thiểu thời gian. UI tiếng Việt, nội dung học tiếng Anh.

Kế hoạch đầy đủ (BMAD): `_bmad-output/planning-artifacts/` — PRD, architecture spine (14 AD), epics/stories, sprint plans ở `docs/superpowers/plans/`.

## Layout (monorepo)
```
backend/   # Go modular monolith (go.mod = github.com/memorix/memorix, Go 1.26)
web/       # React 19 + Vite + TS
_bmad-output/, docs/, design/   # planning artifacts (không phải code)
```
Backend ở `backend/` — **luôn `cd backend` trước lệnh Go**.

## Kiến trúc: Modular Monolith + Hexagonal core
Module = bounded context dưới `backend/internal/`: `identity` (auth), `vocabulary` (từ/deck), `scheduling` (thẻ+FSRS+queue), `review` (grade/log), `progress` (stats), `notification` (thin), `platform` (hạ tầng dùng chung), `shared` (kernel: events).

Mỗi module: `domain/` (thuần) · `service/` (usecase) · `ports/` (interface) · `handler/` (Gin adapter) · `repo/` (pgx adapter) · `internal/` (ruột — compiler chặn import chéo).

## Invariant BẮT BUỘC (từ ARCHITECTURE-SPINE.md, đừng phá)
- **AD-1/2**: module gọi nhau CHỈ qua interface public + event bus. `domain/` import **chỉ stdlib(+uuid)** — depguard (`.golangci.yml`) chặn gin/pgx/net/http/go-fsrs trong `**/domain/**`, vi phạm = **fail CI**.
- **AD-3**: grade = **1 transaction** (`db.WithinTx`): update card + insert review_log + insert receipt. Idempotent qua `unique(card_id, client_review_id)`.
- **AD-4**: `review.review_logs` **append-only = nguồn chân lý**. Read model (progress) rebuild-được từ log (River reconcile job).
- **AD-5**: grade **server-authoritative** — client chỉ gửi `{card_id, grade, client_review_id}`; server tính S/D/Due.
- **AD-6**: `entries` (nội dung, owner_id NULL = curated dùng chung) tách `cards` (FSRS per-user). Card ref `entry_id` (logical).
- **AD-7**: FSRS chỉ qua `SchedulerPort` (bọc `go-fsrs` v3.3.1 ở `scheduling/repo/fsrsadapter` — file DUY NHẤT import go-fsrs). Fuzz off cho replay.
- **AD-8**: read model async fire-and-forget (progress ingest on `CardGraded`); số tức thì (North Star, summary) đọc **thẳng review_logs**, không lấy daily_stats lag.
- **AD-9**: cross-module đọc qua **port định nghĩa consumer-side** (vd `vocabulary/ports.CardService`, `scheduling/ports.ReviewActivityPort`, `review/ports.VocabularyPort`) — tránh import cycle. KHÔNG join/ghi bảng schema khác.
- **AD-10**: FK **chỉ trong cùng schema**; chéo module = cột id, KHÔNG FK vật lý. Postgres schema-per-module.
- **AD-12**: Due so theo server-time; "ngày học" theo TZ user (`StudyDayStart`/`StartOfStudyDay`, civil `Day`).
- **AD-14**: envelope lỗi `{error:{code,message,fields,trace_id}}` (`platform/httpx`); cursor pagination; path `/api/v1`.

## Auth contract (Sprint 1 sở hữu `platform/authmw`)
Downstream handler đọc principal qua `authmw.UserID(c) (string, bool)` (parse uuid ở ranh giới), guard route bằng `authmw.RequireAuth(jwtManager)` trên group `secured`. Test fake bằng `authmw.SetPrincipal(c, authmw.Principal{UserID:...})`. **TZ KHÔNG trong context** — lấy qua IdentityPort/TZResolver. Đừng đọc key thô `c.Get("user_id")`.

## Lệnh
```bash
# backend (cd backend)
go build ./...
go test ./... -short            # nhanh, bỏ testcontainers
go test ./...                   # đầy đủ, CẦN Docker daemon (testcontainers postgres:18)
$(go env GOPATH)/bin/golangci-lint run ./... ./cmd/...   # lint + depguard
sqlc generate                   # nếu đổi db/queries (progress block; identity block chỉ .gitkeep)

# frontend (cd web)
npm ci && npx vitest run && npm run build

# full stack
docker compose up -d --build    # api + worker + postgres:18 + redis:8
curl localhost:8080/api/v1/health   # {"status":"ok"}
```
`cmd/api` = HTTP server; `cmd/worker` = River jobs (enroll bulk-create, GDPR purge, progress reconcile).

## Migrations (golang-migrate, expand-contract)
`backend/migrations/NNNN_name.{up,down}.sql` — 0001 init(schemas+ext) · 0002 identity · 0003 vocabulary · 0004 cards · 0005 seed IELTS · 0006 fsrs(review_logs partitioned+receipts) · 0007 queue · 0008 progress.
**Đánh số kế tiếp là 0009.** River dùng migrator riêng (không chiếm số). Áp bằng `db.Migrate` (chạy mọi migration up).

## Conventions
- Test dùng `dbtest.RunPostgres(t)` (spins postgres:18 + áp mọi migration); guard `if testing.Short(){t.Skip()}` cho container test.
- Deferred Close/Rollback dùng `defer func(){ _ = x.Close() }()` (errcheck sạch).
- Commit: Conventional Commits + trailer `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.
- Card type thật: `Status CardStatus` (STRING: new/learning/review/relearning/suspended), `DueAt`/`LastReviewAt *time.Time` (pointer). go-fsrs State (int) map explicit ↔ string trong fsrsadapter.
- Frontend test cần `import "@testing-library/jest-dom/vitest"` (setupFiles rỗng).

## Trạng thái
MVP xong (E1-E6, FR-1..34): auth · vocabulary+seed · FSRS+review · smart queue+anti-flood · progress+stats. build/test/lint xanh, real-PG + p95 verified (grade 1.23ms, queue 7.84ms/10k). Xem README "Roadmap" cho V1+.

## Nợ kỹ thuật đã biết (non-blocking)
- enroll-enqueue chưa nguyên tử (dùng River InsertTx cùng tx) · prefs daily-limit=0 bị ép default · `db/queries/scheduling/queue.sql` chưa dùng (query hand-written) · progress TZ seam UTC vs DB-session (self-heal qua reconcile) · ReviewDone/gradeQueue frontend chưa wire vào tab Ôn · OAuth chưa wire runtime cmd/api (cần client secret).
