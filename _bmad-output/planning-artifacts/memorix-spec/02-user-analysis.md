# Phase 2 — Phân tích Người dùng

> **Thách thức**: "người học tiếng Anh" quá rộng. Người mới và luyện IELTS hành vi ngược nhau. Chọn **1 persona lõi** (luyện thi), phục vụ 2 phụ, bỏ phần còn lại ở MVP.

## Phân khúc
| # | Phân khúc | Đặc điểm | Ưu tiên |
|---|---|---|---|
| 1 | Luyện thi (IELTS/TOEFL/GRE) | deadline, WTP cao, đo được | **Lõi MVP** |
| 2 | Dân công sở / định cư | từ vựng công việc/đời, dài hạn | Phụ |
| 3 | Tự học đam mê (Anki refugee) | cày lâu, thích data | Phụ (loa) |
| 4 | Học sinh/sinh viên | ngân sách thấp | Sau |
| 5 | Giáo viên/gia sư | tạo deck lớp | V3 B2B |

## Personas
- **Linh (LÕI)** — 23t, VN, cần IELTS 7.0 trong 10 tuần. Mobile-heavy, học vụn. Goal: 800 từ academic. Pain: Quizlet không lên lịch, Anki dọa. Trả tiền: có (deadline gấp).
- **Minh** — 30t, định cư Canada, cần từ công việc + đời. Rất bận, 10 phút/ngày. Pain: nghỉ vài ngày → queue Anki nổ 300 → bỏ. Trả tiền: có nếu sync + ít ma sát.
- **Anki refugee** — tự học lâu, thích thống kê. Goal: cùng FSRS nhưng UX đẹp + curated + AI-fill. Loa mạnh.

## Mục tiêu / Pain
- Goals: nhớ đúng từ đúng lúc, thời gian tối thiểu, biết chắc tiến bộ, không quá tải sau nghỉ, tạo thẻ nhanh.
- Pains: quên (sai thời điểm), ôn thẻ đã thuộc, queue nổ sau nghỉ, không tín hiệu tiến độ, tạo thẻ chậm, Anki dốc/xấu, chán→bỏ.

## Hành vi / Động lực / Thói quen
- Học **vụn** (micro-session 2-10 phút), nhiều lần/ngày. Mobile ôn, desktop nhập hàng loạt. Ghét cấu hình. Chấm nhanh không nghĩ. Nghỉ ngày → cần cứu không phạt.
- Động lực: ngoại tại (thi/việc/visa), nội tại (tiến bộ thấy được, làm chủ), xã hội (streak, chia sẻ), kiểm soát (data, biểu đồ).
- Thói quen: neo routine sẵn (sáng/tối/commute). Nhắc đúng liều. Streak-ảo phản tác dụng với Anki refugee.

## Kịch bản hằng ngày
1. Sáng bus — "12 thẻ đến hạn", ôn 4 phút, đóng.
2. Trưa — đọc bài gặp từ mới, thêm nhanh (V2: highlight→thẻ).
3. Tối — dọn queue, xem heatmap + streak + % nhớ.
4. Sau nghỉ 5 ngày — app đã dàn overdue mượt, chỉ 20 thẻ ưu tiên → không bỏ.

## Customer / User Journey
- Customer: Nhận biết (TikTok creator, SEO) → Cân nhắc (landing "nhớ đo được", so Anki xấu) → Chuyển đổi (free, chọn deck IELTS, xong ôn đầu) → Giữ chân (ôn hằng ngày, nghỉ không nổ queue) → Kiếm tiền (đụng cap/cần sync → Pro trước thi) → Ủng hộ (đậu, giới thiệu).
- User flow lõi: Mở → có thẻ due? → phiên ôn → hiện front → lật+đánh giá → chấm 1-4 → FSRS xếp lịch → còn thẻ? → tổng kết (streak, %nhớ, heatmap).

## Onboarding (activation = xong ôn đầu <3 phút, zero-config)
1. Đăng ký email/OAuth → 2. Hỏi mục tiêu (Thi/Công việc/Chung) → 3. Chọn deck curated gợi ý → 4. Học nhanh 5 thẻ (dạy chấm bằng làm) → 5. Đặt nhắc optional → 6. Activation ✓ vào dashboard.
- Không bắt tạo thẻ trước (ma sát cao). Curated trước, tự thêm sau.

## Chiến lược giữ chân
| Đòn bẩy | Cách |
|---|---|
| Thói quen | nhắc đúng giờ neo, đúng liều |
| Cứu người nghỉ | dàn overdue mượt, không phạt queue nổ |
| Tiến độ trung thực | heatmap + %nhớ + North Star |
| Streak-ghi-nhớ | gắn recall thật, không chỉ mở app |
| Aha sớm | "bạn sắp quên từ này — ôn cứu nó" |
| Email tiến độ | tuần: "nhớ +45 từ" |
| Gấp deadline | chế độ đếm ngược thi |
| Winback | nghỉ >7 ngày → "queue đã dọn sẵn, 5 phút" |

## Cơ hội ẩn
1. Onboarding theo mục tiêu → cá nhân hóa deck+lịch, tăng activation.
2. "Cứu từ sắp quên" — biến FSRS trừu tượng thành hành động hữu hình.
3. **Chống-nổ-queue** = khác biệt lớn với Anki (đau #1 khiến bỏ). Đặt tên riêng, quảng bá.
4. Micro-session <5 phút làm mặc định thiết kế.

**Chốt**: thiết kế cho **Linh (luyện thi)** lõi — mobile, vụn, gấp, WTP cao. Vũ khí giữ chân lớn nhất = chống nổ queue + tiến độ trung thực.
