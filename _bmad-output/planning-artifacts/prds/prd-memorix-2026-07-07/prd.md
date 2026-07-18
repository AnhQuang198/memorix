---
title: Memorix — PRD (MVP)
status: final
created: 2026-07-07
updated: 2026-07-07
scope: MVP (E1-E6)
source: _bmad-output/planning-artifacts/memorix-spec/ (BMAD 14-phase), design/prototype/
---

# Memorix — PRD (MVP)

## 1. Tổng quan

Memorix là web app giúp người học **nghiêm túc** ghi nhớ từ vựng tiếng Anh bằng thuật toán lặp lại ngắt quãng **FSRS**. Định vị: *"não Anki, độ mượt Duolingo"*. Mục tiêu sản phẩm: **tối đa ghi nhớ, tối thiểu thời gian học**.

**Vấn đề**: người tự học ghi nhớ kém hiệu quả vì (1) ôn sai thời điểm, (2) học từ đã biết bỏ từ yếu, (3) không có tín hiệu tiến độ thật. Anki giải đúng bằng FSRS nhưng UX khắc nghiệt; Duolingo mượt nhưng nông và không cho tự thêm từ nghiêm túc.

**Cược cốt lõi cần validate ở MVP**: *người học nghiêm túc (đã bật khỏi Anki, chán Duolingo) có dùng — và về sau trả tiền — cho một công cụ FSRS UX tốt hơn không?* MVP này tồn tại để trả lời câu đó bằng **retention đo được**, không phải để kiếm tiền (billing hoãn sang V1.5).

**Ranh giới MVP**: web-first (responsive, PWA-ready về sau), 1 người dùng 1 tài khoản, tự thêm từ + ôn FSRS + thống kê cơ bản. Miễn phí trong beta.

## 2. Mục tiêu & Success Metrics

### Mục tiêu
- G1: Người học tự thêm từ và ôn theo lịch FSRS với ma sát tối thiểu.
- G2: Lịch ôn đúng (FSRS chuẩn) — không ôn thừa, không để quên.
- G3: Người dùng thấy **tiến độ trung thực**, tạo vòng thói quen quay lại hằng ngày.
- G4: Chứng minh giữ chân đủ tốt để biện minh xây V1 (curated + billing).

### North Star
**Số Từ Nhớ Được Mỗi Tuần** — số thẻ user recall đúng mà lịch ôn kế tiếp ≥ N ngày (mặc định N=7). Chỉ tăng khi sản phẩm thực sự dạy được, không gian lận bằng hoạt động ảo.

### Success Metrics (mục tiêu beta)
| Metric | Mục tiêu | [ASSUMPTION] |
|---|---|---|
| Activation (xong phiên ôn đầu) | ≥60% người đăng ký | ✓ giả định |
| D7 retention | ≥30% | ✓ |
| D30 retention | ≥15% | ✓ |
| % thẻ due được dọn/ngày (active user) | ≥80% | ✓ |
| Recall thật tại 30 ngày | ≥85% | dựa desired 0.9 |
| Thời gian thêm 1 từ | <10s | thiết kế |
| Chấm thẻ p95 | <150ms | NFR |

### Counter-metrics (chống tối ưu lệch)
- **Thời gian học/ngày tăng mà retention không tăng** → sản phẩm bắt cày, không dạy. Xấu.
- **Streak tăng nhưng recall thật giảm** → streak thành vanity. Xấu.
- **Số thẻ thêm tăng nhưng % ôn giảm** → tạo thẻ rồi bỏ. Xấu.

## 3. Người dùng & Hành trình

**Persona lõi — Linh**: 23 tuổi, đang luyện IELTS 7.0 trong 10 tuần để du học. Dùng điện thoại là chính, học vụn (trên xe buýt, lúc chờ, trước khi ngủ). Muốn mở app dọn thẻ đến hạn trong dưới 5 phút, không cấu hình. Coi trọng tiến độ trung thực và tốc độ hơn số lượng tính năng.

**Persona phụ (phục vụ ké, không tối ưu riêng ở MVP)**: Minh (định cư, bận, dài hạn); Anki-refugee (thích data, UX đẹp).

### UJ-1 — Phiên ôn hằng ngày (Linh)
Linh mở app trên xe buýt. Trang chủ hiện "24 thẻ đến hạn" + nút "Ôn ngay". Bấm → thẻ đầu hiện mặt trước (một từ). Cô nhớ lại, bấm "Lật thẻ", thấy nghĩa/ví dụ/IPA. Bấm một trong 4 mức (Again/Hard/Good/Easy) — dưới mỗi nút hiện khoảng cách ôn kế. Thẻ kế hiện ngay (không chờ). Hết queue → màn tổng kết "+12 từ nhớ được hôm nay". Cô đóng app sau 4 phút.

### UJ-2 — Thêm từ mới (Linh)
Đọc bài gặp từ "ubiquitous". Mở app, bấm "＋". Gõ term, lưu ngay (các trường khác để trống được). Từ vào hàng học mới, chờ tới lượt theo giới hạn ngày.

### UJ-3 — Quay lại sau kỳ nghỉ (Minh)
Nghỉ 5 ngày. Mở app, sợ queue nổ như Anki. Nhưng Memorix chỉ hiện ~20 thẻ ưu tiên (overdue nặng trước), phần còn lại rải sang các ngày sau. Không bị ngợp → không bỏ.

## 4. Phạm vi

### Trong MVP (E1-E6)
- Đăng ký / đăng nhập (email + OAuth), xác thực email, đặt lại mật khẩu.
- CRUD từ vựng (term + nghĩa/ví dụ/IPA/syn-ant/note optional), tạo thẻ.
- **Bộ thẻ khởi đầu seed sẵn** (một bộ nhỏ vài trăm từ IELTS) + enroll tối giản — chống cold-start để đo onboarding/retention beta thực tế.
- Lịch FSRS: chấm Again/Hard/Good/Easy → tính S/D/Due; lịch sử ôn.
- Queue đến hạn theo ưu tiên + giới hạn ngày + chống nổ queue; học thẻ mới.
- Thống kê cơ bản: đã ôn, đến hạn, streak, North Star; empty states.
- Web responsive (mobile-first), dark mode, i18n cơ bản (vi/en UI), a11y AA.
- Export dữ liệu + xóa tài khoản (GDPR).

### Ngoài MVP (đẩy sang V1+)
- Bộ thẻ curated + enroll; onboarding theo mục tiêu đầy đủ.
- Collection/Tag/Favorite/Search nâng cao, Import Anki/CSV.
- Thông báo/nhắc, heatmap/calendar/forecast trực quan đầy đủ.
- Sync đa thiết bị, AI card-fill, audio, PWA offline.
- Billing/Pro, optimizer FSRS/user, RN mobile, exam mode.

> Cắt tàn nhẫn có chủ đích: MVP validate *cược retention*, không phải phủ tính năng.

## 4b. Thuật ngữ (Glossary)

| Thuật ngữ | Nghĩa |
|---|---|
| **Entry (Từ)** | Đơn vị nội dung: term + nghĩa, ví dụ, IPA, đồng/trái nghĩa, ghi chú. Có thể của user hoặc curated (dùng chung). |
| **Card (Thẻ)** | Đơn vị **học** của một user cho một Entry theo một hướng. Một Entry → nhiều Card. Trạng thái FSRS (S/D/Due) gắn với Card, không gắn Entry. |
| **Grade (Mức chấm)** | Đánh giá độ nhớ khi ôn: Again / Hard / Good / Easy (1–4). |
| **Due (Đến hạn)** | Thời điểm nên ôn thẻ kế tiếp, do FSRS tính. |
| **Overdue (Quá hạn)** | Thẻ đã qua Due chưa được ôn. |
| **Desired retention** | Mục tiêu xác suất nhớ (mặc định 0.90) — quyết định độ dày lịch ôn. |
| **North Star** | Số Từ Nhớ Được Mỗi Tuần: recall đúng + interval kế ≥ N ngày (N=7). |

> **Entry vs Card** là phân biệt load-bearing xuyên toàn PRD: nội dung tách khỏi tiến trình học/user.

## 5. Tính năng & Yêu cầu Chức năng

FR đánh số toàn cục, ID ổn định. Nhóm theo epic.

### E1 · Xác thực & Tài khoản
- **FR-1** Người dùng đăng ký bằng email + mật khẩu, hoặc OAuth. [ASSUMPTION: cả Google + Apple ở MVP; Apple có thể lùi fast-follow nếu vướng thời điểm lên store — xem OQ-4.]
- **FR-2** Hệ thống gửi email xác thực; tài khoản chưa xác thực bị giới hạn quyền tới khi xác thực.
- **FR-3** Người dùng đăng nhập và duy trì phiên an toàn qua nhiều lần mở app.
- **FR-4** Người dùng đặt lại mật khẩu qua liên kết email 1 lần, hết hạn ngắn.
- **FR-5** Người dùng cập nhật hồ sơ (tên hiển thị, múi giờ, ngôn ngữ UI, theme).
- **FR-6** Người dùng xuất toàn bộ dữ liệu của mình và xóa tài khoản (GDPR).

### E2 · Từ vựng
- **FR-7** Người dùng tạo một từ với **chỉ term là bắt buộc**; nghĩa (nhiều), ví dụ, IPA, đồng/trái nghĩa, ghi chú đều optional. Lưu trong <10s.
- **FR-8** Khi tạo từ, hệ thống tự tạo thẻ học (mặc định 1 hướng; cho chọn 2 hướng).
- **FR-9** Người dùng xem, sửa, xóa (soft delete) từ của mình.
- **FR-10** Hệ thống cảnh báo khi tạo từ trùng (cùng term chuẩn hóa, cùng chủ sở hữu) và đề nghị mở từ cũ.
- **FR-11** Người dùng xem danh sách từ của mình với lọc theo trạng thái thẻ và sắp xếp cơ bản; danh sách lớn cuộn mượt (ảo hóa).

### E2b · Bộ thẻ khởi đầu (chống cold-start)
- **FR-11a** Hệ thống cung cấp một **bộ thẻ khởi đầu seed sẵn** (một bộ nhỏ, ưu tiên từ vựng IELTS) để người dùng mới có nội dung học ngay.
- **FR-11b** Người dùng enroll bộ khởi đầu; thẻ được tạo và **rải theo giới hạn thẻ mới/ngày** (không đổ hết), tôn trọng FR-27.
- **FR-11c** Trong onboarding, hệ thống gợi ý enroll bộ khởi đầu để đạt activation (xong phiên ôn đầu) mà không cần tự tạo thẻ trước.

> Phạm vi tối giản: một bộ curated read-only + luồng enroll. **Không** gồm duyệt nhiều deck, deck theo nhiều mục tiêu, hay marketplace (đó là V1).

### E3 · Lịch FSRS (lõi)
- **FR-12** Hệ thống dùng thuật toán FSRS để tính Stability/Difficulty/Due cho mỗi thẻ sau mỗi lần chấm.
- **FR-13** Người dùng chấm thẻ ở 4 mức: Again / Hard / Good / Easy.
- **FR-14** Hệ thống hiển thị **khoảng cách ôn kế** cho từng mức trước khi chấm.
- **FR-15** Việc chấm là **idempotent**: chấm lại cùng thẻ với cùng định danh phía client không tạo bản ghi trùng, không đổi lịch lần hai.
- **FR-16** Hệ thống ghi lịch sử mỗi lần ôn (append-only) đủ để tính lại trạng thái thẻ.
- **FR-17** Người dùng đặt mục tiêu ghi nhớ mong muốn (desired retention, 0.80–0.97; mặc định 0.90).
- **FR-18** Lịch tính theo **thời gian máy chủ** và "ngày học" theo múi giờ người dùng (không phụ thuộc đồng hồ thiết bị).

### E4 · Phiên ôn
- **FR-19** Người dùng bắt đầu phiên ôn và thấy các thẻ đến hạn lần lượt.
- **FR-20** Mặt trước hiện tối giản (term); người dùng lật để xem mặt sau đầy đủ.
- **FR-21** Sau khi chấm, thẻ kế hiện **ngay** (giao diện lạc quan, không chờ mạng); điểm được lưu và đồng bộ nền.
- **FR-22** Mất mạng khi ôn: điểm vẫn được ghi cục bộ và đồng bộ sau; **không bao giờ mất điểm đã chấm**.
- **FR-23** Người dùng chấm nhanh bằng bàn phím (mức 1–4, Space để lật) — cho tốc độ và khả năng tiếp cận.
- **FR-24** Khi hết thẻ, hiện màn tổng kết ăn mừng (số từ nhớ được, forecast ngày mai).

### E5 · Queue & Giới hạn ngày
- **FR-25** Hệ thống dựng queue thẻ đến hạn theo **ưu tiên**: overdue nặng (R thấp) → relearning → review đến hạn → thẻ mới.
- **FR-26** Người dùng đặt giới hạn thẻ mới/ngày và thẻ ôn/ngày (mặc định 20 / 200).
- **FR-27** Thẻ mới được **rải** theo giới hạn ngày, không đổ hết vào queue một lúc.
- **FR-28** **Chống nổ queue**: sau kỳ nghỉ dài, hệ thống giới hạn số thẻ hiển thị và rải overdue qua nhiều ngày thay vì dồn tất cả. Mặc định khởi điểm: hiển thị tối đa ~2× giới hạn review/ngày trong số overdue (ưu tiên R thấp nhất), rải phần dư qua tối đa ~7 ngày. [ASSUMPTION — tinh chỉnh bằng dữ liệu, xem OQ-2.]
- **FR-29** Người dùng học thẻ mới (luồng riêng), có hướng dẫn ngắn lần đầu về cách chấm.

### E6 · Thống kê & North Star
- **FR-30** Trang chủ hiện: số thẻ đến hạn + CTA "Ôn ngay", số thẻ mới hôm nay, streak.
- **FR-31** Hệ thống hiển thị **North Star** (số từ nhớ được tuần này) nổi bật.
- **FR-32** Streak gắn với **recall thật** (dọn thẻ due + nhớ được), không phải chỉ mở app; streak reset khi lỡ ngày nhưng chỉ số ghi nhớ tích lũy **không** reset.
- **FR-33** Người dùng xem thống kê cơ bản: đã ôn hôm nay, phân bố mức chấm, và **dự báo tải 7 và 30 ngày tới** (số thẻ due mỗi ngày, tính từ Due hiện có).
- **FR-34** Mọi màn có trạng thái empty/loading/error được thiết kế; màn "hết thẻ" ăn mừng thay vì để trống.

## 6. Yêu cầu Phi chức năng (cross-cutting)

### Hiệu năng
- NFR-1 Chấm thẻ (tính lịch + ghi log) p95 < 150ms.
- NFR-2 Dựng queue cho 10k thẻ p95 < 500ms.
- NFR-3 Không loading giữa các thẻ khi ôn (prefetch thẻ kế).

### Tin cậy & Dữ liệu
- NFR-4 Không mất bản ghi ôn (RPO < 5 phút).
- NFR-5 Chấm thẻ nguyên tử (cập nhật trạng thái + ghi log cùng một giao dịch).
- NFR-6 Trạng thái FSRS tính lại được từ lịch sử ôn (nguồn chân lý).

### Bảo mật
- NFR-7 Mật khẩu băm mạnh; phiên an toàn (access ngắn + refresh xoay vòng, phát hiện tái dùng).
- NFR-8 Kiểm quyền sở hữu trên mọi tài nguyên cá nhân; deny-by-default.
- NFR-9 Chống gian lận lịch: máy chủ tính điểm; client chỉ gửi mức chấm.
- NFR-10 Tuân OWASP Top 10; rate-limit theo tầng (đăng nhập chặt, ôn nới).

### Khả dụng / Tiếp cận / Đa nền
- NFR-11 Web responsive mobile-first; màn ôn nhất quán mọi kích thước.
- NFR-12 WCAG 2.1 AA: điều hướng bàn phím (phím 1–4), contrast AA cả 2 theme, screen reader đọc đúng term/nghĩa/IPA, tôn trọng prefers-reduced-motion.
- NFR-13 Dark/Light/System theme; i18n UI (vi/en), nội dung học tiếng Anh không dịch.

### Riêng tư & Vận hành
- NFR-14 GDPR: xuất toàn bộ dữ liệu + xóa tài khoản; không log PII/token.
- NFR-15 Có logging/metrics/trace đủ để chẩn đoán hot path.
- NFR-16 Uptime mục tiêu 99.9%.

## 7. Ràng buộc, Giả định, Phụ thuộc

**Giả định**
- [ASSUMPTION] Người học nghiêm túc sẽ dùng đều nếu UX đủ tốt (validate qua retention beta).
- [ASSUMPTION] Web-first chấp nhận được ở MVP (nhiều người mobile-only nhưng web responsive + PWA-ready đủ tạm).
- [ASSUMPTION] Tự thêm từ là hành vi đủ phổ biến để MVP không cần curated (dù cold-start là rủi ro — xem Open Questions).
- Mục tiêu số liệu ở Mục 2 là giả định beta, chưa có baseline.

**Ràng buộc**
- Team nhỏ/solo → phạm vi MVP phải cắt gọn; nợ kỹ thuật có chủ đích.
- Không tự viết FSRS — dùng thư viện chuẩn (chi tiết ở addendum).

**Phụ thuộc**
- Nhà cung cấp email (verify/reset), OAuth (Google/Apple), object storage (export/audio sau).
- Thư viện FSRS chính thức.

## 8. Open Questions

- ~~OQ-1 Cold-start~~ **ĐÃ CHỐT**: seed một bộ khởi đầu nhỏ vào MVP (FR-11a-c) để đo onboarding/retention thực tế.
- OQ-2 Ngưỡng "chống nổ queue" cụ thể (bao nhiêu thẻ/ngày sau nghỉ)? → tinh chỉnh bằng dữ liệu.
- OQ-3 N của North Star (interval ≥ N ngày) = 7? Cần chốt để đo nhất quán. [ASSUMPTION: N=7]
- OQ-4 OAuth cả Google + Apple ở MVP hay chỉ Google trước? [ASSUMPTION: cả hai]
- ~~OQ-5 Import CSV~~ **ĐÃ CHỐT**: import Anki/CSV để V1 (không vào MVP). Cold-start đã giải bằng seed thay vì import.

## 9. Milestones (MVP)

| Sprint | Nội dung | Done |
|---|---|---|
| S0 | Foundation: CI/CD, DB, auth skeleton, deploy staging | pipeline xanh, deploy được |
| S1 | E2 Từ vựng CRUD + tạo thẻ + **bộ khởi đầu seed + enroll** | thêm/sửa/xóa từ, auto-card, enroll bộ IELTS |
| S2 | E3 FSRS core + idempotent + replay test | chấm → lịch đúng, test replay xanh |
| S3 | E4 Phiên ôn (lật/chấm/phím/optimistic) | ôn end-to-end mượt |
| S4 | E5 Queue + giới hạn + chống nổ + learn | queue ưu tiên, không nổ sau nghỉ |
| S5 | E6 Stats + North Star + empty states | trang chủ + tổng kết + thống kê |
| → M1 | Beta kín | seed user thật, đo retention 30 ngày |

**Tiêu chí ra khỏi MVP**: đạt (hoặc học được vì sao trượt) mục tiêu D7/D30 + recall thật → quyết định build V1 (curated + billing).
