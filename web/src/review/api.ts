export interface NextIntervals {
  again_seconds: number;
  hard_seconds: number;
  good_seconds: number;
  easy_seconds: number;
}

export interface QueueItem {
  card_id: string;
  entry_id?: string;
  direction?: string;
  term: string;
  ipa?: string;
  meaning?: string;
  example?: string;
  next_intervals: NextIntervals;
}

export interface GradePayload {
  card_id: string;
  grade: number; // 1..4
  client_review_id: string;
}

export interface GradeResult {
  card_id: string;
  stability: number;
  due_at?: string;
}

export interface SessionSummary {
  reviewed: number;
  remembered: number;
  forecast_tomorrow: number;
}

const BASE = "/api/v1";

export async function fetchQueue(): Promise<QueueItem[]> {
  const res = await fetch(`${BASE}/review/queue`, { credentials: "include" });
  if (!res.ok) throw new Error(`queue ${res.status}`);
  const body = await res.json();
  return body.data ?? [];
}

export async function submitGrade(p: GradePayload): Promise<GradeResult> {
  const res = await fetch(`${BASE}/review/grade`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(p),
  });
  if (!res.ok) throw new Error(`grade ${res.status}`);
  return res.json();
}

export async function fetchSummary(): Promise<SessionSummary> {
  const res = await fetch(`${BASE}/review/summary`, { credentials: "include" });
  if (!res.ok) throw new Error(`summary ${res.status}`);
  return res.json();
}
