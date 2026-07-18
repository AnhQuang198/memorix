# Phase 6 — Phân tích UX

> **Thách thức**: cho Linh mobile ôn vụn, màn quan trọng nhất = **Review** + **Dashboard**. MVP = 4 điểm đến. Review mở nhanh nhất (0 → chấm thẻ đầu trong 2 chạm).

## Information Architecture
Dashboard/Home · Học&Ôn (Review, Learn) · Thư viện (List, Detail, Collections, Curated, Search) · Thống kê (Heatmap, Calendar) · Cài đặt/Hồ sơ (Profile, Notif, Scheduler, Import/Export).

## Navigation
- Mobile: bottom tab 4 mục — **Home · Ôn · Thư viện · Thống kê**. Cài đặt trong avatar. FAB "＋ thêm từ". Nút "Ôn ngay" badge số due.
- Desktop: sidebar trái + top bar (search, avatar, nút Ôn nổi bật), master-detail.
- ≤4 tab (giới hạn nhận thức + mobile-first).

## Screen List (MoSCoW)
Must: Dashboard, Review, Learn, Vocabulary List, Vocabulary Detail, Add/Edit Entry, Settings. Should: Statistics, Collections, Curated Decks, Search, Profile/Billing, Import/Export. Could: Notifications center.

## Màn chính
- **Dashboard**: chào + streak; thẻ due to + "Ôn ngay"; new-today; North Star "+45 từ nhớ được"; mini heatmap; forecast "mai ~30"; FAB.
- **Review (thiêng)**: Front = term to + "Lật thẻ" + progress + pause. Back = term+IPA+audio, POS, nghĩa, ví dụ, syn/ant. **4 nút chấm có interval dưới mỗi nút** (`<1m/10m/4d/9d`). Phím 1-4 + Space. Optimistic: chấm → thẻ kế ngay.
- **Learn**: như Review + mini-onboarding lần đầu (giải thích 4 nút).
- **Vocabulary List**: search + filter chip + sort; virtualize (10k); badge trạng thái FSRS; bulk select.
- **Vocabulary Detail**: toàn bộ Entry + card stats (S,D,due,lapse,lịch sử) + actions (sửa/favorite/suspend/reset/xóa/AI-fill).
- **Add/Edit**: term bắt buộc, còn lại optional (<10s); AI-fill (Pro) gợi ý; chọn chiều/tag/collection.
- **Statistics**: heatmap, streak, retention 30/90/180d, North Star, forecast, phân bố S/D, true-vs-desired.
- **Collections / Curated / Search / Notifications / Settings / Profile**: xem spec.

## Responsive
| Breakpoint | Layout |
|---|---|
| Mobile <640 | bottom tab, 1 cột, FAB, nút chấm thumb-reach |
| Tablet 640-1024 | 2 cột master-detail |
| Desktop >1024 | sidebar + top bar, phím tắt |
Review giống nhau mọi kích thước (muscle memory). Touch ≥44px.

## States
- **Empty**: Dashboard "chưa có thẻ due 🎉"; Review-done "Xong! +12 từ nhớ được" + forecast; Library "chưa có từ" + CTA; Search "không tìm thấy — bỏ bớt filter".
- **Loading**: skeleton (không spinner trắng); Review **prefetch thẻ kế** → 0 loading giữa thẻ; optimistic grade/add/favorite.
- **Error**: mất mạng khi ôn = banner "offline — grade đã lưu, sẽ sync" (không chặn); load fail → retry + cache; save fail → giữ nháp; 401 → refresh ngầm; **không bao giờ mất grade/nháp**.

## Cơ hội ẩn
1. Hiện interval kế trên nút chấm → dạy trực giác spaced-repetition + tin thuật toán.
2. Empty Review = ăn mừng North Star, không màn trống.
3. Prefetch thẻ kế → nhanh hơn mọi đối thủ web.
4. FAB thêm từ mọi nơi + AI-fill → diệt ma sát #1.
5. Forecast "mai ~30 thẻ" → đặt kỳ vọng, giảm bỏ cuộc.

**Chốt**: 4 tab. Review thiêng — nhanh nhất, prefetch, optimistic, phím 1-4, hiện interval kế. Mọi state có empty/loading/error; không bao giờ mất grade/nháp. Mobile-first.

> Prototype thật (18 trang app + auth): `../../design/prototype/index.html`.
