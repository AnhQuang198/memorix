import "@testing-library/jest-dom/vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, afterEach } from "vitest";
import Stats from "./Stats";

function mockFetch(data: unknown, ok = true) {
  vi.stubGlobal("fetch", vi.fn(async () => ({ ok, status: ok ? 200 : 500, json: async () => ({ data }) })));
}
afterEach(() => vi.unstubAllGlobals());

const sample = {
  reviewed_today: 20,
  distribution: { again: 2, hard: 3, good: 10, easy: 5 },
  forecast: [{ day: "2026-07-19", due: 8 }, { day: "2026-07-20", due: 4 }],
  heatmap: [{ day: "2026-07-08", reviews: 10, retained: 6 }],
  streak_current: 3, streak_best: 9, retention: 0.9, total_retained: 120, north_star: 12,
};

describe("Stats", () => {
  it("skeleton khi tải", () => {
    mockFetch(sample);
    render(<Stats />);
    expect(screen.getByTestId("stats-skeleton")).toBeInTheDocument();
  });

  it("hiển thị retention, distribution, forecast, streak best", async () => {
    mockFetch(sample);
    render(<Stats />);
    await waitFor(() => expect(screen.getByText(/90%/)).toBeInTheDocument()); // retention
    expect(screen.getByText(/Đã ôn hôm nay: 20/)).toBeInTheDocument();
    expect(screen.getByLabelText("Phân bố mức chấm")).toBeInTheDocument();
    expect(screen.getByLabelText(/Dự báo tải/)).toBeInTheDocument();
    expect(screen.getByText(/Kỷ lục 9/)).toBeInTheDocument();
  });

  it("lỗi → error state", async () => {
    mockFetch(null, false);
    render(<Stats />);
    await waitFor(() => expect(screen.getByText(/Không tải được/)).toBeInTheDocument());
  });
});
