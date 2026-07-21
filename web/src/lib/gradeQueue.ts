export type PendingGrade = { card_id: string; grade: number; client_review_id: string };

const KEY = "memorix.pendingGrades";

function read(): PendingGrade[] {
  try {
    return JSON.parse(localStorage.getItem(KEY) ?? "[]") as PendingGrade[];
  } catch {
    return [];
  }
}
function write(list: PendingGrade[]) {
  localStorage.setItem(KEY, JSON.stringify(list));
}

export function enqueueGrade(g: PendingGrade): void {
  write([...read(), g]);
}

export function pendingGrades(): PendingGrade[] {
  return read();
}

// flushGrades gửi từng grade; giữ lại cái gửi lỗi để retry (server idempotent theo client_review_id).
export async function flushGrades(send: (g: PendingGrade) => Promise<boolean>): Promise<void> {
  const remaining: PendingGrade[] = [];
  for (const g of read()) {
    let ok = false;
    try {
      ok = await send(g);
    } catch {
      ok = false;
    }
    if (!ok) remaining.push(g);
  }
  write(remaining);
}
