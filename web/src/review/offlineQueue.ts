import type { GradePayload } from "./api";

const KEY = "memorix.pendingGrades";

// OfflineGradeQueue đảm bảo KHÔNG mất điểm (FR-22): thử gửi ngay; nếu lỗi (offline)
// thì lưu localStorage, flush lại sau. client_review_id giữ idempotency server-side.
export class OfflineGradeQueue {
  private submit: (g: GradePayload) => Promise<unknown>;

  constructor(submit: (g: GradePayload) => Promise<unknown>) {
    this.submit = submit;
  }

  pending(): GradePayload[] {
    try {
      return JSON.parse(localStorage.getItem(KEY) ?? "[]");
    } catch {
      return [];
    }
  }

  private save(list: GradePayload[]) {
    localStorage.setItem(KEY, JSON.stringify(list));
  }

  // enqueue thử gửi ngay; thất bại → xếp hàng bền vững.
  async enqueue(g: GradePayload): Promise<void> {
    try {
      await this.submit(g);
    } catch {
      const list = this.pending();
      if (!list.some((x) => x.client_review_id === g.client_review_id)) {
        list.push(g);
        this.save(list);
      }
    }
  }

  // flush gửi lần lượt; mục nào lỗi giữ lại (không mất). An toàn gọi lặp.
  async flush(): Promise<void> {
    const list = this.pending();
    const remain: GradePayload[] = [];
    for (const g of list) {
      try {
        await this.submit(g);
      } catch {
        remain.push(g);
      }
    }
    this.save(remain);
  }
}
