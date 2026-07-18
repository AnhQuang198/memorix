# Phase 5 — Phân tích FSRS

> **Quyết định lớn nhất**: KHÔNG tự viết lại FSRS. Dùng thư viện chính thức (`go-fsrs`), bọc sau interface domain. Tinh chỉnh ở tham số + queue policy + optimizer, không re-code toán.

## Overview
FSRS mô hình trí nhớ bằng 3 biến/thẻ:
| Biến | Nghĩa | Miền |
|---|---|---|
| Stability (S) | số ngày để R tụt 100%→90% | >0 |
| Difficulty (D) | độ khó nội tại | 1–10 |
| Retrievability (R) | xác suất nhớ lúc này | 0–1 |
Ý tưởng: ôn khi R rớt xuống **desired retention** (mặc định 0.9) → nhớ tối đa/lần ôn tối thiểu.

## Công thức lõi (FSRS-4.5/5/6)
```
DECAY  = -0.5
FACTOR = 0.9^(1/DECAY) - 1   # dùng hằng của lib

# Retrievability sau t ngày, stability S:
R(t, S) = (1 + FACTOR * t / S) ^ DECAY

# Khoảng cách để R = desired retention r_d:
I(r_d, S) = (S / FACTOR) * ( r_d ^ (1/DECAY) - 1 )
```

## Difficulty
```
D0(G) = w4 - exp(w5*(G-1)) + 1              # clamp [1,10]
ΔD    = -w6 * (G - 3)
D'    = D + ΔD * (10 - D)/9                   # linear damping
D'    = w7 * D0(4) + (1 - w7) * D'            # mean reversion
```

## Stability
```
# NHỚ (G∈{2,3,4}):
S' = S * (1 + exp(w8)*(11-D)*S^(-w9)*(exp(w10*(1-R))-1)*hardPenalty*easyBonus)
# QUÊN (G=1):
S' = w11 * D^(-w12) * ((S+1)^w13 - 1) * exp(w14*(1-R));  S' = min(S', S)
# Ôn trong ngày (short-term, FSRS-5): S' = S * exp(w17*(G-3+w18))
```
Điểm quan trọng: ôn khi R thấp cho stability gain lớn hơn (số hạng `e^(w10*(1-R))`) — lý do FSRS thắng SM-2.

## Grade — ý nghĩa
| Grade | Rating | Nghĩa | Hiệu ứng |
|---|---|---|---|
| Again | 1 | quên hẳn | lapse, S giảm, Relearning |
| Hard | 2 | nhớ chật vật | S tăng ít, phạt hard |
| Good | 3 | nhớ bình thường | S tăng chuẩn |
| Easy | 4 | nhớ dễ | S tăng mạnh, easy bonus |
**Giữ 4 lựa chọn** đúng ngữ nghĩa Anki (Anki-refugee quen tay).

## Scheduling Algorithm
```
onGrade(card, G, now):
    elapsed = now - card.lastReviewAt          # server time
    R       = R(elapsed, card.S)
    S'      = (G==Again) ? lapseStability(...) : recallStability(...)
    D'      = updateDifficulty(card.D, G)
    interval= I(user.desiredRetention, S')      # + fuzz ±5% chống dồn cụm
    interval= clamp(interval, minInterval, maxInterval)
    card.S, card.D, card.dueAt, card.lastReviewAt = S', D', now+interval, now
    append ReviewLog(G, R, S, S', D, D', now)    # nguyên tử
```

## Desired Retention
Mặc định 0.90 (0.80-0.97). Cao → ôn nhiều, nhớ chắc, tốn giờ. Thấp → ít ôn. Chế độ deadline thi: tạm nâng r_d. Đổi r_d chỉ áp lịch sau.

## Review Queue + Priority
```
priority = w_overdue*max(0,daysOverdue) + w_lowR*(1-R) + w_lapse*isRelearning + w_new*isNew
```
Thứ tự: Overdue nặng → Relearning → Review due → New (tới cap). Trộn xen new.

## Scheduling / Daily Limit / New Card
- "Ngày học" theo **TZ user**, grace qua nửa đêm (tới ~4h sáng).
- 2 cap: new/ngày (20), review/ngày (200). Vượt review → giữ overdue sang mai. Vượt new → entry chờ.
- Rải new: enroll 500 + cap 20/ngày → 25 ngày. Nếu review backlog lớn → tạm giảm new.

## Review History / Overdue / Bulk
- **ReviewLog append-only** = nguồn chân lý, replay tính lại state. Partition theo thời gian, index (cardId,ts).
- Overdue: FSRS tự xử lý (ôn muộn, R thấp, nhớ được → S tăng mạnh, thưởng). **Policy chống nổ** sau nghỉ dài.
- Bulk enroll/import → chỉ tạo Card New, rải theo new cap, job nền idempotent. Không chấm gộp.

## Statistics
Đã ôn/due còn lại; retention thật (30/90/180d); streak gắn recall; **North Star = từ nhớ được/tuần**; phân bố S/D; dự báo tải; true retention vs desired.

## Visualization
Heatmap (GitHub-style); Calendar forecast (thẻ due ngày tới từ dueAt); retention curve; forecast bar 7/30 ngày.

## Database (cho FSRS)
- `cards`: S, D, dueAt, lastReviewAt, status, reps, lapses. Index nóng **(ownerId, dueAt)**.
- `review_logs`: append-only, partition tháng, index (cardId,ts)/(ownerId,ts).
- `user_scheduler_prefs`: desiredRetention, dailyNew/Review, TZ, weights(optional/user).

## Performance
Build queue = index scan (ownerId,dueAt≤now)+limit <500ms. Grade = 1 update + 1 insert nguyên tử p95<150ms. Tính S/D là µs (I/O mới là bottleneck). Read model queue/stats tách write path. Cache Redis ở 100k user. Forecast/heatmap tính nền + cache.

## Future Extension
1. **Optimizer weights/user** (FSRS optimizer từ lịch sử log) → lịch cá nhân hóa, job nền, cần ~1000 review.
2. FSRS-6 (DECAY học được) — nhờ replay-from-log nâng cấp không mất data.
3. Đa loại thẻ (cloze, đảo chiều) — Direction là VO.
4. Load balancing lịch (fuzz) làm phẳng tải ngày.
5. Chế độ thi: r_d động theo ngày còn lại.

## Cơ hội ẩn
1. Optimizer/user = moat thật (càng dùng lâu càng khớp).
2. "True retention vs desired" hiển thị = trung thực + vòng tin cậy.
3. Forecast tải giảm bỏ cuộc.
4. Wrap lib sau interface → A/B thuật toán trên cùng log.

**Chốt**: dùng **thư viện FSRS chính thức** bọc port. Giá trị ở **queue priority + chống nổ + optimizer/user + replay-from-log**. Grade **server-authoritative** (client chỉ gửi grade). Hai bảng nóng: `cards(ownerId,dueAt)`, `review_logs` append-only partition.
