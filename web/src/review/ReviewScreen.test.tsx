import "@testing-library/jest-dom/vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { ReviewScreen } from "./ReviewScreen";
import type { QueueItem } from "./api";

const items: QueueItem[] = [
  { card_id: "c1", term: "ephemeral", ipa: "/ɪ'fem/", meaning: "chóng tàn", example: "ephemeral trend",
    next_intervals: { again_seconds: 600, hard_seconds: 3600, good_seconds: 86400, easy_seconds: 777600 } },
  { card_id: "c2", term: "ubiquitous", meaning: "phổ biến khắp nơi",
    next_intervals: { again_seconds: 600, hard_seconds: 3600, good_seconds: 172800, easy_seconds: 1209600 } },
];

describe("ReviewScreen", () => {
  it("mặt trước ẩn nghĩa; Space lật hiện mặt sau", () => {
    render(<ReviewScreen items={items} onGrade={vi.fn()} onDone={vi.fn()} />);
    expect(screen.getByText("ephemeral")).toBeInTheDocument();
    expect(screen.queryByText("chóng tàn")).not.toBeInTheDocument();
    fireEvent.keyDown(window, { key: " " });
    expect(screen.getByText("chóng tàn")).toBeInTheDocument();
  });

  it("nút grade hiển thị khoảng cách ôn kế", () => {
    render(<ReviewScreen items={items} onGrade={vi.fn()} onDone={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /Lật thẻ/ }));
    expect(screen.getByRole("button", { name: /Again/ })).toHaveTextContent("10 phút");
    expect(screen.getByRole("button", { name: /Good/ })).toHaveTextContent("1 ngày");
  });

  it("phím 3 chấm Good và advance sang thẻ kế ngay (optimistic)", async () => {
    const onGrade = vi.fn(async () => {});
    render(<ReviewScreen items={items} onGrade={onGrade} onDone={vi.fn()} />);
    fireEvent.keyDown(window, { key: " " }); // lật
    fireEvent.keyDown(window, { key: "3" }); // Good
    expect(onGrade).toHaveBeenCalledWith(expect.objectContaining({ card_id: "c1", grade: 3 }));
    await waitFor(() => expect(screen.getByText("ubiquitous")).toBeInTheDocument());
  });

  it("hết thẻ gọi onDone", async () => {
    const onDone = vi.fn();
    render(<ReviewScreen items={[items[0]]} onGrade={vi.fn(async () => {})} onDone={onDone} />);
    fireEvent.keyDown(window, { key: " " });
    fireEvent.keyDown(window, { key: "3" });
    await waitFor(() => expect(onDone).toHaveBeenCalled());
  });
});
