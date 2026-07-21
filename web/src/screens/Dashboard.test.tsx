import "@testing-library/jest-dom/vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, afterEach } from "vitest";
import Dashboard from "./Dashboard";

function mockFetch(data: unknown, ok = true) {
  vi.stubGlobal("fetch", vi.fn(async () => ({ ok, status: ok ? 200 : 500, json: async () => ({ data }) })));
}

afterEach(() => vi.unstubAllGlobals());

const sample = {
  due_count: 24, new_today: 5, streak_current: 3, north_star: 12,
  heatmap: [{ day: "2026-07-08", reviews: 10, retained: 6 }],
  tomorrow_forecast: 8,
};

describe("Dashboard", () => {
  it("hiển thị skeleton khi đang tải", () => {
    mockFetch(sample);
    render(<Dashboard />);
    expect(screen.getByTestId("dashboard-skeleton")).toBeInTheDocument();
  });

  it("hiển thị due count, North Star, streak, CTA Ôn ngay", async () => {
    mockFetch(sample);
    render(<Dashboard />);
    await waitFor(() => expect(screen.getByText("24")).toBeInTheDocument());
    expect(screen.getByText(/\+12 từ nhớ được/)).toBeInTheDocument();
    expect(screen.getByText(/3/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Ôn ngay/ })).toBeInTheDocument();
  });

  it("không có thẻ due → ẩn CTA, hiện trạng thái đã xong", async () => {
    mockFetch({ ...sample, due_count: 0 });
    render(<Dashboard />);
    await waitFor(() => expect(screen.getByText(/Bạn đã ôn hết/)).toBeInTheDocument());
    expect(screen.queryByRole("button", { name: /Ôn ngay/ })).not.toBeInTheDocument();
  });

  it("lỗi tải → hiện error state với nút thử lại", async () => {
    mockFetch(null, false);
    render(<Dashboard />);
    await waitFor(() => expect(screen.getByText(/Không tải được/)).toBeInTheDocument());
    expect(screen.getByRole("button", { name: /Thử lại/ })).toBeInTheDocument();
  });
});
