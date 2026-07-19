import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { SessionSummary } from "./SessionSummary";

describe("SessionSummary", () => {
  it("hiển thị số từ nhớ được + forecast mai (không màn trống)", () => {
    render(<SessionSummary summary={{ reviewed: 12, remembered: 9, forecast_tomorrow: 15 }} />);
    expect(screen.getByText(/9/)).toBeInTheDocument();
    expect(screen.getByText(/nhớ được/i)).toBeInTheDocument();
    expect(screen.getByText(/15/)).toBeInTheDocument();
  });
});
