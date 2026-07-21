import { describe, it, expect, beforeEach, vi } from "vitest";
import { enqueueGrade, pendingGrades, flushGrades } from "./gradeQueue";
import type { PendingGrade } from "./gradeQueue";

beforeEach(() => localStorage.clear());

describe("gradeQueue (không bao giờ mất điểm)", () => {
  it("enqueue ghi vào localStorage và đọc lại được", () => {
    enqueueGrade({ card_id: "c1", grade: 3, client_review_id: "r1" });
    const p = pendingGrades();
    expect(p).toHaveLength(1);
    expect(p[0].card_id).toBe("c1");
  });

  it("giữ nhiều grade theo thứ tự", () => {
    enqueueGrade({ card_id: "c1", grade: 3, client_review_id: "r1" });
    enqueueGrade({ card_id: "c2", grade: 1, client_review_id: "r2" });
    expect(pendingGrades().map((g) => g.card_id)).toEqual(["c1", "c2"]);
  });

  it("flush gửi từng grade; giữ lại cái gửi lỗi (idempotent retry)", async () => {
    enqueueGrade({ card_id: "c1", grade: 3, client_review_id: "r1" });
    enqueueGrade({ card_id: "c2", grade: 2, client_review_id: "r2" });
    const send = vi.fn(async (g: PendingGrade) => g.card_id === "c1"); // c2 fail
    await flushGrades(send);
    expect(send).toHaveBeenCalledTimes(2);
    expect(pendingGrades().map((g) => g.card_id)).toEqual(["c2"]); // c1 xong, c2 giữ lại
  });
});
