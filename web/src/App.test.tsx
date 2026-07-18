import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import "@testing-library/jest-dom";
import App from "./App";

describe("App shell", () => {
  it("renders 4 bottom-nav tabs", () => {
    render(<App />);
    ["Trang chủ", "Ôn", "Thư viện", "Thống kê"].forEach((t) =>
      expect(screen.getByRole("button", { name: t })).toBeInTheDocument()
    );
  });
  it("switches active tab on click", () => {
    render(<App />);
    fireEvent.click(screen.getByRole("button", { name: "Thư viện" }));
    expect(screen.getByRole("heading", { name: "Thư viện" })).toBeInTheDocument();
  });
  it("toggles theme attribute", () => {
    render(<App />);
    fireEvent.click(screen.getByLabelText("Đổi giao diện"));
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
  });
});
