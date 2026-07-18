# Phase 4 — Yêu cầu Chức năng

> **Thách thức**: 40+ mục không cùng ưu tiên → gắn MoSCoW. Offline/sync full = bẫy phạm vi lớn nhất; MVP chỉ server-truth.

## Functional Requirements (MoSCoW)
| ID | FR | Ưu tiên |
|---|---|---|
| FR-1 | Auth (email + OAuth) | Must |
| FR-2 | CRUD Entry (term, POS, nhiều nghĩa, ví dụ, IPA, syn/ant, note) | Must |
| FR-3 | Tạo Card từ Entry (1/2 chiều) | Must |
| FR-4 | Phiên ôn FSRS: hiện, lật, chấm Again/Hard/Good/Easy | Must |
| FR-5 | Queue đến hạn, sắp ưu tiên, daily limit | Must |
| FR-6 | Học thẻ mới (new card strategy, cap/ngày) | Must |
| FR-7 | Stats cơ bản: đã ôn, đến hạn, streak | Must |
| FR-8 | Collection + Tag + Favorite | Should |
| FR-9 | Search / Filter / Sort | Should |
| FR-10 | Deck curated + enroll | Should |
| FR-11 | Import (Anki/CSV) / Export (CSV/JSON) | Should |
| FR-12 | Heatmap + Calendar | Should |
| FR-13 | Thông báo/nhắc | Should |
| FR-14 | Sync đa thiết bị | Should (V1.5) |
| FR-15 | AI điền thẻ | Could |
| FR-16 | Audio phát âm | Could |
| FR-17 | Chế độ deadline thi | Could |
| FR-18 | Backup/Restore | Could |
| FR-21 | Dark mode | Should |
| FR-22 | Localization (i18n UI) | Should |
| FR-23 | Offline ôn | Could (V1.5) |
| FR-19/20 | Reading capture / Social | Won't (V2+) |

## Non-functional
| NFR | Mục tiêu |
|---|---|
| Chấm thẻ → lịch kế | p95 <150ms |
| Build queue 10k thẻ | <500ms |
| Tải | 100k user, 10k đồng thời, scale ngang |
| Uptime | 99.9% |
| Bền dữ liệu | RPO <5 phút (không mất review log) |
| Bảo mật | OWASP top 10 |
| A11y | WCAG 2.1 AA |
| i18n | UI đa ngôn ngữ, RTL-ready |
| Riêng tư | GDPR xóa/xuất |

## User Stories (lõi) + Acceptance
- **US-1 Ôn thẻ due**: queue đúng due≤now sắp ưu tiên; lật hiện mặt sau đầy đủ; chấm → biến khỏi queue + FSRS tính Due; queue rỗng → tổng kết; chấm p95<150ms không mất grade.
- **US-2 Thêm từ nhanh**: lưu <10s chỉ với term; auto-tạo Card (tôn trọng cap); trùng term → cảnh báo mở entry cũ.
- **US-3 Enroll deck**: bulk Card nhưng **không đổ hết** queue, rải theo daily new; thấy tiến độ.
- **US-4 Nghỉ không nổ queue**: sau 5 ngày → giới hạn ưu tiên (overdue nặng trước), rải qua vài ngày.
- **US-5 Tiến độ thật**: heatmap, streak, %recall 30/90d; streak gắn recall đúng.
- **US-6 Import Anki**: upload .apkg/CSV → map → preview → nhập; giữ lịch sử nếu có; báo cáo X nhập/Y bỏ.

## Business Rules
- 1 Entry duy nhất theo (term normalized, ownerId). Card 1 owner; curated read-only. FSRS state cập nhật nguyên tử cùng ReviewLog. Daily new/review limit riêng (mặc định 20/200). Enroll không vượt daily new (rải). Xóa Entry = soft delete. Desired retention mặc định 0.9 (0.8-0.97). Streak reset khi lỡ ngày; retention agg không reset. Again = Lapse.

## Validation
email RFC unique · password ≥8 · term 1-200 trim · meaning/example ≤2000 escape HTML · IPA ≤100 · audioUrl https whitelist · tag 1-50 unique/owner · grade ∈{1,2,3,4} · desiredRetention 0.8-0.97 · dailyLimit 1-9999 · import mime+size hợp lệ.

## Edge Cases
Chấm 2 lần (double-tap/2 thiết bị) → **idempotency (cardId, clientReviewId)**, lần 2 no-op. Clock lệch → Due theo **server time**. DST → "ngày học" theo TZ user. Enroll 10k → job nền. Term khác dấu/hoa → normalize. Entry xóa khi đang session → skip an toàn. Đổi desiredRetention giữa chừng → chỉ áp thẻ sau. Queue overdue lớn sau nghỉ → chống nổ.

## Error Handling
Mã lỗi chuẩn (Phase 9) + message thân thiện + hành động. Chấm lỗi mạng → **optimistic UI + retry queue**, không mất grade. Import lỗi 1 dòng → không fail cả file. 401 → refresh; fail → logout mượt giữ nháp. 5xx → banner + backoff.

## Offline (Could, V1.5)
Ôn offline: tải queue + nội dung trước; chấm ghi local queue. Thêm/sửa offline → lưu local dirty. Online → sync.

## Synchronization
- MVP: **server là chân lý**. Thiết bị pull queue + push review (idempotent). Không offline write.
- V1.5: sync 2 chiều đơn giản — last-write-wins theo trường cho Entry metadata; ReviewLog append-only (union, không xung đột).
- Xung đột FSRS state: giải bằng **replay ReviewLog theo server-ts** → hết xung đột.
- V2+: CRDT nếu offline-heavy (chưa cần sớm).

## Notification / Reminder
Nhắc hằng ngày giờ user (theo TZ); winback khi nghỉ >N ngày; email tiến độ tuần. Kênh web push + email (mobile push V2). Tôn trọng opt-out + quiet hours, dedupe theo ngày. Chỉ nhắc khi có thẻ due chưa ôn; user dọn queue → hủy nhắc.

## Review Queue (ưu tiên)
1. Overdue nặng (R thấp) 2. Relearning (lapse) 3. Review due hôm nay 4. New (tới cap). Trộn new xen review để đỡ mệt. Cap tổng, dư rải ngày sau.

## Search/Filter/Sort · Org · Import/Export
- Search: term/nghĩa/note/tag (MVP ILIKE/FTS, V1 search engine). Filter: tag/collection/status/due/difficulty/favorite. Sort: alphabet/ngày/due/difficulty/stability/lapse.
- Collection (M:N phẳng), Folder (cây, Could), Tag (M:N), Favorite (cờ).
- Import Anki .apkg/CSV/JSON (map+preview). Export CSV/JSON (gồm FSRS state+log). Backup snapshot JSON (Pro). Restore merge/replace. GDPR: tải toàn bộ + xóa account.

## Dark Mode / i18n / A11y
- Theme light/dark/system, lưu server+local, tôn trọng prefers-color-scheme.
- i18n ICU, ngày/số theo locale, RTL-ready. Nội dung học (Anh) không dịch; UI dịch (Anh, Việt trước).
- WCAG 2.1 AA: phím tắt chấm **1-4** khi ôn, ARIA, contrast AA 2 theme, screen reader đọc term/IPA, không dựa chỉ màu, focus visible, prefers-reduced-motion.

## Cơ hội ẩn
1. Idempotency key trên grade = nền sync+offline. Làm ngay MVP.
2. Rebuild FSRS từ ReviewLog = giải xung đột **và** đổi thuật toán retroactive.
3. Phím 1-4 = a11y + tốc độ power-user.
4. "Data khả chuyển" = marketing tin cậy.

**Chốt**: Must = auth + entry/card CRUD + review FSRS + queue + stats. Cắt offline/sync khỏi MVP (server-truth). Hai quyết định sớm: **idempotent grade** + **ReviewLog append-only replay**.
