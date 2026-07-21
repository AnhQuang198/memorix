import "@testing-library/jest-dom/vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import Learn from "./Learn";

function mockFetch(session: unknown) {
  return vi.fn((url: string) => {
    if (url === "/api/v1/learn") {
      return Promise.resolve({ json: () => Promise.resolve(session) });
    }
    return Promise.resolve({ json: () => Promise.resolve({}) });
  });
}

describe("Learn screen", () => {
  beforeEach(() => vi.restoreAllMocks());

  it("shows loading skeleton before data arrives", () => {
    (globalThis as unknown as { fetch: unknown }).fetch = vi.fn(() => new Promise(() => {}));
    render(<Learn />);
    expect(screen.getByTestId("learn-skeleton")).toBeInTheDocument();
  });

  it("shows first-time coach then acks + hides on dismiss", async () => {
    const f = mockFetch({ cards: [{ id: "1", entry_id: "e", state: 0, due_at: "x" }], show_coach: true });
    (globalThis as unknown as { fetch: unknown }).fetch = f;
    render(<Learn />);
    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "Bắt đầu học" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(f).toHaveBeenCalledWith("/api/v1/learn/coach/ack", { method: "POST" });
  });

  it("hides coach when already seen", async () => {
    (globalThis as unknown as { fetch: unknown }).fetch = mockFetch({ cards: [], show_coach: false });
    render(<Learn />);
    await waitFor(() => expect(screen.getByRole("heading", { name: "Học thẻ mới" })).toBeInTheDocument());
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
