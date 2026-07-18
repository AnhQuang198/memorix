# Phase 12 — Quy trình Phát triển

> Team solo/nhỏ không cần Gitflow nặng. Chọn **trunk-based nhẹ** — nhanh, ít merge hell, hợp CD.

## Git Strategy — Trunk-Based + short-lived branches
1 nhánh `main` luôn deploy được. Feature branch **ngắn** (<2 ngày), merge sớm qua PR. Feature chưa xong → **feature flag**, không giữ branch lâu. Không Gitflow.

## Branch Naming
`feat/<scope>-<mô-tả>` · `fix/...` · `chore/` `refactor/` `docs/` `test/`. Gắn issue: `feat/123-review-queue`.

## Commit — Conventional Commits
```
<type>(<scope>): <subject ≤50 ký tự>
<body: vì sao>
<footer: BREAKING CHANGE, Refs #123>
```
type: feat|fix|chore|refactor|test|docs|perf|ci|build. Enforce commitlint. → sinh changelog + semver tự động.

## PR Workflow
Branch → code+test → PR (template: mô tả, ảnh UI, checklist, cách test) → CI xanh bắt buộc → ≥1 approve → **squash merge** → auto-deploy staging → smoke → promote prod. PR nhỏ (<400 dòng).

## Code Review Checklist
Đúng (acceptance, edge case) · Ranh giới (module qua interface/event) · Bảo mật (ownership, param SQL, validate, no secret log) · Hot path (grade nguyên tử+idempotent, index, p95) · Test (nhánh mới, integ DB) · Data (migration reversible, soft-delete filter) · FSRS (không phơi S/D, dùng lib qua port) · UX (empty/loading/error, a11y 1-4).

## Testing (kim tự tháp)
| Tầng | Công cụ | Cho |
|---|---|---|
| Unit | Go testing+testify · Vitest | domain: FSRS wrapper, queue priority, policy |
| Integration | **testcontainers (Postgres thật)** | repo, migration, grade nguyên tử, idempotency |
| Contract | OpenAPI schema test | BE↔FE khớp |
| E2E | Playwright | luồng lõi: onboarding→ôn→grade→stats |
| Load | k6 | queue build, grade p95, 10k thẻ |
**Test đặc thù bắt buộc**: idempotent grade (2 lần→1 log); replay review_logs→cùng state; chống nổ queue sau nghỉ; enroll không vượt daily new. TDD cho core (Scheduling/Review).

## CI/CD (GitHub Actions)
PR → lint+fmt (golangci-lint, eslint) → unit+integ (testcontainers) → security (govulncheck, gitleaks, npm audit) → build image → deploy staging → smoke/E2E → promote prod. Cache deps, fail-fast. Image tag = SHA+semver. Migration **trước** deploy (expand→migrate→contract).

## Release Strategy
Continuous Delivery: merge→staging tự động, prod 1 bấm/auto sau smoke. Progressive (canary/blue-green). **Feature flags** (tách deploy khỏi release). Migration expand-and-contract. Rollback = tắt flag > revert.

## Versioning
SemVer cho API (`/v1`) + client packages. Auto bump từ Conventional Commits (release-please/semantic-release) → changelog. Breaking API = `/v2` song song + sunset.

## Documentation (docs-as-code)
```
/docs
  architecture/  ADR + C4
  api/           OpenAPI (sinh từ code)
  domain/        ubiquitous language, bounded context
  runbooks/      deploy, rollback, incident, backup/restore
  onboarding/    dev setup
  fsrs/          thuật toán, weights, optimizer
README · CONTRIBUTING · CHANGELOG(auto)
```
ADR cho mọi quyết định lớn (ghi *vì sao*). Docs review cùng PR.

## Môi trường
local (Compose, seed) · CI (testcontainers ephemeral) · staging (giống prod, data ẩn danh) · production (backup+monitoring).

## Cơ hội ẩn
1. Conventional Commits → changelog+semver tự động.
2. testcontainers Postgres → bắt bug idempotency/partition mà mock giấu.
3. Expand-contract + feature flag → deploy an toàn 2 version song song, rollback = tắt flag.
4. Test replay-from-log hạng nhất → bảo vệ tài sản lõi.

**Chốt**: Trunk-based + branch ngắn + Conventional Commits + squash PR. Test kim tự tháp + test đặc thù (idempotent/replay/chống nổ). CI/CD GHA, feature flags + expand-contract migration. Docs-as-code + ADR.
