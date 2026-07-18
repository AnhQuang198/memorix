# PRD Quality Review — Memorix MVP (2026-07-07)

## Overall verdict

**Gate: PASS-WITH-FIXES.** This is a genuinely strong MVP PRD for a solo/small team: it has a stated bet, a North Star chosen to validate that bet (not vanity activity), honest counter-metrics, ruthless and explicit scope-cutting, and NFRs with real product-specific thresholds. It reads as a thesis, not a backlog. What holds it back from a clean PASS is a small set of internal inconsistencies (OQ-3 and OQ-4 are marked "open" in §8 but treated as decided in FR-1 and the addendum) and the absence of a Glossary / Assumptions Index for downstream extraction. None of these are structural; they are reconciliation edits. The done-ness of a handful of FRs (FR-28, FR-33, FR-34) leans on adjectives that should be bounded before story creation.

---

## Decision-readiness — strong

The PRD is decision-ready. §1 states the core bet explicitly ("Cược cốt lõi cần validate ở MVP") and — crucially — states what the MVP is *not* for ("không phải để kiếm tiền (billing hoãn sang V1.5)"). Trade-offs are named with what was given up: web-first accepts losing mobile-only users (§7 assumption), PWA-before-RN is a deliberate ceiling accepted for cheap validation (addendum Rejected alternatives). The exit criterion in §9 ties the go/no-go decision directly to the thesis: hit or learn-why-you-missed D7/D30 + real recall → decide whether to build V1. A decision-maker can act on this.

Open Questions are mostly real: OQ-2 (anti-flood threshold) and OQ-4 (OAuth scope) are genuinely unresolved and deferred to data/decision. The strikethrough-and-"ĐÃ CHỐT" convention on OQ-1 and OQ-5 is a clean, honest way to show decisions that moved.

### Findings
- **medium** OQ-3 stated as open but treated as closed (§8 OQ-3 vs addendum "North Star") — §8 says "Cần chốt để đo nhất quán" while the addendum states `N=7 (chốt ở OQ-3)`. A reader cannot tell if N=7 is decided. *Fix:* mark OQ-3 ĐÃ CHỐT (N=7) like OQ-1/OQ-5, or remove the "chốt" claim from the addendum.

## Substance over theater — strong

Little furniture here. **Personas** pass the test: one driving persona (Linh) whose constraints (mobile, fragmented study, <5 min, no configuration) directly shape FR-19–24, FR-23 (keyboard), and the queue/anti-flood design; two secondary personas explicitly labeled "phục vụ ké, không tối ưu riêng" — no persona is padding. **Vision** ("não Anki, độ mượt Duolingo") is specific and would not swap cleanly into another PRD. **NFRs** carry product-specific bounds (NFR-1 p95<150ms, NFR-2 10k cards<500ms, NFR-4 RPO<5min), not "must be scalable/secure." **Counter-metrics** (§2) are the strongest signal of non-theater: each names a concrete failure mode (study-time up but retention flat; streak up but real recall down; cards added up but review-rate down). This is a PRD that knows how it could fool itself.

### Findings
- (none material)

## Strategic coherence — strong

The PRD has a clear thesis and the features serve it. Thesis: serious self-learners will use (and later pay for) an FSRS engine with better UX than Anki; MVP validates this via measurable retention. The North Star ("Số Từ Nhớ Được Mỗi Tuần" gated on next interval ≥ N days) is deliberately built to validate *teaching*, not activity, and §2 explicitly rejects gaming via "hoạt động ảo." Feature prioritization follows the thesis: E3 (FSRS core) and E5 (queue/anti-flood) are the load-bearing bets; the seed deck (E2b) exists specifically to make retention measurable in beta by killing cold-start. Scope kind is coherent — a problem-solving/experience validation MVP with matching scope logic.

### Findings
- (none material)

## Done-ness clarity — adequate

Most FRs carry a testable consequence. Strong examples: FR-7 (<10s to add, only term required), FR-15 (idempotent grading — mechanism `unique(card_id, client_review_id)` in addendum), FR-22 ("không bao giờ mất điểm đã chấm"), FR-18 (server-time day boundary), FR-14/FR-25 (backed by concrete addendum contracts — `next_intervals` payload, priority formula). There is no dedicated Acceptance Criteria section, but FR consequences plus the Milestones "Done" column (§9) carry it acceptably for this scope.

The soft spots cluster in E5/E6. These need bounding before story creation, which will lean on this dimension hardest.

### Findings
- **high** FR-28 anti-flood is unbounded (§5 E5 / OQ-2) — "giới hạn số thẻ hiển thị và rải overdue qua nhiều ngày" has no testable number; OQ-2 admits the threshold is open. An engineer cannot know when this is done. *Fix:* set a provisional default (e.g. cap shown ≤ daily review limit; spread overdue over K days) even if tuned later — a testable placeholder beats an adjective.
- **medium** FR-33 "thống kê cơ bản" underspecified (§5 E6) — lists "đã ôn hôm nay, phân bố mức chấm, dự báo tải ngày tới" but "dự báo tải" has no definition of horizon or method. *Fix:* name the forecast window (e.g. next 7 days due count).
- **low** FR-24 / FR-34 celebration screens rely on subjective "ăn mừng" (§5 E4/E6) — testable content ("số từ nhớ được, forecast ngày mai") is present, so this is minor; just ensure the celebratory framing isn't the acceptance bar.
- **low** FR-11 "cuộn mượt (ảo hóa)" (§5 E2) — "mượt" is an adjective, but "ảo hóa" (virtualization) names the mechanism, so this is testable enough.

## Scope honesty — strong

This is the PRD's best dimension. §4 has an explicit "Ngoài MVP" list that does real work (billing, curated marketplace, multi-device sync, AI fill, PWA offline, RN all named as deferred), closed with the deliberate statement "Cắt tàn nhẫn có chủ đích: MVP validate cược retention, không phải phủ tính năng." E2b explicitly fences its own scope ("Không gồm duyệt nhiều deck… marketplace (đó là V1)"). Assumptions are tagged `[ASSUMPTION]` in §7 and the metrics table; decisions that moved are shown via strikethrough + ĐÃ CHỐT. Open-items density is low and appropriate for a green-light MVP. Nothing is de-scoped silently.

### Findings
- **low** Assumptions are tagged inline but not collected in an Assumptions Index. *Fix:* add a short index (or note that §7 is the index) so downstream can roundtrip them.

## Downstream usability — adequate

This PRD is chain-top (feeds architecture via the addendum, and story creation via Milestones), so traceability matters. IDs are unique and cross-references resolve: FR-11b cites FR-27; the addendum maps FR-14/15/16/25/28 and NFR-5/9 to concrete mechanisms. UJs have named protagonists (Linh, Minh) carrying context inline. The addendum cleanly separates "how" from the capability-level PRD.

The gap is the absence of a Glossary. Domain nouns (Stability/Difficulty/Due, queue, streak, North Star, "thẻ mới", "bộ khởi đầu"/seed, Entry vs Card) are used consistently, but Entry-vs-Card — a load-bearing distinction — is only defined in the addendum's Domain section, not in the PRD proper. For a chain-top PRD this should be surfaced.

### Findings
- **medium** No Glossary; Entry vs Card distinction lives only in addendum (addendum "Domain") — this is the central content-vs-learning-unit split and drives curated-sharing semantics. *Fix:* add a short Glossary to the PRD defining Entry, Card, S/D/Due, queue, seed deck, North Star.
- **low** FR numbering inserts FR-11a/b/c between FR-11 and FR-12 (§5 E2b) — intentional and readable, but breaks strict contiguity. Acceptable; note it so future edits don't collide.

## Shape fit — strong

The shape matches the product. Memorix is a consumer product with meaningful UX, so UJs with named protagonists are load-bearing — and they are: UJ-1 (Linh's daily session) directly generates the E4 review-flow FRs; UJ-3 (Minh's return after a break) directly motivates FR-28. Rigor is appropriately light for solo/small team (no enterprise ceremony), while the substance bar is met. The addendum pattern — pushing stack, mechanisms, and rejected alternatives out of the capability-level PRD — is exactly right for a chain-top document that must stay readable while still feeding architecture. Not over-formalized, not under-formalized.

### Findings
- (none material)

---

## Mechanical notes

- **OQ-4 vs FR-1 tension.** FR-1 states "OAuth (Google/Apple)" as if both are in scope, while OQ-4 (§8) leaves open whether MVP ships both or Google-first with `[ASSUMPTION: cả hai]`. Reconcile FR-1's wording with the open question, or close OQ-4.
- **OQ-3 vs addendum** (repeated from Decision-readiness): "Cần chốt" in §8 contradicts "chốt ở OQ-3" in the addendum. Pick one.
- **No dedicated Glossary or Assumptions Index sections** (repeated above) — the main downstream-extraction gap.
- **ID continuity:** FR-1–34 (with FR-11a/b/c inserted), NFR-1–16, UJ-1–3, OQ-1–5, G1–G4 — all unique, no dangling cross-refs found. FR-11a/b/c is the only numbering irregularity.
- **Metric caveat is honest:** "Recall thật tại 30 ngày ≥85%" is tagged as derived from desired retention 0.90, and §7 states the beta targets have no baseline yet — good scope honesty, no fix needed.
