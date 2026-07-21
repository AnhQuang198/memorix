import "@testing-library/jest-dom/vitest";
import { render, screen, act } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import OfflineBanner from "./OfflineBanner";

describe("OfflineBanner", () => {
  it("ẩn khi online, hiện banner non-blocking khi offline", () => {
    render(<OfflineBanner />);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    act(() => { window.dispatchEvent(new Event("offline")); });
    const b = screen.getByRole("status");
    expect(b).toHaveTextContent(/điểm đã lưu, sẽ sync/);
    act(() => { window.dispatchEvent(new Event("online")); });
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });
});
