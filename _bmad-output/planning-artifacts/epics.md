---
stepsCompleted: ["step-01", "step-02", "step-03", "step-04"]
inputDocuments:
  - _bmad-output/planning-artifacts/prds/prd-memorix-2026-07-07/prd.md
  - _bmad-output/planning-artifacts/architecture/architecture-memorix-2026-07-07/ARCHITECTURE-SPINE.md
  - _bmad-output/planning-artifacts/memorix-spec/06-ux-analysis.md
  - _bmad-output/planning-artifacts/ui-design-prompt.md
  - design/prototype/ (18 màn prototype: app + auth)
scope: MVP (E1-E6)
---

# Memorix - Epic Breakdown

## Overview

Bản phân rã epic/story cho **Memorix MVP (E1-E6)**, decompose từ PRD (FR/NFR), Architecture Spine (AD), và UX (spec 06 + prototype). Chỉ phạm vi MVP — V1+ (curated đầy đủ, notif, sync, AI-fill, billing) không thuộc bản này.

## Requirements Inventory

### Functional Requirements

**E1 · Xác thực & Tài khoản**
- FR-1: Đăng ký bằng email + mật khẩu hoặc OAuth (Google + Apple; Apple có thể lùi fast-follow).
- FR-2: Gửi email xác thực; tài khoản chưa xác thực bị giới hạn quyền tới khi xác thực.
- FR-3: Đăng nhập, duy trì phiên an toàn qua nhiều lần mở app.
- FR-4: Đặt lại mật khẩu qua liên kết email 1 lần, hết hạn ngắn.
- FR-5: Cập nhật hồ sơ (tên hiển thị, múi giờ, ngôn ngữ UI, theme).
- FR-6: Xuất toàn bộ dữ liệu + xóa tài khoản (GDPR).

**E2 · Từ vựng**
- FR-7: Tạo từ với chỉ term bắt buộc; nghĩa/ví dụ/IPA/syn-ant/note optional; lưu <10s.
- FR-8: Tạo từ tự tạo thẻ học (mặc định 1 hướng; cho chọn 2 hướng).
- FR-9: Xem, sửa, xóa (soft delete) từ của mình.
- FR-10: Cảnh báo khi tạo từ trùng (term chuẩn hóa, cùng chủ sở hữu), đề nghị mở từ cũ.
- FR-11: Danh sách từ với lọc theo trạng thái thẻ + sắp xếp cơ bản; danh sách lớn cuộn mượt (ảo hóa).

**E2b · Bộ thẻ khởi đầu (chống cold-start)**
- FR-11a: Cung cấp bộ thẻ khởi đầu seed sẵn (bộ nhỏ, từ vựng IELTS).
- FR-11b: Enroll bộ khởi đầu; thẻ tạo và rải theo giới hạn thẻ mới/ngày (FR-27).
- FR-11c: Onboarding gợi ý enroll bộ khởi đầu để đạt activation mà không cần tự tạo thẻ.

**E3 · Lịch FSRS**
- FR-12: Dùng FSRS tính Stability/Difficulty/Due sau mỗi lần chấm.
- FR-13: Chấm thẻ 4 mức Again/Hard/Good/Easy.
- FR-14: Hiển thị khoảng cách ôn kế cho từng mức trước khi chấm.
- FR-15: Chấm idempotent — chấm lại cùng thẻ với cùng định danh client không tạo bản ghi/không đổi lịch lần hai.
- FR-16: Ghi lịch sử mỗi lần ôn (append-only) đủ để tính lại trạng thái thẻ.
- FR-17: Đặt mục tiêu ghi nhớ mong muốn (0.80–0.97; mặc định 0.90).
- FR-18: Lịch tính theo thời gian máy chủ; "ngày học" theo múi giờ người dùng.

**E4 · Phiên ôn**
- FR-19: Bắt đầu phiên ôn, thấy thẻ đến hạn lần lượt.
- FR-20: Mặt trước tối giản (term); lật xem mặt sau đầy đủ.
- FR-21: Sau chấm, thẻ kế hiện ngay (optimistic, không chờ mạng); điểm lưu + đồng bộ nền.
- FR-22: Mất mạng khi ôn: điểm ghi cục bộ + đồng bộ sau; không bao giờ mất điểm.
- FR-23: Chấm nhanh bằng bàn phím (mức 1–4, Space lật).
- FR-24: Hết thẻ → màn tổng kết ăn mừng (số từ nhớ được, forecast mai).

**E5 · Queue & Giới hạn ngày**
- FR-25: Dựng queue theo ưu tiên: overdue nặng (R thấp) → relearning → review đến hạn → thẻ mới.
- FR-26: Đặt giới hạn thẻ mới/ngày và thẻ ôn/ngày (mặc định 20 / 200).
- FR-27: Thẻ mới rải theo giới hạn ngày, không đổ hết vào queue.
- FR-28: Chống nổ queue: sau nghỉ dài, giới hạn số thẻ hiển thị (mặc định ~2× review limit trong overdue, ưu tiên R thấp) và rải phần dư qua ≤7 ngày.
- FR-29: Học thẻ mới (luồng riêng) + hướng dẫn ngắn lần đầu về cách chấm.

**E6 · Thống kê & North Star**
- FR-30: Trang chủ: số thẻ đến hạn + CTA "Ôn ngay", số thẻ mới hôm nay, streak.
- FR-31: Hiển thị North Star (số từ nhớ được tuần này) nổi bật.
- FR-32: Streak gắn recall thật (dọn due + nhớ được); reset khi lỡ ngày nhưng chỉ số ghi nhớ tích lũy không reset.
- FR-33: Thống kê cơ bản: đã ôn hôm nay, phân bố mức chấm, dự báo tải 7 & 30 ngày tới.
- FR-34: Mọi màn có empty/loading/error; màn "hết thẻ" ăn mừng thay vì trống.

### NonFunctional Requirements

- NFR-1: Chấm thẻ (tính lịch + ghi log) p95 < 150ms.
- NFR-2: Dựng queue cho 10k thẻ p95 < 500ms.
- NFR-3: Không loading giữa các thẻ khi ôn (prefetch thẻ kế).
- NFR-4: Không mất bản ghi ôn (RPO < 5 phút).
- NFR-5: Chấm thẻ nguyên tử (cập nhật trạng thái + ghi log cùng 1 giao dịch).
- NFR-6: Trạng thái FSRS tính lại được từ lịch sử ôn (nguồn chân lý).
- NFR-7: Mật khẩu băm mạnh; access ngắn + refresh xoay vòng + phát hiện tái dùng.
- NFR-8: Kiểm quyền sở hữu mọi tài nguyên cá nhân; deny-by-default.
- NFR-9: Chống gian lận lịch: máy chủ tính điểm; client chỉ gửi mức chấm.
- NFR-10: Tuân OWASP Top 10; rate-limit theo tầng (đăng nhập chặt, ôn nới).
- NFR-11: Web responsive mobile-first; màn ôn nhất quán mọi kích thước.
- NFR-12: WCAG 2.1 AA: điều hướng bàn phím (1–4), contrast AA 2 theme, screen reader đọc term/nghĩa/IPA, tôn trọng prefers-reduced-motion.
- NFR-13: Dark/Light/System theme; i18n UI (vi/en); nội dung học tiếng Anh không dịch.
- NFR-14: GDPR: xuất toàn bộ + xóa tài khoản; không log PII/token.
- NFR-15: Logging/metrics/trace đủ chẩn đoán hot path.
- NFR-16: Uptime mục tiêu 99.9%.

### Additional Requirements

*(Từ Architecture Spine — không có starter template; greenfield Go monolith. Foundation phải làm trước ở Epic 1.)*

- ARCH-1: **Foundation/greenfield setup** — repo Go, cấu trúc module (identity/vocabulary/scheduling/review/progress/notification + platform), CI/CD (GitHub Actions), Docker Compose, deploy staging. *(→ Epic 1 Story 1)*
- ARCH-2: Modular Monolith + Hexagonal core; module giao tiếp qua interface + event bus in-process (AD-1, AD-2).
- ARCH-3: Postgres schema-per-module; FK chỉ trong cùng schema, chéo module = ref logic (AD-10); migration expand-contract golang-migrate (AD-13).
- ARCH-4: Grade nguyên tử + idempotent `unique(card_id, client_review_id)` (AD-3); ReviewLog append-only = nguồn chân lý, replay-được (AD-4).
- ARCH-5: FSRS qua port bọc go-fsrs (AD-7); scheduling server-authoritative, client chỉ gửi grade (AD-5).
- ARCH-6: Entry (nội dung) tách Card (FSRS/user); curated owner_id NULL dùng chung (AD-6).
- ARCH-7: Read model Progress async eventual; MVP fire-and-forget + job reconcile rebuild daily_stats; số tức thì đọc thẳng review_logs (AD-8).
- ARCH-8: Auth JWT stateless 15m + refresh rotation/reuse-detect + OAuth linking trong Identity (AD-11).
- ARCH-9: Thời gian: Due theo server-time, "ngày học" theo TZ user (AD-12).
- ARCH-10: API envelope lỗi chuẩn + cursor pagination + `/api/v1` (AD-14).
- ARCH-11: Backup WAL + PITR (pgBackRest) RPO<5m từ prod (NFR-4); Prometheus/Grafana/Loki prod.
- ARCH-12: Job runner River (Postgres-backed) cho reconcile/forecast/purge.

### UX Design Requirements

*(Từ spec 06 + ui-design-prompt + prototype 18 màn. Accent #5b5bd6, mobile-first, light/dark, WCAG AA, UI tiếng Việt/nội dung Anh.)*

- UX-DR1: **Design tokens** — hệ màu light + dark (accent #5b5bd6, neutral, semantic 4 grade Again/Hard/Good/Easy + trạng thái thẻ New/Learning/Review/Suspended); type scale; spacing/radius; touch target ≥44px.
- UX-DR2: **Bottom tab navigation** 4 mục (Trang chủ · Ôn · Thư viện · Thống kê) + FAB "＋ thêm từ"; badge số due trên tab Ôn; desktop sidebar + top bar.
- UX-DR3: **Màn Review** — front (term to + "Lật thẻ" + progress), back (IPA/audio/nghĩa/ví dụ/syn-ant), 4 nút grade **hiện interval kế dưới mỗi nút**; phím 1–4 + Space; optimistic advance; prefetch thẻ kế (0 loading).
- UX-DR4: **Màn Home/Dashboard** — due card + "Ôn ngay", new-today, streak, North Star "+N từ nhớ được", mini heatmap, forecast mai.
- UX-DR5: **Màn Learn** — như Review + mini-onboarding lần đầu giải thích 4 nút; loading skeleton state.
- UX-DR6: **Màn Vocabulary List** — search + filter chip (status/tag/favorite) + sort; row (term/POS/badge trạng thái/audio); virtual scroll; bulk-select.
- UX-DR7: **Màn Vocabulary Detail** — entry đầy đủ + card stats (S/D/due/lapse/lịch sử) + actions (sửa/favorite/suspend/reset/xóa).
- UX-DR8: **Màn Add/Edit Entry** — term bắt buộc, còn lại optional; chọn hướng thẻ/tag; lưu <10s.
- UX-DR9: **Màn Statistics** — heatmap, streak, retention 30/90/180d, North Star, forecast tải, phân bố S/D.
- UX-DR10: **Màn Settings** — cấu hình lịch (desired-retention slider, daily new/review limit), theme, ngôn ngữ, export/xóa account.
- UX-DR11: **Màn Auth** — welcome/splash, login, signup (độ mạnh mật khẩu), verify email (OTP), forgot, reset, onboarding chọn mục tiêu.
- UX-DR12: **Empty states** mỗi màn — Review-done ăn mừng North Star; Library empty CTA; Search no-result.
- UX-DR13: **Loading states** — skeleton (không spinner trắng); optimistic cho grade/add/favorite.
- UX-DR14: **Error states** — offline khi ôn = banner non-blocking "điểm đã lưu, sẽ sync"; không bao giờ mất grade/nháp; 401 refresh ngầm.
- UX-DR15: **Responsive** — mobile <640 (bottom tab, nút chấm thumb-reach), tablet 640-1024 (master-detail), desktop >1024 (sidebar); Review nhất quán mọi kích thước.
- UX-DR16: **Accessibility WCAG AA** — keyboard nav (phím 1-4 chấm, Space lật), focus visible, contrast AA 2 theme, screen reader term/nghĩa/IPA, prefers-reduced-motion.

### FR Coverage Map

| FR | Epic | Mô tả ngắn |
|---|---|---|
| FR-1..6 | Epic 1 | Auth: đăng ký/đăng nhập/verify/reset/hồ sơ/GDPR |
| FR-7 | Epic 2 | Tạo từ term-only <10s |
| FR-8 | Epic 2 | Tạo từ → tự tạo thẻ |
| FR-9 | Epic 2 | Xem/sửa/xóa từ |
| FR-10 | Epic 2 | Cảnh báo trùng term |
| FR-11 | Epic 2 | Danh sách từ + lọc/sort/virtual |
| FR-11a | Epic 2 | Bộ khởi đầu seed IELTS |
| FR-11b | Epic 2 | Enroll bộ khởi đầu, rải new |
| FR-11c | Epic 2 | Onboarding gợi ý enroll |
| FR-12 | Epic 3 | FSRS tính S/D/Due |
| FR-13 | Epic 3 | Chấm 4 mức |
| FR-14 | Epic 3 | Hiện interval kế mỗi mức |
| FR-15 | Epic 3 | Chấm idempotent |
| FR-16 | Epic 3 | ReviewLog append-only |
| FR-17 | Epic 3 | Desired retention |
| FR-18 | Epic 3 | Server-time + TZ user |
| FR-19 | Epic 3 | Bắt đầu phiên ôn |
| FR-20 | Epic 3 | Front/back lật thẻ |
| FR-21 | Epic 3 | Optimistic advance |
| FR-22 | Epic 3 | Offline không mất grade |
| FR-23 | Epic 3 | Phím 1-4 + Space |
| FR-24 | Epic 3 | Màn tổng kết ăn mừng |
| FR-25 | Epic 4 | Queue ưu tiên |
| FR-26 | Epic 4 | Giới hạn new/review |
| FR-27 | Epic 4 | Rải thẻ mới |
| FR-28 | Epic 4 | Chống nổ queue |
| FR-29 | Epic 4 | Học thẻ mới + hướng dẫn |
| FR-30 | Epic 5 | Trang chủ due+CTA+streak |
| FR-31 | Epic 5 | North Star |
| FR-32 | Epic 5 | Streak gắn recall thật |
| FR-33 | Epic 5 | Thống kê + forecast 7/30d |
| FR-34 | Epic 5 | Empty/loading/error states |

## Epic List

### Epic 1: Xác thực & Nền tảng tài khoản
Người dùng đăng ký, đăng nhập, quản hồ sơ an toàn; MVP có nền tảng greenfield (repo/module/CI/deploy) + khung app + design tokens.
**FRs covered:** FR-1, FR-2, FR-3, FR-4, FR-5, FR-6
**ARCH:** ARCH-1, ARCH-2, ARCH-3, ARCH-8, ARCH-10, ARCH-11 · **UX-DR:** UX-DR1, UX-DR2, UX-DR11, UX-DR15, UX-DR16

### Epic 2: Từ vựng, Thẻ & Bộ khởi đầu
Người dùng thêm/sửa/xóa từ và enroll bộ IELTS seed để có nội dung học ngay (chống cold-start).
**FRs covered:** FR-7, FR-8, FR-9, FR-10, FR-11, FR-11a, FR-11b, FR-11c
**ARCH:** ARCH-6 · **UX-DR:** UX-DR6, UX-DR7, UX-DR8, UX-DR12

### Epic 3: Ôn tập với FSRS
Người dùng ôn thẻ đến hạn, chấm 4 mức, FSRS xếp lịch đúng — vòng học lõi (scheduling + review).
**FRs covered:** FR-12, FR-13, FR-14, FR-15, FR-16, FR-17, FR-18, FR-19, FR-20, FR-21, FR-22, FR-23, FR-24
**ARCH:** ARCH-4, ARCH-5, ARCH-7, ARCH-9 · **UX-DR:** UX-DR3, UX-DR5, UX-DR13, UX-DR14

### Epic 4: Queue thông minh & Giới hạn ngày
Queue ưu tiên, giới hạn new/review, chống nổ queue sau nghỉ, học thẻ mới.
**FRs covered:** FR-25, FR-26, FR-27, FR-28, FR-29
**ARCH:** ARCH-7, ARCH-12

### Epic 5: Tiến độ & Động lực
Trang chủ, North Star, streak thật, forecast, thống kê — vòng thói quen.
**FRs covered:** FR-30, FR-31, FR-32, FR-33, FR-34
**ARCH:** ARCH-7 · **UX-DR:** UX-DR4, UX-DR9, UX-DR10, UX-DR12

---

## Epic 1: Xác thực & Nền tảng tài khoản

Người dùng đăng ký, xác thực, đăng nhập và quản hồ sơ an toàn. Story 1.1 dựng nền tảng greenfield (repo, module, CI/CD, migrate, app shell + design tokens) để mọi story sau xây trên đó.

### Story 1.1: Nền tảng dự án & khung ứng dụng

As a nhà phát triển,
I want một bộ khung Go monolith + frontend chạy được với CI/CD và deploy staging,
So that mọi tính năng sau xây trên nền nhất quán, ranh giới module rõ.

**Acceptance Criteria:**

**Given** repo trống
**When** khởi tạo dự án theo Spine (cmd/api, cmd/worker, internal/<module>, internal/platform, migrations)
**Then** `go build ./...` xanh và server Gin chạy, trả `GET /api/v1/health` 200
**And** Postgres + Redis chạy qua Docker Compose, golang-migrate áp migration khởi tạo (schema-per-module rỗng)
**And** frontend React+Vite render app shell: bottom-nav 4 tab + theme light/dark/system + design tokens (accent #5b5bd6), responsive mobile <640 / tablet / desktop sidebar — UX-DR1, UX-DR2, UX-DR15
**And** GitHub Actions chạy lint + test + build; deploy staging thành công
**And** slog JSON structured + envelope lỗi chuẩn `{error:{code,message,fields,trace_id}}` + cursor pagination helper có sẵn (AD-14)

### Story 1.2: Đăng ký bằng email và mật khẩu

As a người học mới,
I want tạo tài khoản bằng email + mật khẩu,
So that tôi có không gian học của riêng mình.

**Acceptance Criteria:**

**Given** tôi ở màn đăng ký
**When** nhập email hợp lệ + mật khẩu ≥8 ký tự và gửi
**Then** tài khoản được tạo, mật khẩu băm argon2id (không lưu raw), bảng `identity.users` có bản ghi
**And** trả access token + phát email xác thực
**Given** email đã tồn tại
**When** đăng ký lại
**Then** trả 409 CONFLICT với message không tiết lộ chi tiết vượt mức
**Given** mật khẩu yếu (zxcvbn score <2)
**When** gửi
**Then** trả 400 VALIDATION_ERROR nêu rõ trường

### Story 1.3: Xác thực email

As a người dùng mới đăng ký,
I want xác thực email qua liên kết,
So that tài khoản được tin cậy và mở đầy đủ quyền.

**Acceptance Criteria:**

**Given** tôi vừa đăng ký
**When** hệ thống gửi email
**Then** một token hash 1-lần TTL 24h lưu ở `identity.email_tokens (kind=verify)`
**Given** tôi mở liên kết xác thực hợp lệ
**When** hệ thống nhận token
**Then** `users.email_verified_at` được set, token đánh dấu used, không dùng lại được
**Given** tài khoản chưa xác thực
**When** gọi API cần quyền đầy đủ
**Then** bị giới hạn theo chính sách (FR-2) tới khi xác thực

### Story 1.4: Đăng nhập và duy trì phiên

As a người dùng đã đăng ký,
I want đăng nhập và giữ phiên an toàn,
So that tôi không phải đăng nhập lại mỗi lần mở app.

**Acceptance Criteria:**

**Given** thông tin đúng
**When** đăng nhập
**Then** nhận access JWT (15m) + refresh token (opaque, hash lưu `identity.sessions`, cookie httpOnly+Secure+SameSite=Strict)
**Given** access token hết hạn
**When** gọi `/auth/refresh` với cookie
**Then** cấp cặp token mới, xoay vòng, vô hiệu token cũ (rotation)
**Given** refresh token cũ bị dùng lại
**When** phát hiện
**Then** thu hồi cả family_id (reuse-detection) — AD-11, NFR-7
**Given** đăng nhập sai N lần
**When** vượt ngưỡng
**Then** rate-limit + backoff; message không phân biệt email tồn tại (chống enumeration)

### Story 1.5: Đăng nhập bằng Google/Apple (OAuth)

As a người học,
I want đăng nhập bằng Google hoặc Apple,
So that tôi vào nhanh không cần nhớ mật khẩu.

**Acceptance Criteria:**

**Given** tôi chọn đăng nhập Google/Apple
**When** hoàn tất luồng Authorization Code + PKCE
**Then** hệ thống verify id_token (sig/aud/iss + state/nonce) và link `identity.oauth_identities (provider, provider_uid)`
**Given** provider_uid chưa liên kết
**When** đăng nhập lần đầu
**Then** tạo/liên kết user; không tự-merge bằng email chưa verified (AD-11)
**And** trả token như luồng đăng nhập chuẩn

### Story 1.6: Đặt lại mật khẩu

As a người dùng quên mật khẩu,
I want đặt lại qua email,
So that tôi lấy lại quyền truy cập an toàn.

**Acceptance Criteria:**

**Given** tôi yêu cầu đặt lại với email
**When** gửi
**Then** phát token hash 1-lần TTL 1h (`kind=reset`); response giống nhau dù email tồn tại hay không (FR-4)
**Given** tôi mở liên kết + đặt mật khẩu mới hợp lệ
**When** xác nhận
**Then** cập nhật mật khẩu, token used, **thu hồi mọi session hiện có**

### Story 1.7: Quản lý hồ sơ

As a người dùng,
I want cập nhật hồ sơ và tùy chọn,
So that app hoạt động đúng múi giờ, ngôn ngữ, giao diện của tôi.

**Acceptance Criteria:**

**Given** tôi đã đăng nhập
**When** cập nhật tên hiển thị / múi giờ / ngôn ngữ UI / theme
**Then** lưu vào `users`, áp dụng ngay (theme + i18n) — UX-DR11, NFR-13
**And** múi giờ dùng cho tính "ngày học" downstream (AD-12)

### Story 1.8: Xuất dữ liệu và xóa tài khoản (GDPR)

As a người dùng,
I want xuất toàn bộ dữ liệu và xóa tài khoản,
So that tôi kiểm soát dữ liệu của mình và tin tưởng sản phẩm.

**Acceptance Criteria:**

**Given** tôi đã đăng nhập
**When** yêu cầu xuất dữ liệu
**Then** nhận file JSON toàn bộ dữ liệu của mình (sau xác thực lại) — NFR-14
**Given** tôi yêu cầu xóa tài khoản
**When** xác nhận
**Then** tài khoản soft-delete → purge theo lịch, thu hồi mọi session, không log PII/token

---

## Epic 2: Từ vựng, Thẻ & Bộ khởi đầu

Người dùng thêm/sửa/xóa từ và enroll bộ IELTS seed để có nội dung học ngay. Xây trên auth (Epic 1).

### Story 2.1: Thêm từ nhanh và tự tạo thẻ

As a người học,
I want thêm một từ chỉ cần nhập term,
So that tôi ghi lại từ mới trong vài giây mà không bị cản.

**Acceptance Criteria:**

**Given** tôi đã đăng nhập ở màn thêm từ
**When** nhập term và lưu (các trường nghĩa/ví dụ/IPA/syn-ant/note để trống)
**Then** tạo `vocabulary.entries` với `term_normalized` (unaccent+lower), `owner_id = tôi`; thao tác <10s qua form Add/Edit (term bắt buộc, còn lại optional) (FR-7, UX-DR8)
**And** tự tạo `scheduling.cards` trạng thái New, mặc định hướng front→back (chọn 2 hướng được) (FR-8)
**And** Card tham chiếu `entry_id` (ref logic, không FK chéo schema — AD-10)
**Given** tôi nhập thêm nghĩa/ví dụ/IPA
**When** lưu
**Then** lưu vào bảng con (`meanings/examples/pronunciations/synonyms_antonyms`), escape HTML

### Story 2.2: Cảnh báo từ trùng

As a người học,
I want được cảnh báo khi thêm từ đã có,
So that tôi không tạo trùng và mở lại từ cũ.

**Acceptance Criteria:**

**Given** tôi đã có từ với `term_normalized` = X
**When** thêm từ mới cũng chuẩn hóa thành X (khác dấu/hoa thường vẫn tính trùng)
**Then** trả 409 CONFLICT + gợi ý mở entry hiện có (FR-10)
**And** không tạo bản ghi trùng

### Story 2.3: Xem, sửa, xóa từ

As a người học,
I want xem chi tiết, sửa và xóa từ của mình,
So that tôi giữ nội dung học chính xác và gọn.

**Acceptance Criteria:**

**Given** tôi mở chi tiết một từ
**When** màn detail hiển thị
**Then** thấy entry đầy đủ + card stats (S/D/due/lapse/lịch sử) + actions (UX-DR7)
**Given** tôi sửa trường bất kỳ
**When** lưu
**Then** cập nhật entry, giữ nguyên trạng thái FSRS của card
**Given** tôi xóa từ
**When** xác nhận
**Then** soft delete (`deleted_at`); card + log giữ tới purge; từ biến khỏi danh sách (FR-9)
**And** chỉ chủ sở hữu thao tác được (ownership check — NFR-8)

### Story 2.4: Danh sách từ với lọc và sắp xếp

As a người học có nhiều từ,
I want lọc, sắp xếp và cuộn danh sách mượt,
So that tôi tìm và quản lý từ dễ dàng.

**Acceptance Criteria:**

**Given** tôi ở màn Thư viện
**When** danh sách tải
**Then** hiển thị row (term/POS/badge trạng thái FSRS) với cursor pagination; cuộn ảo hóa mượt cho 10k+ từ (FR-11, UX-DR6)
**Given** tôi chọn lọc theo trạng thái thẻ / sắp xếp
**When** áp dụng
**Then** danh sách cập nhật đúng, filter whitelist
**Given** tôi chưa có từ nào
**When** mở Thư viện
**Then** empty state với CTA thêm từ / enroll bộ khởi đầu (UX-DR12)

### Story 2.5: Bộ thẻ khởi đầu và enroll

As a người học mới,
I want enroll bộ IELTS seed sẵn,
So that tôi có nội dung học ngay mà không phải tự tạo thẻ.

**Acceptance Criteria:**

**Given** hệ thống có bộ khởi đầu seed (`curated_decks` + `entries` owner_id NULL) (FR-11a, AD-6)
**When** tôi enroll bộ khởi đầu
**Then** tạo `deck_enrollments` + bulk tạo `cards` (New) cho tôi qua job nền, idempotent (FR-11b)
**And** thẻ tạo ở trạng thái New (chưa đổ vào due queue); tốc độ vào học tôn trọng giới hạn thẻ mới (thực thi ở Epic 4)
**Given** tôi đã enroll bộ này
**When** enroll lại
**Then** trả 409 CONFLICT, không nhân đôi

### Story 2.6: Onboarding gợi ý enroll

As a người dùng mới sau đăng ký,
I want được dẫn chọn mục tiêu và enroll bộ khởi đầu,
So that tôi đạt phiên ôn đầu (activation) nhanh.

**Acceptance Criteria:**

**Given** tôi vừa xác thực và lần đầu vào app
**When** onboarding chạy
**Then** hỏi mục tiêu, gợi ý enroll bộ khởi đầu, dẫn tới phiên ôn đầu (FR-11c, UX-DR11)
**Given** tôi bỏ qua
**When** chọn skip
**Then** vào dashboard trống với CTA rõ ràng (không kẹt)

---

## Epic 3: Ôn tập với FSRS

Người dùng ôn thẻ đến hạn, chấm 4 mức, FSRS xếp lịch đúng — vòng học lõi. Xây trên thẻ từ Epic 2.

### Story 3.1: Chấm thẻ với FSRS (nguyên tử, idempotent, server-authoritative)

As a người học,
I want mỗi lần chấm được FSRS tính lịch đúng và lưu chắc chắn,
So that thẻ được lên lịch tối ưu và không bao giờ hỏng dữ liệu.

**Acceptance Criteria:**

**Given** một thẻ và mức chấm (1–4) + `client_review_id`
**When** gọi `POST /api/v1/review/grade` với `{card_id, grade, client_review_id}` (client KHÔNG gửi S/D)
**Then** server tính S/D/Due qua port FSRS (bọc go-fsrs) và cập nhật card (AD-5, AD-7)
**And** update `cards` + insert `review.review_logs` trong **1 transaction** (FR-12, FR-13, FR-16, AD-3, NFR-5)
**Given** gửi lại cùng `(card_id, client_review_id)`
**When** trùng
**Then** trả kết quả cũ, không chấm lại, không tạo log trùng (FR-15, unique constraint)
**And** `review_logs` chỉ append (không update/delete), đủ để replay lại trạng thái (AD-4, NFR-6)
**And** chấm p95 < 150ms (NFR-1)

### Story 3.2: Mục tiêu ghi nhớ và thời gian theo múi giờ

As a người học,
I want đặt mục tiêu ghi nhớ và app tính "ngày học" theo múi giờ tôi,
So that lịch ôn khớp thói quen và không sai vì lệch đồng hồ.

**Acceptance Criteria:**

**Given** tôi mở cấu hình lịch
**When** đặt desired retention (0.80–0.97)
**Then** lưu `user_scheduler_prefs`, áp cho lịch tính về sau (không rewrite quá khứ) (FR-17)
**Given** tính Due và "ngày học"
**When** hệ thống so hạn
**Then** Due so theo server-time; "ngày học" theo TZ user, grace qua nửa đêm (FR-18, AD-12)

### Story 3.3: Lấy thẻ đến hạn kèm khoảng cách ôn kế

As a người học,
I want mở phiên ôn và thấy thẻ đến hạn với khoảng cách ôn kế mỗi mức,
So that tôi ôn đúng thẻ và hiểu hệ quả mỗi lựa chọn.

**Acceptance Criteria:**

**Given** tôi bắt đầu phiên ôn
**When** gọi `GET /api/v1/review/queue`
**Then** trả thẻ due ≤ now kèm nội dung entry (batch-load qua VocabularyPort, không join hot path — AD-9) (FR-19)
**And** mỗi thẻ kèm `next_intervals:{again,hard,good,easy}` do server tính (FR-14)
*(Sắp xếp ưu tiên nâng cao + giới hạn ngày thuộc Epic 4; ở đây trả thẻ due cơ bản.)*

### Story 3.4: Màn ôn — lật thẻ và chấm

As a người học,
I want lật thẻ và chấm nhanh bằng chạm hoặc bàn phím,
So that tôi ôn vụn hiệu quả trong vài phút.

**Acceptance Criteria:**

**Given** thẻ hiện mặt trước (term) + nút "Lật thẻ" + progress
**When** tôi lật
**Then** hiện mặt sau đầy đủ (IPA/nghĩa/ví dụ/syn-ant) (FR-20, UX-DR3)
**When** tôi chấm 1 trong 4 mức
**Then** thẻ được chấm và chuyển thẻ kế
**And** hỗ trợ phím 1–4 (chấm) + Space (lật) (FR-23, UX-DR16, NFR-12)

### Story 3.5: Optimistic và không mất điểm khi offline

As a người học,
I want chấm mượt không chờ mạng và không mất điểm khi mất kết nối,
So that trải nghiệm ôn nhanh và đáng tin.

**Acceptance Criteria:**

**Given** tôi chấm một thẻ
**When** mạng bình thường
**Then** thẻ kế hiện ngay (optimistic), điểm đồng bộ nền; prefetch thẻ kế nên không loading giữa các thẻ (FR-21, NFR-3)
**Given** mất mạng khi ôn
**When** tôi chấm
**Then** điểm ghi cục bộ (hàng đợi) + banner non-blocking "điểm đã lưu, sẽ sync"; đồng bộ idempotent khi có mạng; **không bao giờ mất điểm** (FR-22, UX-DR14)

### Story 3.6: Màn tổng kết cuối phiên

As a người học,
I want thấy tổng kết ăn mừng khi hết thẻ,
So that tôi cảm nhận giá trị mỗi phiên và biết tải ngày mai.

**Acceptance Criteria:**

**Given** queue rỗng sau khi ôn
**When** phiên kết thúc
**Then** hiện màn ăn mừng: số từ nhớ được phiên này (đọc thẳng `review_logs` của phiên — AD-8) + forecast mai (FR-24, UX-DR3)
**And** không hiển thị màn trống (FR-34)

---

## Epic 4: Queue thông minh & Giới hạn ngày

Queue ưu tiên, giới hạn ngày, chống nổ queue, học thẻ mới. Xây trên vòng ôn (Epic 3).

### Story 4.1: Queue ưu tiên

As a người học,
I want queue sắp thẻ theo mức cấp thiết,
So that tôi cứu thẻ sắp quên trước và ôn hiệu quả nhất.

**Acceptance Criteria:**

**Given** tôi có thẻ overdue, relearning, review-due và new
**When** dựng queue
**Then** sắp theo ưu tiên: overdue nặng (R thấp) → relearning → review đến hạn → new (FR-25)
**And** dùng index `cards(owner_id, due_at)`; dựng queue 10k thẻ p95 < 500ms (NFR-2)

### Story 4.2: Giới hạn thẻ mới và thẻ ôn mỗi ngày

As a người học,
I want đặt giới hạn số thẻ mới/ôn mỗi ngày,
So that tôi kiểm soát tải học và không quá sức.

**Acceptance Criteria:**

**Given** cấu hình mặc định (20 new / 200 review)
**When** tôi đổi giới hạn (1–9999)
**Then** lưu `user_scheduler_prefs`, queue tôn trọng giới hạn (FR-26)
**Given** vượt giới hạn review/ngày
**When** còn thẻ due
**Then** giữ phần dư sang ngày sau (không bỏ)

### Story 4.3: Rải thẻ mới theo giới hạn

As a người học vừa thêm/enroll nhiều thẻ,
I want thẻ mới được rải đều theo ngày,
So that tôi không bị ngợp và review không nổ về sau.

**Acceptance Criteria:**

**Given** tôi có nhiều thẻ New (tự thêm hoặc enroll bộ khởi đầu)
**When** vào học mỗi ngày
**Then** chỉ tối đa `daily_new_limit` thẻ mới vào học/ngày; phần còn lại chờ (FR-27, FR-11b)

### Story 4.4: Chống nổ queue sau kỳ nghỉ

As a người học vừa nghỉ vài ngày,
I want queue không dồn hết thẻ overdue,
So that tôi quay lại mà không bị ngợp và bỏ cuộc.

**Acceptance Criteria:**

**Given** tôi nghỉ nhiều ngày, overdue tích lũy lớn
**When** mở phiên ôn
**Then** hiển thị tối đa ~2× giới hạn review trong overdue (ưu tiên R thấp nhất), rải phần dư qua ≤7 ngày (FR-28)
**And** không dồn toàn bộ overdue vào một ngày

### Story 4.5: Học thẻ mới với hướng dẫn lần đầu

As a người học mới,
I want luồng học thẻ mới có hướng dẫn cách chấm,
So that tôi hiểu 4 mức và bắt đầu tự tin.

**Acceptance Criteria:**

**Given** tôi vào học thẻ mới lần đầu
**When** màn Learn mở
**Then** hiện mini-onboarding giải thích Again/Hard/Good/Easy (UX-DR5)
**And** cấp thẻ New tới `daily_new_limit`, có loading skeleton (FR-29, UX-DR13)

---

## Epic 5: Tiến độ & Động lực

Trang chủ, North Star, streak thật, forecast, thống kê — vòng thói quen. Xây trên lịch sử ôn (Epic 3-4).

### Story 5.1: Read model tiến độ và job reconcile

As a hệ thống,
I want cập nhật read model tiến độ từ sự kiện ôn và tự chỉnh sai lệch,
So that thống kê nhanh và luôn đúng theo nguồn chân lý.

**Acceptance Criteria:**

**Given** một thẻ được chấm (event `CardGraded`)
**When** event phát
**Then** cập nhật `progress.daily_stats` bất đồng bộ (ngoài TX grade) — fire-and-forget in-process (AD-8, ARCH-7)
**Given** job reconcile định kỳ (River)
**When** chạy
**Then** rebuild `daily_stats` từ `review_logs` để chữa drift (AD-4)

### Story 5.2: Trang chủ

As a người học,
I want trang chủ cho tôi biết ngay cần làm gì,
So that tôi bắt đầu ôn trong một chạm.

**Acceptance Criteria:**

**Given** tôi mở app
**When** trang chủ tải
**Then** hiện số thẻ đến hạn + CTA "Ôn ngay", số thẻ mới hôm nay, streak (FR-30, UX-DR4)
**And** badge số due trên tab Ôn (UX-DR2)

### Story 5.3: North Star — số từ nhớ được

As a người học,
I want thấy số từ mình thật sự nhớ được tuần này,
So that tôi tin sản phẩm đang dạy tôi hiệu quả.

**Acceptance Criteria:**

**Given** tôi có lịch sử ôn tuần này
**When** xem trang chủ/thống kê
**Then** hiện North Star = số thẻ recall đúng với interval kế ≥ 7 ngày (FR-31)
**And** số tức thì đọc thẳng `review_logs` (không lấy daily_stats lag — AD-8)

### Story 5.4: Streak gắn recall thật

As a người học,
I want streak phản ánh việc học thật, không phải chỉ mở app,
So that động lực của tôi trung thực.

**Acceptance Criteria:**

**Given** tôi dọn thẻ due và nhớ được trong ngày
**When** cập nhật streak
**Then** streak tăng dựa recall thật (FR-32)
**Given** tôi lỡ một ngày
**When** streak tính lại
**Then** streak reset nhưng tổng số từ ghi nhớ tích lũy **không** reset

### Story 5.5: Thống kê và dự báo tải

As a người học,
I want xem thống kê và dự báo tải sắp tới,
So that tôi chủ động sắp xếp việc học.

**Acceptance Criteria:**

**Given** tôi mở Thống kê
**When** màn tải
**Then** hiện đã ôn hôm nay, phân bố mức chấm, heatmap, và **dự báo tải 7 & 30 ngày tới** (số thẻ due/ngày từ Due hiện có) (FR-33, UX-DR9)
**And** cấu hình lịch (desired-retention slider, daily limit) truy cập từ Settings (UX-DR10)

### Story 5.6: Trạng thái empty/loading/error toàn app

As a người học,
I want mọi màn xử lý gọn khi trống/đang tải/lỗi,
So that trải nghiệm mượt và không mất dữ liệu.

**Acceptance Criteria:**

**Given** bất kỳ màn nào (dashboard/library/review/stats)
**When** không có dữ liệu / đang tải / lỗi
**Then** hiện empty (có CTA, màn hết-thẻ ăn mừng) / skeleton / error được thiết kế (FR-34, UX-DR12, UX-DR13, UX-DR14)
**And** lỗi mạng không mất grade/nháp; 401 refresh ngầm (NFR)
