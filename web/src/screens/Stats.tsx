import { useEffect, useState, useCallback } from "react";
import { fetchStats, type Stats as StatsData } from "../lib/api";

type State =
  | { kind: "loading" }
  | { kind: "error" }
  | { kind: "ready"; data: StatsData };

const GRADE_COLORS: Record<string, string> = {
  again: "var(--again)", hard: "var(--hard)", good: "var(--good)", easy: "var(--easy)",
};

function Heatmap({ cells }: { cells: StatsData["heatmap"] }) {
  const max = Math.max(1, ...cells.map((c) => c.reviews));
  return (
    <div aria-label="Lịch học 90 ngày" style={{ display: "flex", gap: 3, flexWrap: "wrap" }}>
      {cells.map((c) => {
        const intensity = c.reviews / max;
        return (
          <span
            key={c.day}
            title={`${c.day}: ${c.reviews} ôn / ${c.retained} nhớ`}
            style={{
              width: 12, height: 12, borderRadius: 3,
              background: c.reviews > 0 ? "var(--good)" : "var(--line)",
              opacity: c.reviews > 0 ? 0.35 + 0.65 * intensity : 1,
            }}
          />
        );
      })}
    </div>
  );
}

export default function Stats() {
  const [state, setState] = useState<State>({ kind: "loading" });

  const load = useCallback(() => {
    setState({ kind: "loading" });
    fetchStats()
      .then((data) => setState({ kind: "ready", data }))
      .catch(() => setState({ kind: "error" }));
  }, []);

  useEffect(() => { load(); }, [load]);

  if (state.kind === "loading") {
    return <div data-testid="stats-skeleton" aria-busy="true" style={{ padding: 16 }}>
      <div style={{ height: 100, background: "var(--line)", borderRadius: 14, marginBottom: 12 }} />
      <div style={{ height: 160, background: "var(--line)", borderRadius: 14 }} />
    </div>;
  }
  if (state.kind === "error") {
    return <div style={{ padding: 16 }}>
      <p>Không tải được thống kê.</p>
      <button onClick={load} style={{ minHeight: "var(--tap)" }}>Thử lại</button>
    </div>;
  }

  const s = state.data;
  const distEntries = Object.entries(s.distribution) as [keyof StatsData["distribution"], number][];
  const distTotal = distEntries.reduce((a, [, v]) => a + v, 0) || 1;
  const maxDue = Math.max(1, ...s.forecast.map((f) => f.due));
  const next7 = s.forecast.slice(0, 7);
  const due7 = next7.reduce((a, f) => a + f.due, 0);
  const due30 = s.forecast.reduce((a, f) => a + f.due, 0);

  return (
    <div style={{ padding: 16, display: "grid", gap: 16 }}>
      <section style={{ background: "var(--surface)", borderRadius: 14, padding: 16 }}>
        <div>Đã ôn hôm nay: {s.reviewed_today}</div>
        <div style={{ fontSize: 28, fontWeight: 700 }}>{Math.round(s.retention * 100)}% ghi nhớ</div>
        <div style={{ color: "var(--muted)" }}>🔥 {s.streak_current} ngày · Kỷ lục {s.streak_best} · +{s.north_star} tuần này · {s.total_retained} tổng</div>
      </section>

      <section style={{ background: "var(--surface)", borderRadius: 14, padding: 16 }}>
        <Heatmap cells={s.heatmap} />
      </section>

      <section aria-label="Phân bố mức chấm" style={{ background: "var(--surface)", borderRadius: 14, padding: 16 }}>
        <div style={{ display: "flex", height: 16, borderRadius: 8, overflow: "hidden" }}>
          {distEntries.map(([k, v]) => (
            <span key={k} title={`${k}: ${v}`} style={{ width: `${(v / distTotal) * 100}%`, background: GRADE_COLORS[k] }} />
          ))}
        </div>
      </section>

      <section aria-label="Dự báo tải 30 ngày" style={{ background: "var(--surface)", borderRadius: 14, padding: 16 }}>
        <div style={{ color: "var(--muted)", marginBottom: 8 }}>7 ngày tới: {due7} thẻ · 30 ngày tới: {due30} thẻ</div>
        <div style={{ display: "flex", alignItems: "flex-end", gap: 2, height: 80 }}>
          {s.forecast.map((f) => (
            <span key={f.day} title={`${f.day}: ${f.due}`}
              style={{ flex: 1, height: `${(f.due / maxDue) * 100}%`, background: "var(--accent)", borderRadius: 2, minHeight: 2 }} />
          ))}
        </div>
        <div style={{ marginTop: 8 }}>
          <a href="/settings" style={{ color: "var(--accent)" }}>Cấu hình lịch học →</a>
        </div>
      </section>
    </div>
  );
}
