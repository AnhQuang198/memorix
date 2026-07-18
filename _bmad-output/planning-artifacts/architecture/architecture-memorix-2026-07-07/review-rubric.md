# Reviewer-Gate Review — Memorix MVP Architecture Spine

**Reviewer:** rubric-walker (good-spine checklist)
**Target:** `ARCHITECTURE-SPINE.md` (Memorix MVP, feature altitude, solo/small-team build-substrate)
**Source of truth:** `prd.md` (MVP, E1–E6)
**Date:** 2026-07-07

## Gate verdict: **PASS-WITH-FIXES**

The spine is genuinely good as a build-substrate. It fixes the real divergence points a solo/small team would otherwise decide incompatibly, no structural dimension is left silent, the seed is minimal (module-level, not a code mirror), Deferred names what it refuses to decide, and the Capability→Architecture map covers every epic. Nearly every AD passes the spine test — two independently-built units could choose incompatibly, the call is non-obvious, and there is a real trade-off. The fixes below are one diagram contradiction, one unresolved read-path decision, and a handful of low-severity polish/coverage items. None block handoff; all are cheap.

---

## 1. Per-AD judgement (Binds / Prevents / Rule + spine test)

Legend: **INV** = genuine invariant (passes spine test); **B/P/R** = has all three fields.

| AD | B/P/R | Spine test (2 units could diverge · non-obvious · real trade-off) | Verdict |
|---|---|---|---|
| AD-1 Module boundary via port + event bus | ✓ | ✓ direct-import vs port; non-obvious; coupling vs indirection | **INV — strong** |
| AD-2 Domain independent of framework/infra | ✓ | ✓ gin.Context leak into domain; non-obvious (many Go apps skip strict hexagonal); testability vs boilerplate | **INV** |
| AD-3 Grade atomic + idempotent | ✓ | ✓ TX boundary + idempotency key; non-obvious; double-grade risk | **INV — load-bearing** |
| AD-4 ReviewLog append-only = source of truth | ✓ | ✓ mutate-in-place vs event-sourced; non-obvious; enables replay/sync | **INV — load-bearing** |
| AD-5 Scheduling server-authoritative | ✓ | ✓ client vs server FSRS calc; non-obvious; anti-cheat + version lock | **INV** |
| AD-6 Entry separate from Card | ✓ | ✓ embed content in card vs reference; non-obvious; the PRD's load-bearing distinction (§4b) | **INV** |
| AD-7 FSRS via port | ✓ | ✓ inline formula vs wrapped lib; non-obvious; A/B + swap | **INV** |
| AD-8 Read model async, eventual | ✓ | ✓ Progress in hot-path TX vs event; non-obvious; latency vs consistency | **INV** — see Finding #2 |
| AD-9 Cross-module data via owner port | ✓ | ✓ join foreign tables vs ask owner; non-obvious; boundary leak. Distinct from AD-1 (reads vs calls) | **INV** |
| AD-10 FK only within same schema | ✓ | ✓ physical FK vs logical ref cross-module; non-obvious; DB coupling vs app-enforced integrity | **INV** |
| AD-11 JWT stateless + refresh rotation | ✓ | ✓ session storage + rotation/reuse-detection; non-obvious; revocation vs statelessness | **INV** |
| AD-12 Server-truth Due, user-TZ study-day | ✓ | ✓ whose clock decides Due vs "day"; non-obvious; DST/clock-skew correctness | **INV** |
| AD-13 Migration expand-and-contract | ✓ | ✓ migration discipline; non-obvious; two-version safety | **INV** — see Finding #6 (premature but cheap) |
| AD-14 API contract consistency | ✓ | ✓ per-endpoint error/pagination shapes; non-obvious; consumer uniformity | **INV** (borderline convention, but binds independent handlers → keep) |

No AD reduces to a seed decision or an obvious default. No two ADs are redundant (AD-1/AD-9 and AD-9/AD-10 are close but decide different axes: call-vs-import, read-vs-join, and physical-vs-logical FK respectively).

## 2. Structural dimensions — none silent

- **Operational / environmental envelope:** Deployment section (Docker Compose on 1 VPS, staging+prod, Prometheus/Grafana/Loki, stateless→LB scale path) + AD-13 migrations. **Covered.** Minor gap: backup/RPO (Finding #5).
- **Data ownership:** AD-6, AD-9, AD-10 + schema-per-module. **Covered — strong.**
- **Auth:** AD-11 + Auth convention row (Bearer, ownership check, deny-by-default). **Covered.** Minor: OAuth provider seam (Finding #4).
- **State mutation:** AD-3 (atomic), AD-4 (append-only truth), AD-8 (async read model), Mutation convention row. **Covered — strong.**
- **Dependency direction:** AD-1, AD-2 + the mermaid graph. **Covered but the diagram has an error (Finding #1).**

## 3. Seed minimality

Container view, core-entity ER (names + relations only, "thuộc tính là invariant → xem AD"), source tree at module granularity, deployment topology. This is a shape, not a code mirror — appropriately minimal for feature altitude. **Pass.**

## 4. Deferred

Names outbox, OpenSearch, 2-way sync/CRDT, service-split/shard, per-user FSRS optimizer, RN/Expo+PWA, and the tuning open-questions (OQ-2/3/4). Each states why it can wait and which invariant already carries the seam (e.g. replay-from-log via AD-4, boundary via AD-1/9/10). **Pass — clearly states what it won't decide.**

## 5. Capability → Architecture map

E1, E2, E2b, E3, E4, E5, E6, plus cross-cutting API/error and events — all present and each mapped to owning module + governing ADs. Traced against PRD epics: complete. **Pass.**

## 6. PRD consistency

No hard contradictions. Spot-checks: FR-15 idempotency = AD-3; FR-16/NFR-6 replay = AD-4; FR-18 = AD-12; FR-7/8 + §4b Entry/Card = AD-6; NFR-7 = AD-11; NFR-9 = AD-5; FR-22 offline-never-lose is served by AD-3's client-gen `client_review_id` seam (client internal spine deferred, but the seam is specified — good). Two soft tensions surface as Findings #2 and #4.

---

## Findings (ranked)

### Finding #1 — [MEDIUM] Dependency diagram contradicts hexagonal (AD-1/AD-2)
In "Invariants & Rules" the graph labelled *"ai được phụ thuộc ai"* (who may depend on whom) contains `repo --> service` as a solid dependency edge. Under hexagonal + AD-2, the repo is an **adapter that implements a port**; it may depend on `ports` and `domain` but must **not** depend on `service`. The correct edges are `service --> ports` and `repo -.adapter.-> ports` (both already present). The extra solid `repo --> service` inverts the dependency and is exactly the kind of call two devs would implement incompatibly (does repo import service or not?). **Fix:** delete the `repo --> service` edge; repo's only relationships are the dotted `repo -.adapter.-> ports` (+ domain types).

### Finding #2 — [MEDIUM] Immediate session summary vs eventual read model (AD-8 × FR-24/FR-31/UJ-1)
AD-8 makes Progress (daily_stats / streak / North Star) an **async, eventually-consistent** read model updated off `CardGraded` + a reconcile job. But UJ-1 / FR-24 show a celebration screen immediately after the session ("+12 từ nhớ được hôm nay") and FR-31 surfaces North Star prominently on home. If the celebration count reads the async `daily_stats`, it can be stale at exactly the moment the user looks. This is an unresolved divergence point: the review module could compute the post-session number directly from that session's `review_logs`, while another builder reads the (lagging) progress read model — two incompatible implementations. **Fix:** add a one-line rule (in AD-8 or a new AD) stating the source for the *immediate* post-session/home count — e.g. "session summary computes from the current session's review_logs (hot read of truth), not from the async read model; daily_stats/North Star aggregates may lag." Or record it as an explicit open item.

### Finding #3 — [LOW] Status-tag inconsistency across ADs
AD-1..AD-7 carry `` `[ADOPTED]` ``; AD-8..AD-14 carry no status tag. Since all fourteen are stated as binding invariants with full Binds/Prevents/Rule, the missing tags read as an editing artifact and will trip the mechanical lint pass. **Fix:** add `` `[ADOPTED]` `` to AD-8..AD-14 (or drop the tag convention entirely for uniformity).

### Finding #4 — [LOW] OAuth provider seam not governed (FR-1)
FR-1 requires email/password **and** OAuth (Google + Apple, per OQ-4 assumption). AD-11 governs the JWT/refresh session model but says nothing about the external-IdP token exchange / account-linking (same email via password + Google → one account or two?). This is a genuine cross-unit decision, though arguably seed-level at MVP. **Fix (optional):** one line in AD-11 (or convention) — "OAuth identities link to a single user by verified email; provider tokens are exchanged in identity, never trusted downstream." Otherwise leave as an open item.

### Finding #5 — [LOW] Backup / RPO not named (NFR-4)
NFR-4 requires RPO < 5 min and NFR-16 targets 99.9% uptime, but neither the Deployment section nor Deferred names a backup/PITR posture for the single Postgres. Not a two-unit divergence point (so not strictly a spine invariant), but the operational envelope leaves it silent. **Fix:** one line in Deployment ("Postgres PITR/WAL-archiving, RPO<5m") or an explicit Deferred entry.

### Finding #6 — [LOW / INFO] AD-13 expand-and-contract may be premature
Zero-downtime two-version safety only bites once rolling deploys / multiple API instances exist (Stage 4, deferred). At MVP (single instance, Docker Compose) it is not yet load-bearing. It is cheap to adopt as a discipline and prevents future pain, so **keep it** — but consider a half-sentence noting it is a forward-looking discipline, not an MVP-day constraint, so a solo builder doesn't over-invest early.

---

## Summary
Strong, comprehensive spine. All 14 ADs are genuine invariants with complete Binds/Prevents/Rule; every structural dimension is decided or explicitly deferred; seed and Deferred are well-scoped; the epic map is complete; no PRD contradictions. Ship after fixing the diagram edge (#1) and resolving the immediate-count read path (#2); the rest are polish.
