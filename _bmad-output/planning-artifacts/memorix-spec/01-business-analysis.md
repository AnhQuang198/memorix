# Phase 1 — Phân tích Kinh doanh

> **Thách thức**: ý tưởng như mô tả = hàng phổ thông. Anki + Quizlet + Memrise + Duolingo đã chiếm. FSRS không phải lợi thế (mã nguồn mở, Anki tích hợp từ 2023). Phase này tìm **wedge thật**.

## Tầm nhìn
Người học đạt tiếng Anh bền vững với nửa thời gian so flashcard cũ. Trí nhớ thành thứ dự đoán được, không hên xui.

## Sứ mệnh
Lên lịch mỗi lần ôn đúng khoảnh khắc ngay trước khi quên. Biến cày từ vựng thành thói quen đo được, tối thiểu, tích lũy.

## Giá trị cốt lõi
| Giá trị | Nghĩa |
|---|---|
| Ghi nhớ hơn số lượng | Nhớ ít từ > thấy nhiều từ |
| Dựa bằng chứng | Lịch ôn dựa khoa học trí nhớ (FSRS) |
| Ít ma sát | Thêm từ <10s, ôn <3s/thẻ |
| Người học sở hữu data | Export đầy đủ, không khóa chân |
| Tiến độ trung thực | % nhớ thật, không streak ảo |

## Mục tiêu sản phẩm
1. Giảm thời-gian-tới-ghi-nhớ vs Anki ≥30% (cảm nhận + đo).
2. Vừa tự thêm từ vừa dùng bộ curated — ít ma sát.
3. Vòng thói quen: hoàn thành ôn hằng ngày ≥ mục tiêu ghi nhớ.
4. Trí nhớ dài hạn: từ sống >6 tháng, recall ≥90%.

## Phát biểu vấn đề
Người học ghi nhớ kém hiệu quả: **sai thời điểm** (ôn quá sớm/muộn), **sai nội dung** (học từ đã biết), **không phản hồi** (không biết method có hiệu quả tới khi thi/nói vấp). Anki giải đúng nhưng UX khắc nghiệt; Quizlet chỉ lưu trữ; Duolingo cho người mới.

## Cơ hội / Wedge
**"Sức mạnh Anki, độ mượt Duolingo, cho người học tiếng Anh."** Wedge: người học Anh trung–cao cấp (IELTS/TOEFL/công việc/định cư) đã vượt Duolingo, bật khỏi Anki. Nhóm lớn, ít ai phục vụ, sẵn trả tiền.

## Giải pháp hiện có
| Tool | Thuật toán | Curated | UX tự thêm | Độ bóng | Giá |
|---|---|---|---|---|---|
| Anki | FSRS (tốt nhất) | Không (lộn xộn) | Mạnh, xấu | Kém | Free / $25 iOS |
| Quizlet | Leitner cơ bản | Có (UGC khủng) | Dễ | Tốt | ~$36/năm |
| Memrise | SRS riêng | Có | Vừa | Tốt | ~$60/năm |
| Vocabulary.com | Thích ứng | Có | Yếu | Tốt | ~$30/năm |
| Duolingo | Riêng | Có (lộ trình) | Không | Tốt nhất | ~$84/năm |
| Readlang/LingQ | SRS + đọc | Theo bài | Tự động | Vừa | ~$70-120/năm |

## SWOT
| | Có lợi | Có hại |
|---|---|---|
| **Nội tại** | FSRS đã chứng minh, ngách tập trung, data model sạch, export trung thực | thương hiệu mới/solo, chưa có thư viện nội dung ngày 1, không hiệu ứng mạng, FSRS không độc quyền |
| **Bên ngoài** | khe UX Anki, WTP luyện thi, AI gen nội dung rẻ, mobile sau | Anki cải thiện UX, Duolingo thêm SRS, app LLM auto-thẻ, chi phí chuyển đổi thấp |

## Value Proposition Canvas
- **Jobs**: nhớ từ cho thi/công việc/đời; không phí thời gian; biết mình tiến bộ.
- **Pains**: quên, chán, cấu hình phức tạp, không tín hiệu tiến độ, quá tải sau nghỉ.
- **Gains**: đậu thi, nói tự tin, thời gian tối thiểu, cảm giác kiểm soát.
- **Pain relievers**: thời điểm tối ưu, queue thông minh, chấm 1-chạm, % nhớ trung thực, zero-config.
- **Gain creators**: deck curated luyện thi, AI điền thẻ, streak-gắn-ghi-nhớ, sync đa thiết bị.

## Business Model Canvas
| Khối | Nội dung |
|---|---|
| Phân khúc | Tự học nghiêm túc; luyện thi; định cư/công sở; sau: giáo viên/trường |
| Value prop | Ghi nhớ tối đa, thời gian tối thiểu |
| Kênh | SEO, app store, TikTok/YouTube creator, cộng đồng luyện thi |
| Quan hệ | Tự phục vụ, thông báo thói quen, email tiến độ |
| Doanh thu | Freemium subscription; deck cao cấp; sau B2B/edu |
| Nguồn lực | Impl FSRS, thư viện curated, sync infra, thương hiệu |
| Đối tác | Cấp phép nội dung, Stripe, influencer luyện thi |
| Chi phí | Hạ tầng, nội dung, audio/TTS, dev, CAC |

## Revenue Model & Pricing
- Freemium subscription (chính), gói nội dung cao cấp (phụ), B2B/EDU (tương lai). **Tránh quảng cáo**.
| Bậc | Giá neo | Cổng |
|---|---|---|
| Free | $0 | FSRS lõi, thẻ tự tạo, 1 thiết bị, stats cơ bản, cap curated |
| Pro | ~$5-7/th, ~$40-50/năm | Không giới hạn, sync, curated đầy đủ, stats/heatmap, AI-fill |
| Edu/Team | theo ghế | Quản lớp, deck chung, dashboard |

## Success Metrics & North Star
- Acquisition (signup, activation), Engagement (DAU/WAU), **Retention (D1/D7/D30, % due cleared)**, Learning outcome (recall 30/90/180d), Monetization (Free→Pro, MRR, churn, LTV/CAC).
- **North Star = Số Từ Nhớ Được Mỗi Tuần**: recall đúng, interval kế ≥ N ngày. Chống vanity (streak/cards-reviewed gian lận được).

## Risks & Assumptions
- Rủi ro: không moat (FSRS mở) → moat = nội dung+UX+data; cold-start nội dung; chi phí chuyển đổi thấp; giữ thói quen khó; phình phạm vi.
- Giả định (chưa kiểm chứng): người nghiêm túc trả tiền cho UX tốt hơn Anki free (**cược lớn nhất**); lợi ích FSRS cảm nhận được; nội dung curated mua được hợp pháp; web-first chấp nhận được; tự thêm từ phổ biến.

## Roadmap chiến lược
MVP (vocab CRUD, FSRS core, review, stats, web) → V1 (curated, org, heatmap, import, notif) → V1.5 (sync, AI-fill, audio, Pro) → V2 (mobile, exam decks, reading capture) → V3 (B2B/edu, social).

## Cơ hội ẩn
1. Retention proof-of-outcome (marketing "nhớ đo được").
2. AI card generation (diệt ma sát tạo thẻ).
3. Reading capture (highlight→thẻ).
4. Personal history = moat.
5. Exam-deadline mode (WTP cao).
6. Anki refugee funnel (CAC rẻ nhất).

**Chốt**: tái định vị từ "app SRS chung chung" → **"tool từ vựng tiếng Anh trung-thực-về-ghi-nhớ, AI hỗ trợ, cho người học nghiêm túc"**. Moat = nội dung + UX + data cá nhân, không phải FSRS.
