# Memorix — Đặc tả Sản phẩm (BMAD)

Web app học từ vựng tiếng Anh bằng thuật toán **FSRS**. Định vị: *"não Anki, độ mượt Duolingo, cho người học nghiêm túc"*. Mục tiêu: **tối đa ghi nhớ, tối thiểu thời gian học**.

> Tài liệu sinh từ phân tích BMAD 14 phase. UI tiếng Việt · nội dung học tiếng Anh.

## Mục lục
| # | Tài liệu | Nội dung |
|---|---|---|
| 01 | [Business Analysis](01-business-analysis.md) | Vision, market, SWOT, BMC, revenue, North Star, roadmap |
| 02 | [User Analysis](02-user-analysis.md) | Persona, pain, journey, onboarding, retention |
| 03 | [Domain Model](03-domain-model.md) | Bounded context, aggregate, event, state machine |
| 04 | [Functional Requirements](04-functional-requirements.md) | FR/NFR, user story, edge case, sync, offline |
| 05 | [FSRS Analysis](05-fsrs-analysis.md) | Thuật toán, queue, scheduling, stats |
| 06 | [UX Analysis](06-ux-analysis.md) | IA, màn hình, state, responsive |
| 07 | [System Architecture](07-system-architecture.md) | Modular monolith, diagrams, flows |
| 08 | [Database Design](08-database-design.md) | ERD, bảng, index, constraint |
| 09 | [API Design](09-api-design.md) | REST endpoints, auth, error, versioning |
| 10 | [Technology Stack](10-technology-stack.md) | Backend/FE/mobile/db/infra + tradeoff |
| 11 | [Security](11-security.md) | Auth, RBAC, OWASP, rate limit |
| 12 | [Development Workflow](12-development-workflow.md) | Git, CI/CD, testing, release |
| 13 | [Performance & Scalability](13-performance-scalability.md) | Cache, index, 6-stage scaling |
| 14 | [Delivery Plan](14-delivery-plan.md) | Epic, sprint, milestone, roadmap |
| 15 | [Final](15-final.md) | Executive summary, chốt architecture/stack/risk |

## Tài sản kèm
- Prototype 18 trang (app + auth): `../../design/prototype/index.html`
- Prompt thiết kế UI: `../ui-design-prompt.md`
- File design gốc (Design Composer): `../../design/Memorix.html`

## Chốt nhanh
- **North Star**: Số Từ Nhớ Được Mỗi Tuần (recall đúng, interval kế ≥ N ngày).
- **Moat**: nội dung curated + UX + data cá nhân + AI-fill + retention đo được — **không** phải FSRS (mã nguồn mở).
- **Kiến trúc**: Modular Monolith Go/Gin + Postgres, tách service chỉ khi 1M+ ép.
- **Stack**: Go+Gin · React/TS/Vite · Postgres/Redis/S3 · PWA→RN/Expo.
- **MVP**: E1-E6 (auth→vocab→FSRS→review→queue→stats), beta đo retention validate cược WTP.
