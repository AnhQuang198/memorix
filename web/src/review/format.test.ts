import { describe, it, expect } from "vitest";
import { formatInterval } from "./format";

describe("formatInterval", () => {
  it("phút cho < 1 giờ", () => expect(formatInterval(600)).toBe("10 phút"));
  it("giờ cho < 1 ngày", () => expect(formatInterval(3600)).toBe("1 giờ"));
  it("ngày cho >= 1 ngày", () => expect(formatInterval(4 * 86400)).toBe("4 ngày"));
  it("tháng khi lớn", () => expect(formatInterval(60 * 86400)).toBe("2 tháng"));
});
