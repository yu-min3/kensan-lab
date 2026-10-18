import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, it, vi } from "vitest";
import { api, ApiError } from "../../lib/api";
import { diaryBody, TodayDiary } from "./TodayDiary";

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it("shows diary prose and nested headings without leaking adjacent sections or treating code as headings", () => {
  expect(diaryBody("---\ntype: daily\n---\n# 2026-10-18\n\n## 日記\n朝の予定\n### やりたいこと\n- 散歩\n```md\n## 学習\n```\n続きを書く\n## 学習\n別の記録"))
    .toBe("朝の予定\n### やりたいこと\n- 散歩\n```md\n## 学習\n```\n続きを書く");
});

it("keeps a draft tied to its original JST date across midnight", async () => {
  const NativeDate = Date;
  let now = "2026-10-18T14:59:00Z";
  vi.stubGlobal("Date", class extends NativeDate {
    constructor(value?: string | number) { super(value ?? now); }
    static now() { return new NativeDate(now).getTime(); }
  });
  const file = vi.spyOn(api, "file").mockRejectedValue(new ApiError(404, "not found"));
  const save = vi.spyOn(api, "dailyLine").mockRejectedValue(new Error("offline"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  try {
    render(<QueryClientProvider client={client}><MemoryRouter><TodayDiary /></MemoryRouter></QueryClientProvider>);
    const input = screen.getByLabelText("今日の日記（1 行）");
    fireEvent.change(input, { target: { value: "今夜のメモ" } });
    now = "2026-10-18T15:01:00Z";
    fireEvent(window, new Event("focus"));
    await waitFor(() => expect(screen.getByText(/日付が変わりました/).textContent).toContain("2026-10-18"));
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(save).toHaveBeenCalledWith("2026-10-18", "今夜のメモ"));
    await waitFor(() => expect(screen.getByRole("alert").textContent).toBe("offline"));
    expect((input as HTMLInputElement).value).toBe("今夜のメモ");
    expect(file).toHaveBeenCalledWith("daily/2026/10/18.md");
  } finally {
    client.clear();
    vi.unstubAllGlobals();
  }
});
