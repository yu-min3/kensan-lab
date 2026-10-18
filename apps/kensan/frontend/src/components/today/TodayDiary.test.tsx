import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, it, vi } from "vitest";
import { api, ApiError } from "../../lib/api";
import { TodayDiary } from "./TodayDiary";

vi.mock("../editors/MilkdownEditor", () => ({ MilkdownEditor: ({ defaultValue, onChange }: { defaultValue: string; onChange: (body: string) => void }) =>
  <textarea aria-label="テスト用本文エディタ" defaultValue={defaultValue} onChange={event => onChange(event.target.value)} /> }));
afterEach(() => { cleanup(); sessionStorage.clear(); vi.restoreAllMocks(); });
function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><MemoryRouter><TodayDiary /></MemoryRouter></QueryClientProvider>);
}
function mockFile() {
  vi.spyOn(api, "file").mockResolvedValue({ content: "---\ntype: daily\n---\n\n朝のメモ\n\n## 自由な見出し\n\n夕方のこと", doc: { path: "daily/test.md", mtime: "initial", size: 1, meta: {} } });
}
it("edits and appends the free-form diary in place", async () => {
  mockFile();
  const save = vi.spyOn(api, "putFile").mockResolvedValue({ doc: { path: "daily/test.md", mtime: "saved", size: 1, meta: {} } });
  mount();
  await waitFor(() => expect(screen.getByLabelText("今日の日記の本文").textContent).toContain("夕方のこと"));
  fireEvent.click(screen.getByRole("button", { name: "編集・追記" }));
  const editor = await screen.findByLabelText("テスト用本文エディタ");
  fireEvent.change(editor, { target: { value: "朝のメモ\n\n追記したこと" } });
  fireEvent.click(screen.getByRole("button", { name: "編集を終える" }));
  await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
  expect(save.mock.calls[0][1]).toContain("追記したこと");
  await waitFor(() => expect(screen.getByLabelText("今日の日記の本文").textContent).toContain("追記したこと"));
});
it("keeps the draft visible on a conflict", async () => {
  mockFile();
  vi.spyOn(api, "putFile").mockRejectedValue(new ApiError(409, "conflict"));
  mount();
  fireEvent.click(await screen.findByRole("button", { name: "編集・追記" }));
  fireEvent.change(await screen.findByLabelText("テスト用本文エディタ"), { target: { value: "消したくない入力" } });
  fireEvent.click(screen.getByRole("button", { name: "編集を終える" }));
  await waitFor(() => expect((screen.getByLabelText("保存できていない入力") as HTMLTextAreaElement).value).toBe("消したくない入力"));
});
