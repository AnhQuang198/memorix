import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import ReviewDone from "./ReviewDone";

describe("ReviewDone celebration", () => {
  it("ăn mừng với số từ nhớ được + forecast mai", () => {
    render(<ReviewDone retained={12} tomorrow={8} />);
    expect(screen.getByText(/\+12 từ nhớ được/)).toBeInTheDocument();
    expect(screen.getByText(/Ngày mai: 8 thẻ/)).toBeInTheDocument();
  });
});
