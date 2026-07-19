import { describe, it, expect, vi, beforeEach } from "vitest";
import { OfflineGradeQueue } from "./offlineQueue";
import type { GradePayload } from "./api";

const p = (id: string): GradePayload => ({ card_id: id, grade: 3, client_review_id: `cr-${id}` });

beforeEach(() => localStorage.clear());

describe("OfflineGradeQueue", () => {
  it("giữ điểm khi submit lỗi và không mất khi khởi tạo lại", async () => {
    const failing = vi.fn(async () => { throw new Error("offline"); });
    const q = new OfflineGradeQueue(failing);
    await q.enqueue(p("c1"));
    expect(q.pending()).toHaveLength(1);

    // reload: hàng đợi đọc lại từ localStorage → không mất điểm
    const q2 = new OfflineGradeQueue(failing);
    expect(q2.pending()).toHaveLength(1);
  });

  it("flush gửi hết và xóa khỏi hàng đợi khi thành công", async () => {
    const sent: string[] = [];
    const ok = vi.fn(async (g: GradePayload) => { sent.push(g.client_review_id); });
    const q = new OfflineGradeQueue(ok);
    // enqueue khi đang lỗi
    const flaky = new OfflineGradeQueue(async () => { throw new Error("offline"); });
    await flaky.enqueue(p("c1"));
    await flaky.enqueue(p("c2"));

    // cùng storage key → q thấy 2 mục, flush thành công
    await q.flush();
    expect(sent).toContain("cr-c1");
    expect(sent).toContain("cr-c2");
    expect(q.pending()).toHaveLength(0);
  });

  it("enqueue online submit thành công thì không lưu lại", async () => {
    const ok = vi.fn(async () => {});
    const q = new OfflineGradeQueue(ok);
    await q.enqueue(p("c1"));
    expect(ok).toHaveBeenCalledOnce();
    expect(q.pending()).toHaveLength(0);
  });
});
