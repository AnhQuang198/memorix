# Final — Tổng hợp

## 1. Executive Summary
Memorix = web app học từ vựng tiếng Anh bằng **FSRS**, định vị **"não Anki, độ mượt Duolingo, cho người học nghiêm túc"**. Vấn đề: cày từ sai thời điểm, sai nội dung, không phản hồi. Anki giải đúng nhưng UX khắc nghiệt; Duolingo mượt nhưng nông. Wedge: **luyện thi (IELTS/TOEFL) + định cư/công sở** đã vượt Duolingo, bật khỏi Anki.

**Sự thật cốt lõi**: FSRS không phải moat (mã nguồn mở). Moat thật = **nội dung curated + UX + data cá nhân + AI-fill + retention đo được**. Cược lớn nhất chưa chứng minh: *người nghiêm túc trả tiền cho UX tốt hơn Anki free?* → validate ở beta M1.

## 2. Product Specification (tóm)
- North Star: Số Từ Nhớ Được Mỗi Tuần (recall đúng, interval kế ≥ N ngày).
- Persona lõi: Linh (IELTS, mobile, vụn, WTP cao).
- Domain: 2 core context Scheduling+Review quanh Card aggregate; Entry/Collection tách ref-id; Progress/Notification downstream. Entry (nội dung) vs Card (đơn vị học/user + FSRS) = phân biệt sống còn.
- Chức năng Must: auth, vocab CRUD, FSRS core, review (Again/Hard/Good/Easy + phím 1-4), queue + daily limit + chống nổ, stats + North Star.
- UX: 4 tab, Review là màn thiêng (2 chạm, prefetch 0-loading, optimistic, interval dưới nút). Prototype 18 trang tại `design/prototype/`.
- FSRS: thư viện chính thức bọc port; grade server-authoritative; replay-from-log.

## 3. Recommended Architecture
Modular Monolith (Go), Clean trong core, event bus in-process, CQRS nhẹ, 1 Postgres schema-per-module + Redis. Bounded-context = đường cắt service khi 1M+. Hot path grade nguyên tử + idempotent, p95<150ms. ReviewLog append-only = nguồn chân lý (replay giải sync + đổi thuật toán). Worker tách. `entries` tách `cards`; `review_logs` partition tháng; index nóng `cards(owner_id, due_at)`.

## 4. Recommended Technology Stack
| Tầng | Chốt |
|---|---|
| Backend | Go + **Gin** + sqlc/pgx + River + slog + OTel |
| FSRS | go-fsrs (bọc port) |
| Frontend | React + TS + Vite + TanStack + shadcn/Tailwind + Zod |
| Mobile | PWA → **RN/Expo** (không Flutter — tái-dùng TS) |
| DB | Postgres + Redis + S3 + (FTS→OpenSearch) |
| Infra | Docker · Compose→Swarm→K8s · Caddy · Cloudflare · Prometheus/Grafana/Loki · Vault |
| AI | Claude API (card-fill, quota+sanitize) |

## 5. Development Roadmap
M0 Foundation → M1 MVP (E1-E6, beta đo retention) → M2 V1 (curated+org+import+notif) → M3 V1.5 (billing+AI-fill+sync+PWA) → M4 V2 (optimizer+RN+exam). Đường găng E1→E5. Ước thô solo: M1 ~10-12 tuần, M2/M3 ~8 mỗi cái.

## 6. Risk Assessment
| Rủi ro | Mức | Giảm |
|---|---|---|
| Không moat (FSRS mở) | Cao | moat = nội dung+UX+data+AI |
| Cược WTP sai | Cao | validate beta M1 trước billing |
| Cold-start nội dung | Cao | seed curated + AI-fill sớm |
| Chi phí chuyển đổi thấp | Vừa | import Anki 1-chạm + data khóa dần |
| Ôm hết spec vào MVP | Cao | cắt E1-E6, feature flag |
| FSRS impl sai | Cao | lib + test replay + so Anki |
| Solo bandwidth | Cao | ưu tiên đường găng |

## 7. Future Expansion
Retention đo được (marketing) · AI card-fill · reading capture · FSRS optimizer/user (moat tăng theo thời gian) · exam deadline mode · B2B/Edu · Anki refugee funnel.

## "Nếu tôi là Chief Architect của Memorix"

**Sản phẩm**: không bán "app SRS" (bão hòa, FSRS free) — bán **trung thực về trí nhớ + ít ma sát nhất** cho persona đau thật: người luyện thi bị Anki dọa, bị Duolingo làm chán. Mọi quyết định phục vụ 1 câu hỏi validate: họ có trả tiền cho UX tốt hơn Anki? Nên MVP nhỏ, beta đo retention **trước** khi xây doanh thu.

**Kỹ thuật**: tối ưu cho **đúng trước, rẻ để sai, dễ để lớn**:
- Modular Monolith Go/Gin + 1 Postgres — 100k user thừa sức; microservices sớm giết team nhỏ. Nhưng vẽ ranh giới bounded-context ngay → 1M+ cắt service là bê schema ra.
- Ba quyết định trả cổ tức kép: (1) grade idempotent + server-authoritative = sync-safe + chống gian lận; (2) ReviewLog append-only replay = giải xung đột sync + đổi thuật toán retroactive không mất data; (3) Entry tách Card = curated dùng chung, AI-fill 1 lần, moat chi phí.
- Dùng thư viện FSRS, không tự viết — giá trị ở queue-policy, chống-nổ, optimizer, replay.
- PWA trước RN, React/TS xuyên stack để mobile tái dùng, nên không Flutter.

**Triết lý**: moat không phải thuật toán ai cũng có — mà là **nội dung + trải nghiệm + tháng ngày data cá nhân** không sao chép được. Xây thứ càng dùng lâu càng khó rời, và trung thực đến mức người dùng **tin** nó thực sự dạy họ nhớ.
