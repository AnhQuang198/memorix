export default function ReviewDone({ retained, tomorrow }: { retained: number; tomorrow: number }) {
  return (
    <div role="status" style={{ padding: 24, textAlign: "center" }}>
      <div style={{ fontSize: 48 }}>🎉</div>
      <div style={{ fontSize: 28, fontWeight: 700, color: "var(--good)" }}>+{retained} từ nhớ được</div>
      <p style={{ color: "var(--muted)" }}>Hết thẻ phiên này. Ngày mai: {tomorrow} thẻ.</p>
    </div>
  );
}
