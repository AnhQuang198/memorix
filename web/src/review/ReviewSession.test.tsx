import "@testing-library/jest-dom/vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { ReviewSession } from "./ReviewSession";
import type { QueueItem, SessionSummary as Sum } from "./api";

const items: QueueItem[] = [
  { card_id: "c1", term: "ephemeral", meaning: "chóng tàn",
    next_intervals: { again_seconds: 600, hard_seconds: 3600, good_seconds: 86400, easy_seconds: 777600 } },
];
const sum: Sum = { reviewed: 1, remembered: 1, forecast_tomorrow: 3 };

describe("ReviewSession", () => {
  it("chấm hết thẻ → chuyển sang màn tổng kết", async () => {
    const submit = vi.fn(async () => {});
    render(<ReviewSession loadQueue={async () => items} submitGrade={submit} loadSummary={async () => sum} />);
    await waitFor(() => screen.getByText("ephemeral"));
    fireEvent.keyDown(window, { key: " " });
    fireEvent.keyDown(window, { key: "3" });
    await waitFor(() => expect(screen.getByText(/nhớ được/i)).toBeInTheDocument());
  });

  it("submit lỗi (offline) vẫn advance + hiện banner, không mất điểm", async () => {
    const submit = vi.fn(async () => { throw new Error("offline"); });
    render(<ReviewSession loadQueue={async () => items} submitGrade={submit} loadSummary={async () => sum} />);
    await waitFor(() => screen.getByText("ephemeral"));
    fireEvent.keyDown(window, { key: " " });
    fireEvent.keyDown(window, { key: "3" });
    await waitFor(() => expect(screen.getByText(/điểm đã lưu/i)).toBeInTheDocument());
  });
});
