import { describe, it, expect, vi, beforeEach } from "vitest";
import { fetchQueue, submitGrade } from "./api";

beforeEach(() => vi.restoreAllMocks());

describe("review api", () => {
  it("fetchQueue trả data[]", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(
      JSON.stringify({ data: [{ card_id: "c1", term: "x", next_intervals: { again_seconds: 600, hard_seconds: 3600, good_seconds: 86400, easy_seconds: 777600 } }] }),
      { status: 200 })));
    const items = await fetchQueue();
    expect(items).toHaveLength(1);
    expect(items[0].term).toBe("x");
  });

  it("submitGrade gửi đúng payload {card_id,grade,client_review_id}", async () => {
    const spy = vi.fn(async (_url: string, _init: RequestInit) => new Response(JSON.stringify({ card_id: "c1", stability: 5 }), { status: 200 }));
    vi.stubGlobal("fetch", spy);
    await submitGrade({ card_id: "c1", grade: 3, client_review_id: "cr-1" });
    const [, init] = spy.mock.calls[0];
    expect(JSON.parse(init.body as string)).toEqual({ card_id: "c1", grade: 3, client_review_id: "cr-1" });
    expect(init.method).toBe("POST");
  });
});
