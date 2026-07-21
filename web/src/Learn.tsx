import { useEffect, useState } from "react";

type Card = { id: string; entry_id: string; status: string; due_at: string };
type Session = { cards: Card[]; show_coach: boolean };

const GRADES = [
  { key: "1", label: "Again", desc: "Quên hẳn — học lại từ đầu" },
  { key: "2", label: "Hard", desc: "Nhớ nhưng khó khăn" },
  { key: "3", label: "Good", desc: "Nhớ được như mong đợi" },
  { key: "4", label: "Easy", desc: "Nhớ rất dễ dàng" },
];

export default function Learn() {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [coach, setCoach] = useState(false);

  useEffect(() => {
    fetch("/api/v1/learn")
      .then((r) => r.json())
      .then((s: Session) => {
        setSession(s);
        setCoach(s.show_coach);
        setLoading(false);
      });
  }, []);

  const dismissCoach = () => {
    setCoach(false);
    fetch("/api/v1/learn/coach/ack", { method: "POST" });
  };

  if (loading) {
    return (
      <div data-testid="learn-skeleton" aria-busy="true">
        <div className="skeleton" style={{ height: 220 }} />
        <div className="skeleton" style={{ height: 48, marginTop: 12 }} />
      </div>
    );
  }

  return (
    <section>
      {coach && (
        <div role="dialog" aria-label="Hướng dẫn chấm thẻ" style={{ background: "var(--surface)", padding: 16, borderRadius: "var(--radius)" }}>
          <h2>Cách chấm thẻ mới</h2>
          <ul>
            {GRADES.map((g) => (
              <li key={g.key}>
                <strong>{g.key} · {g.label}</strong> — {g.desc}
              </li>
            ))}
          </ul>
          <button onClick={dismissCoach} style={{ minHeight: "var(--tap)", color: "var(--accent)" }}>
            Bắt đầu học
          </button>
        </div>
      )}
      <h1>Học thẻ mới</h1>
      <p style={{ color: "var(--muted)" }}>{session?.cards.length ?? 0} thẻ mới hôm nay</p>
    </section>
  );
}
