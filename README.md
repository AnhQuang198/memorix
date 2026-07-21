# Memorix

Web app học từ vựng tiếng Anh bằng **FSRS** (spaced repetition). Bộ não của Anki, độ tinh tế của Duolingo — cho người học nghiêm túc. **Tối đa ghi nhớ, tối thiểu thời gian học.** UI tiếng Việt, nội dung học tiếng Anh.

Go modular monolith + React. MVP (E1-E6) hoàn tất.

## Tính năng (MVP)

**Xác thực & Tài khoản**
- Đăng ký email/mật khẩu + OAuth (Google/Apple), xác thực email
- Access JWT ngắn + refresh rotation + phát hiện tái dùng token (thu hồi cả family)
- Đặt lại mật khẩu (thu hồi mọi phiên), quản hồ sơ (tên/múi giờ/ngôn ngữ/theme)
- GDPR: xuất toàn bộ dữ liệu + xóa tài khoản (worker purge theo lịch)

**Từ vựng**
- Thêm từ chỉ cần term (<10s), tự tạo thẻ học; cảnh báo trùng
- Xem/sửa/xóa (soft-delete) với kiểm quyền sở hữu; danh sách cursor pagination + full-text search
- Bộ thẻ IELTS curated seed sẵn (chống cold-start); enroll → tạo thẻ hàng loạt bất đồng bộ (River job)

**Ôn tập FSRS**
- Chấm 4 mức (Again/Hard/Good/Easy) — **server tính lịch** (client không giả được)
- Chấm **nguyên tử + idempotent** (1 transaction, chống double-grade)
- Lịch sử ôn append-only, **rebuild-được** (đổi thuật toán không mất data)
- Queue đến hạn + khoảng cách ôn kế hiển thị mỗi nút; màn ôn (lật thẻ, phím 1-4/Space, optimistic, offline không mất điểm)

**Queue thông minh**
- Ưu tiên: overdue (R thấp) → relearning → review → new
- Giới hạn thẻ mới/ôn mỗi ngày (cấu hình được) + carry-over
- **Chống nổ queue** sau kỳ nghỉ (rải overdue qua ≤7 ngày)
- Luồng học thẻ mới + hướng dẫn lần đầu

**Tiến độ & Động lực**
- **North Star**: số từ nhớ được tuần này (đọc thẳng từ lịch sử ôn)
- Streak gắn recall thật (reset khi lỡ ngày, chỉ số tích lũy không reset)
- Dashboard (thẻ đến hạn + "Ôn ngay" + streak + North Star + mini heatmap + forecast mai)
- Thống kê (heatmap 90 ngày, phân bố mức chấm, retention, dự báo tải 7/30 ngày)
- Reconcile job rebuild thống kê từ nguồn chân lý; trạng thái empty/loading/error toàn app

**Nền tảng**: Modular monolith Go/Gin, Postgres schema-per-module, Redis, River jobs, depguard enforce ranh giới, Docker Compose, GitHub Actions CI. Grade p95 ~1.2ms · dựng queue 10k thẻ p95 ~8ms.

## Chạy local
```bash
docker compose up -d --build
curl localhost:8080/api/v1/health   # {"status":"ok"}
```

## Bố cục
```
backend/   # Go modular monolith (cmd/{api,worker}, internal/<module>, migrations, db)
web/       # React 19 + Vite + TS frontend
_bmad-output/, docs/, design/  # planning artifacts (PRD, architecture, epics, sprint plans)
CLAUDE.md  # context codebase cho AI / dev mới
```

## Test
```bash
cd backend && go test ./... -short          # nhanh
cd backend && go test ./...                 # đầy đủ — cần Docker (testcontainers postgres:18)
cd backend && golangci-lint run ./...       # lint + depguard (enforce ranh giới)
cd web && npm ci && npx vitest run && npm run build
```

## Kiến trúc
Modular Monolith + Hexagonal core. Module = bounded context dưới `backend/internal/`; ruột ở `internal/<module>/internal/` (compiler chặn import chéo); depguard chặn `domain` import framework/hạ tầng.
14 invariant (AD-1..14) ở `_bmad-output/planning-artifacts/architecture/architecture-memorix-2026-07-07/ARCHITECTURE-SPINE.md`. Tóm tắt cho dev: `CLAUDE.md`.

## Roadmap (nâng cấp tiếp theo)

**Dọn dẹp trước (nợ kỹ thuật)**
- Enroll-enqueue nguyên tử (River `InsertTx` cùng transaction với enrollment)
- Wire OAuth runtime vào `cmd/api` (cần client secret Google/Apple) + gating tài khoản chưa xác thực
- Wire màn Ôn end-to-end (ReviewSession + offline gradeQueue vào tab Ôn)
- Prefs daily-limit=0 (pause thẻ mới); pin DB session TZ=UTC (đồng nhất retained)

**V1 — Nội dung & Giữ chân**
- Import Anki (.apkg) / CSV — phễu "Anki refugee"
- Bộ thẻ curated đầy đủ (nhiều mục tiêu IELTS/TOEFL/Business) + duyệt theo goal
- Collection / Tag / Favorite / Search nâng cao
- Thông báo & nhắc (web push + email, quiet hours, winback)
- Heatmap/Calendar/Forecast trực quan đầy đủ

**V1.5 — Thu tiền & Mobile**
- Sync đa thiết bị (replay-from-log giải xung đột)
- AI card-fill (Claude API điền IPA/nghĩa/ví dụ/đồng nghĩa từ 1 term)
- Audio phát âm (TTS + curated)
- Subscription Pro (Stripe) + gating tính năng
- PWA offline (service worker) → sau đó React Native (Expo)

**V2 — Moat & Scale**
- FSRS optimizer weights/user (lịch cá nhân hóa từ lịch sử ôn)
- Chế độ deadline thi (dồn lịch trước ngày thi)
- Reading capture (highlight từ trong bài → auto thẻ)
- Retention outcome benchmarking (marketing "nhớ đo được")
- Tách service theo bounded context khi 1M+ user (ranh giới đã sẵn); OpenSearch cho search

## License
Private (chưa cấp phép công khai).
