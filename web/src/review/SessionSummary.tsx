import type { SessionSummary as Sum } from "./api";

export function SessionSummary({ summary }: { summary: Sum }) {
  return (
    <div style={{ maxWidth: 480, margin: "0 auto", padding: 24, textAlign: "center" }}>
      <div style={{ fontSize: 48 }}>🎉</div>
      <h1>Hoàn thành phiên ôn!</h1>
      <p style={{ fontSize: 28, color: "var(--accent)", fontWeight: 700 }}>
        {summary.remembered} từ nhớ được
      </p>
      <p style={{ color: "var(--muted)" }}>Đã ôn {summary.reviewed} thẻ phiên này.</p>
      <p style={{ color: "var(--muted)" }}>Mai có khoảng {summary.forecast_tomorrow} thẻ đến hạn.</p>
    </div>
  );
}
