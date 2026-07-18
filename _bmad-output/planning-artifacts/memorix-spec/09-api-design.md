# Phase 9 — Thiết kế API

> **Thách thức**: đừng phơi FSRS state cho client sửa. Client chỉ gửi **grade + clientReviewId**; server tính S/D/Due. API kể chuyện *hành động học*, không phải *CRUD bảng*.

## Quy ước chung
- **Base** `/api/v1` — version qua URL path. Breaking → `/v2` song song + sunset header.
- **Auth**: `Authorization: Bearer <access JWT>` (15m). Refresh qua cookie httpOnly. Public: `/auth/*`, `/health`.
- **Authz**: personal resource check `owner_id == principal`. Curated read-all, ghi = curator/admin.
- **Idempotency**: mutation nhạy cảm nhận `Idempotency-Key`; grade dùng `clientReviewId` trong body.
- **Pagination**: cursor-based `?limit=50&cursor=...` → `{data, page:{next_cursor, has_more, limit}}`.
- **Filter/Sort**: `?filter[status]=review&sort=-due_at,term` (whitelist field).
- **Lỗi**: `{error:{code, message, fields, trace_id}}`.

## Mã lỗi
| HTTP | code |
|---|---|
| 400 | VALIDATION_ERROR |
| 401 | UNAUTHENTICATED |
| 403 | FORBIDDEN |
| 404 | NOT_FOUND |
| 409 | CONFLICT |
| 422 | UNPROCESSABLE |
| 429 | RATE_LIMITED |
| 500 | INTERNAL |

## 1. Authentication
`POST /auth/register` · `/auth/login` · `/auth/oauth/{provider}` · `/auth/refresh`(cookie) · `/auth/logout` · `/auth/verify-email` · `/auth/resend-verification` · `/auth/forgot-password` · `/auth/reset-password`.
```
POST /auth/login → { access_token, expires_in:900, user } + Set-Cookie refresh(httpOnly,Secure,SameSite=Strict)
POST /auth/register → 201; email unique(409); password zxcvbn≥2
```

## 2. Users / Profile
`GET/PATCH /me` · `PATCH /me/scheduler-prefs` (desired_retention 0.8-0.97, daily limits, exam_deadline) · `GET /me/export`(Pro, GDPR) · `DELETE /me`.

## 3. Vocabulary
`GET/POST /entries` · `GET/PATCH/DELETE /entries/{id}` · `POST /entries/{id}/ai-fill`(Pro) · `POST /entries/import`.
```
GET /entries?filter[status]=review&filter[tag]=ielts&sort=-due_at&limit=50&cursor=...
POST /entries { term, part_of_speech, meanings[], examples[], pronunciations[], direction, tags[], collection_ids[] }
  val: term 1-200; trùng(owner,term_norm)→409; res 201 {entry, card}
POST /entries/{id}/ai-fill → { suggestions:{ipa, meanings[], examples[], synonyms[], antonyms[]} } (403 nếu không Pro)
```

## 4. Review (hot path)
`GET /review/queue` · `POST /review/grade` · `GET /review/forecast` · `GET /learn/new`.
```
GET /review/queue?limit=50 → { data:[{card_id, entry:{term,ipa,meaning,example,syn,ant}, due_at, retrievability, next_intervals:{again,hard,good,easy}}], counts:{due,overdue,new_available} }
POST /review/grade { card_id, grade(1-4), client_review_id, duration_ms } → { card_id, new_status, next_due_at, interval_days }
  idempotency: (card_id, client_review_id) trùng → trả kết quả cũ, không chấm lại. p95<150ms
```

## 5. Collections / Tags / Decks
`GET/POST /collections` · `GET/PATCH/DELETE /collections/{id}` · `POST/DELETE /collections/{id}/entries/{eid}` · `GET/POST /tags` · `DELETE /tags/{id}` · `GET /curated-decks` · `GET /curated-decks/{id}` · `POST /curated-decks/{id}/enroll`.
```
POST enroll → 202 { enrollment_id, total, cards_created_now, status } (bulk = job nền, KHÔNG đổ hết queue). 409 nếu đã enroll; 403 premium & không Pro
```

## 6. Statistics
`GET /stats/summary` · `/stats/heatmap?from&to` · `/stats/retention` · `/stats/distribution`.
```
GET /stats/summary → { streak_current, words_retained_week, total_reviews, retention_30d }
```

## 7. Notifications
`GET /notifications` · `PATCH /notifications/{id}` (ack/snooze) · `GET/PATCH /notifications/prefs` · `POST/DELETE /push/subscribe`.

## 8. Search
`GET /search?q=...&filter[...]` → { data:[{entry, highlight}], page } (MVP pg FTS; scale OpenSearch cùng contract).

## 9. Admin
`GET/POST /admin/curated-decks` · `POST /admin/curated-decks/{id}/publish` · `.../entries` · `GET /admin/users` · `PATCH /admin/users/{id}` (role/plan → **audit_logs**) · `GET /admin/metrics`. authz role ∈ {curator, admin}.

## Versioning
URL path `/api/v1`. Additive = không bump. Breaking = `/v2` song song + Deprecation/Sunset header. OpenAPI 3 sinh từ code.

## Cơ hội ẩn
1. Grade API chỉ nhận grade → không giả lịch; đổi thuật toán không đổi API.
2. `next_intervals` trong queue → client hiện interval dưới nút, không tự tính FSRS.
3. enroll 202 + job → deck khổng lồ không timeout.
4. `/me/export` hạng nhất → GDPR + chống lock-in.
5. Cursor pagination → ổn định khi data đổi, scale tốt.

**Chốt**: REST `/api/v1`, cursor-paginate, envelope lỗi chuẩn, version path. Hot path `POST /review/grade` idempotent qua clientReviewId, p95<150ms, chỉ nhận grade. Client mỏng (server trả next_intervals). Personal check owner khắp nơi.
