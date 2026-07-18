# Phase 11 — Bảo mật

> App học có email + mật khẩu + thanh toán + PII = mục tiêu thật. Và **grade endpoint** = bề mặt gian lận (client giả lịch).

## Authentication
- Mật khẩu hash **argon2id** (alt bcrypt cost≥12). Không lưu raw.
- Chống brute-force: rate-limit login theo IP+email, exponential backoff, captcha sau N fail.
- Chống enumeration: login/forgot trả message giống nhau dù email tồn tại hay không.
- Verify email trước khi cấp quyền đầy đủ.

## JWT + Refresh
- Access JWT ngắn (15m), ký RS256/EdDSA, claim tối thiểu (sub, role, exp, jti).
- Refresh opaque, lưu **hash** trong sessions, cookie **httpOnly+Secure+SameSite=Strict**.
- **Rotation** + **reuse-detection** (dùng lại token cũ → thu hồi cả family_id).
- Access **không** localStorage (XSS trộm) — in-memory; refresh cookie httpOnly.
- Logout = thu hồi refresh family server-side.

## OAuth
Authorization Code + PKCE; verify state(CSRF), nonce(replay); verify id_token sig + aud/iss; không tin email chưa verified.

## Email Verify / Reset
Token 1 lần, hash lưu DB, TTL ngắn (verify 24h, reset 1h), used_at. Reset không lộ email tồn tại; đổi mật khẩu → thu hồi mọi session.

## RBAC
Role learner|curator|admin. **2 tầng**: role gate middleware cho `/admin/*` + **ownership check** mọi personal resource. Deny by default; authz ở service layer. Đổi role/plan → **bắt buộc audit_logs**.

## Rate Limiting
Theo user+IP, sliding window (Redis). Login/forgot **chặt** (5/phút); grade/queue **nới** (không chặn học thật) + trần chống bot; AI-fill chặt + quota theo plan. Header `RateLimit-*` + Retry-After.

## CSRF / CORS
- API bearer (không cookie auth) → miễn nhiễm CSRF phần lớn. Refresh cookie → SameSite=Strict + double-submit/origin check.
- CORS whitelist origin, **không** `*` khi có credentials.

## Injection / XSS
- **sqlc/pgx parameterized** — không nối chuỗi SQL. Markdown → sanitize (DOMPurify). React auto-escape.
- **CSP** chặt: `default-src 'self'`, không inline script, whitelist audio host.

## Secrets / Audit
- Không hardcode/commit; .env local + SOPS/Vault prod; rotation; scan gitleaks CI; least-privilege DB user.
- Audit hành động nhạy cảm (role/plan/xóa/curated/login bất thường/reset). Không log PII/token (slog scrub) + trace_id.

## OWASP Top 10
| # | Chặn |
|---|---|
| A01 Access Control | ownership check, deny-default, test authz |
| A02 Crypto | argon2id, TLS, token hash, no plaintext |
| A03 Injection | sqlc param, sanitize, escape |
| A04 Insecure Design | threat model grade gian lận, rate limit tầng |
| A05 Misconfig | CSP, CORS whitelist, headers cứng |
| A06 Vulnerable Components | govulncheck/Dependabot, SCA CI |
| A07 Auth Failures | rotation, reuse-detect, brute-force limit, MFA sau |
| A08 Data Integrity | verify OAuth sig, CSP, ký release, idempotent grade |
| A09 Logging | audit + alert bất thường, scrub PII |
| A10 SSRF | validate audio_url whitelist |

## Đặc thù Memorix
- **Chống gian lận grade**: server tính S/D/Due (client chỉ gửi grade); rate-limit + phát hiện pattern bot.
- AI-fill quota theo plan; sanitize term (chống prompt injection: data ≠ instruction).
- Import: giới hạn size, validate mime, parse .apkg (SQLite) read-only.
- Export/GDPR: xác thực lại; xóa account soft→purge, thu hồi session.

## Security headers
```
Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
Content-Security-Policy: default-src 'self'; script-src 'self'; object-src 'none'; frame-ancestors 'none'
X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), microphone=()
```

## Cơ hội ẩn
1. Grade server-authoritative = kiến trúc + chống gian lận (1 quyết định, 2 lợi ích).
2. Refresh reuse-detection bắt token trộm sớm, UX 0 ma sát.
3. Prompt-injection guard cho AI-fill.
4. govulncheck + gitleaks CI (shift-left, tự động).

**Chốt**: argon2id + access-ngắn/refresh-rotation-reuse-detect, RBAC 2 tầng, rate-limit theo tầng, sqlc param + CSP + CORS whitelist, audit chọn lọc, OWASP phủ đủ. Đặc thù: grade server-authoritative chống gian lận lịch.
