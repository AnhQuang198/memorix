import { useCallback, useEffect, useState } from "react";
import type { GradePayload, NextIntervals, QueueItem } from "./api";
import { formatInterval } from "./format";

interface Props {
  items: QueueItem[];
  onGrade: (g: GradePayload) => Promise<void>;
  onDone: () => void;
}

const GRADES: ReadonlyArray<{
  g: number;
  label: string;
  key: string;
  color: string;
  field: keyof NextIntervals;
}> = [
  { g: 1, label: "Again", key: "1", color: "var(--again)", field: "again_seconds" },
  { g: 2, label: "Hard", key: "2", color: "var(--hard)", field: "hard_seconds" },
  { g: 3, label: "Good", key: "3", color: "var(--good)", field: "good_seconds" },
  { g: 4, label: "Easy", key: "4", color: "var(--easy)", field: "easy_seconds" },
];

// clientReviewID ổn định theo (card, lần chấm) để server idempotent (AD-3).
function reviewID(cardID: string): string {
  return `${cardID}:${Date.now()}:${Math.random().toString(36).slice(2, 8)}`;
}

export function ReviewScreen({ items, onGrade, onDone }: Props) {
  const [idx, setIdx] = useState(0);
  const [flipped, setFlipped] = useState(false);
  const card = items[idx];

  const advance = useCallback(() => {
    setFlipped(false);
    if (idx + 1 >= items.length) {
      onDone();
    } else {
      setIdx((i) => i + 1);
    }
  }, [idx, items.length, onDone]);

  const grade = useCallback(
    (g: number) => {
      if (!card) return;
      const payload: GradePayload = { card_id: card.card_id, grade: g, client_review_id: reviewID(card.card_id) };
      // optimistic: advance NGAY, gửi nền (không chờ mạng — FR-21).
      void onGrade(payload);
      advance();
    },
    [card, onGrade, advance],
  );

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === " ") {
        e.preventDefault();
        setFlipped(true);
        return;
      }
      if (flipped && ["1", "2", "3", "4"].includes(e.key)) {
        e.preventDefault();
        grade(Number(e.key));
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [flipped, grade]);

  if (!card) return null;

  return (
    <div style={{ maxWidth: 560, margin: "0 auto", padding: 16 }}>
      <p style={{ color: "var(--muted)" }} aria-live="polite">
        {idx + 1} / {items.length}
      </p>

      <section
        aria-label="Thẻ ôn"
        style={{ background: "var(--surface)", border: "1px solid var(--line)", borderRadius: "var(--radius)", padding: 24, textAlign: "center" }}
      >
        <h1 style={{ fontSize: 32 }}>{card.term}</h1>
        {flipped && (
          <div>
            {card.ipa && <p style={{ color: "var(--muted)" }}>{card.ipa}</p>}
            {card.meaning && <p style={{ fontSize: 18 }}>{card.meaning}</p>}
            {card.example && <p style={{ fontStyle: "italic", color: "var(--muted)" }}>{card.example}</p>}
          </div>
        )}
      </section>

      {!flipped ? (
        <button
          onClick={() => setFlipped(true)}
          style={{ width: "100%", minHeight: "var(--tap)", marginTop: 16, borderRadius: "var(--radius)", border: "none", background: "var(--accent)", color: "#fff", fontSize: 16 }}
        >
          Lật thẻ (Space)
        </button>
      ) : (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(4,1fr)", gap: 8, marginTop: 16 }}>
          {GRADES.map((b) => (
            <button
              key={b.g}
              onClick={() => grade(b.g)}
              aria-label={`${b.label} — phím ${b.key}`}
              style={{ minHeight: "var(--tap)", borderRadius: "var(--radius)", border: "none", background: b.color, color: "#fff", padding: 8 }}
            >
              <span style={{ display: "block", fontWeight: 600 }}>{b.label}</span>
              <span style={{ display: "block", fontSize: 12 }}>{formatInterval(card.next_intervals[b.field])}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
