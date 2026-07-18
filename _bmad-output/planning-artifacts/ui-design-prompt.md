# Memorix — UI Design Prompt (for Claude / design tool)

> Dán prompt này cho Claude (hoặc công cụ sinh UI) để thiết kế giao diện Memorix. Đã rút từ Phase 2 (User) + Phase 6 (UX). Ngôn ngữ UI: tiếng Việt (nội dung học = tiếng Anh, không dịch).

---

## PROMPT (copy từ đây)

You are a senior product designer. Design the UI for **Memorix**, a web app that helps serious English learners memorize vocabulary using the FSRS spaced-repetition algorithm. Goal: **maximize retention, minimize study time.** Positioning: "Anki's brain, Duolingo's polish, for the serious learner." Deliver clean, modern, calm, focused screens — not gamified-childish, not developer-ugly.

### Primary persona (design for this one first)
"Linh", 23, studying for IELTS 7.0 in 10 weeks. **Mobile-first**, studies in short bursts (bus, waiting, before bed). Wants to open the app and clear due cards in under 5 minutes with zero configuration. Values honest progress and speed over features.

### Design principles
1. **Review is sacred** — the single most-used screen. Fastest possible: 0 → grading first card in 2 taps. Zero loading between cards. No accidental exits that lose progress.
2. **One thing to do** — Home shows the due count and a big "Review now" CTA, not a cluttered dashboard.
3. **Low friction** — add a word in <10s (only the term is required). A global "＋ add word" affordance everywhere.
4. **Honest progress** — surface the North Star ("words retained this week"), real recall %, streak tied to actual recall (not just app-opens). No vanity metrics.
5. **Calm & focused** — generous whitespace, one accent color, restrained motion, no confetti spam.

### Design system to define
- **Color tokens** for light AND dark mode (theme = light/dark/system). One accent, neutral grays, semantic colors for the 4 grades (Again/Hard/Good/Easy) and card states (New/Learning/Review/Suspended). Use a brand-neutral placeholder palette; keep it accessible.
- **Typography**: readable at small sizes, clear hierarchy (term > meaning > example). IPA renders correctly.
- **Spacing/radius/elevation** scale. Touch targets ≥44px.
- **Accessibility (WCAG 2.1 AA)**: keyboard nav everywhere; grade keys 1–4 + Space to flip; visible focus; AA contrast in both themes; respect prefers-reduced-motion; don't encode state by color alone.

### Screens to design (in priority order)
1. **Review Session** (most important):
   - Front: term only, large, centered, "Flip card" button + progress "12/24" + pause.
   - Back: term + IPA + audio icon, part-of-speech, meaning, example sentence, synonyms/antonyms.
   - Four grade buttons **[Again] [Hard] [Good] [Easy]** with the **next interval shown under each** (e.g. `<1m / 10m / 4d / 9d`). Keyboard 1–4.
   - Optimistic: grading advances to next card instantly.
2. **Home / Dashboard**: greeting + streak; big "N cards due → Review now" card; new-today count; North Star ("+45 words retained this week"); mini heatmap; tomorrow forecast ("~30 cards"); FAB "＋ add word".
3. **Learn (new cards)**: like Review, with a first-time mini-onboarding explaining the 4 buttons.
4. **Vocabulary List**: search bar, filter chips (tag/status/collection/due/favorite), sort; virtualized rows showing term, POS, FSRS state badge, favorite star, audio; bulk-select.
5. **Vocabulary Detail**: full entry (multiple meanings, examples, IPA+audio, syn/ant, notes, tags, collections) + card stats (S, D, due, lapses, mini history) + actions (edit, favorite, suspend, reset, delete, AI-fill).
6. **Add/Edit Entry**: term required, rest optional; AI-fill suggestions (IPA/meaning/example/synonyms) the user approves; pick card direction, tags, collection.
7. **Statistics**: GitHub-style heatmap, streak, retention at 30/90/180 days, North Star, load forecast, S/D distribution, true-vs-desired retention.
8. **Collections** + **Curated Decks** (browse by goal IELTS/TOEFL/Business, preview, enroll), **Settings** (scheduler config: desired-retention slider, daily new/review limits + presets, timezone; theme; language; import/export; billing; GDPR).

### Navigation
- Mobile: bottom tab bar, exactly 4 items — **Home · Review · Library · Stats**. Settings via avatar. FAB for add-word. "Review" tab shows a due-count badge.
- Desktop: left sidebar + top bar (search, avatar, prominent Review button), master-detail layouts.

### Every screen needs designed states
- **Empty**: e.g. Review-done = celebrate the North Star ("Done! +12 words retained today") + tomorrow forecast — never a blank screen. Library empty = add-word + browse-decks CTAs.
- **Loading**: skeletons (not blank spinners); Review prefetches next card so there is NO loading between cards.
- **Error**: offline while reviewing = non-blocking banner "offline — grade saved, will sync"; never lose a grade or a draft.

### Responsive
- Mobile <640: bottom tabs, single column, FAB, thumb-reachable grade buttons at bottom.
- Tablet 640–1024: two-column master-detail.
- Desktop >1024: sidebar + top bar, keyboard shortcuts, master-detail.
- Review screen stays visually identical across sizes (preserve muscle memory).

### UI language
Interface copy in **Vietnamese**; learning content (English words/examples) stays in English.

### Deliverables
1. A short design-system spec (color tokens light+dark, type scale, spacing, component primitives).
2. High-fidelity mockups for the 8 priority screens above, mobile + desktop.
3. The Review screen in all states (front, back, empty/done, offline).
4. Rationale notes: for each key screen, 1–2 lines on why the layout serves speed/retention/honesty.

Before finalizing, self-review against the 5 design principles and the persona: is Review truly 2 taps to first grade? Is Home showing one clear action? Any screen missing empty/loading/error?
```

---

## END PROMPT
