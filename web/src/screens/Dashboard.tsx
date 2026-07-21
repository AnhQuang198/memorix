import { useEffect, useState, useCallback } from "react";
import { fetchDashboard, type Dashboard as DashboardData } from "../lib/api";

type State =
  | { kind: "loading" }
  | { kind: "error" }
  | { kind: "ready"; data: DashboardData };

function MiniHeatmap({ cells }: { cells: DashboardData["heatmap"] }) {
  return (
    <div aria-label="Lịch học 28 ngày" style={{ display: "flex", gap: 3, flexWrap: "wrap" }}>
      {cells.map((c) => {
        const on = c.reviews > 0;
        return (
          <span
            key={c.day}
            title={`${c.day}: ${c.reviews} ôn / ${c.retained} nhớ`}
            style={{
              width: 12, height: 12, borderRadius: 3,
              background: on ? "var(--good)" : "var(--line)",
            }}
          />
        );
      })}
    </div>
  );
}

export default function Dashboard() {
  const [state, setState] = useState<State>({ kind: "loading" });

  const load = useCallback(() => {
    setState({ kind: "loading" });
    fetchDashboard()
      .then((data) => setState({ kind: "ready", data }))
      .catch(() => setState({ kind: "error" }));
  }, []);

  useEffect(() => { load(); }, [load]);

  if (state.kind === "loading") {
    return <div data-testid="dashboard-skeleton" aria-busy="true" style={{ padding: 16 }}>
      <div style={{ height: 80, background: "var(--line)", borderRadius: 14, marginBottom: 12 }} />
      <div style={{ height: 120, background: "var(--line)", borderRadius: 14 }} />
    </div>;
  }
  if (state.kind === "error") {
    return <div style={{ padding: 16 }}>
      <p>Không tải được trang chủ.</p>
      <button onClick={load} style={{ minHeight: "var(--tap)" }}>Thử lại</button>
    </div>;
  }

  const d = state.data;
  return (
    <div style={{ padding: 16, display: "grid", gap: 16 }}>
      <section style={{ background: "var(--surface)", borderRadius: 14, padding: 16 }}>
        {d.due_count > 0 ? (
          <>
            <div style={{ fontSize: 40, fontWeight: 700 }}>{d.due_count}</div>
            <div style={{ color: "var(--muted)" }}>thẻ đến hạn · {d.new_today} thẻ mới hôm nay</div>
            <button style={{ marginTop: 12, minHeight: "var(--tap)", background: "var(--accent)", color: "#fff", border: "none", borderRadius: 12, padding: "0 20px" }}>
              Ôn ngay
            </button>
          </>
        ) : (
          <p>🎉 Bạn đã ôn hết! Quay lại sau nhé.</p>
        )}
      </section>

      <section style={{ background: "var(--surface)", borderRadius: 14, padding: 16 }}>
        <div style={{ fontSize: 24, fontWeight: 700, color: "var(--good)" }}>+{d.north_star} từ nhớ được</div>
        <div style={{ color: "var(--muted)" }}>tuần này · 🔥 streak {d.streak_current} ngày</div>
      </section>

      <section style={{ background: "var(--surface)", borderRadius: 14, padding: 16 }}>
        <MiniHeatmap cells={d.heatmap} />
        <div style={{ marginTop: 8, color: "var(--muted)" }}>Ngày mai: {d.tomorrow_forecast} thẻ</div>
      </section>
    </div>
  );
}
