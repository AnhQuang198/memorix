import { useEffect, useMemo, useState } from "react";
import type { GradePayload, QueueItem, SessionSummary as Sum } from "./api";
import { OfflineGradeQueue } from "./offlineQueue";
import { ReviewScreen } from "./ReviewScreen";
import { SessionSummary } from "./SessionSummary";

interface Props {
  loadQueue: () => Promise<QueueItem[]>;
  submitGrade: (g: GradePayload) => Promise<unknown>;
  loadSummary: () => Promise<Sum>;
}

type Phase = "loading" | "review" | "summary";

export function ReviewSession({ loadQueue, submitGrade, loadSummary }: Props) {
  const [phase, setPhase] = useState<Phase>("loading");
  const [items, setItems] = useState<QueueItem[]>([]);
  const [summary, setSummary] = useState<Sum | null>(null);
  const [offline, setOffline] = useState(false);

  const queue = useMemo(() => new OfflineGradeQueue(submitGrade), [submitGrade]);

  useEffect(() => {
    void (async () => {
      const q = await loadQueue();
      setItems(q);
      setPhase(q.length === 0 ? "summary" : "review");
      if (q.length === 0) setSummary(await loadSummary());
    })();
  }, [loadQueue, loadSummary]);

  const onGrade = async (g: GradePayload) => {
    const before = queue.pending().length;
    await queue.enqueue(g);
    if (queue.pending().length > before) setOffline(true); // lưu offline
  };

  const onDone = async () => {
    void queue.flush();
    setSummary(await loadSummary());
    setPhase("summary");
  };

  return (
    <div>
      {offline && (
        <div role="status" style={{ background: "var(--hard)", color: "#fff", padding: 8, textAlign: "center" }}>
          Mất mạng — điểm đã lưu, sẽ đồng bộ sau.
        </div>
      )}
      {phase === "loading" && <p style={{ padding: 24, color: "var(--muted)" }}>Đang tải…</p>}
      {phase === "summary" && summary && <SessionSummary summary={summary} />}
      {phase === "review" && <ReviewScreen items={items} onGrade={onGrade} onDone={onDone} />}
    </div>
  );
}
