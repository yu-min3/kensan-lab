import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, it, vi } from "vitest";
import { api, ApiError } from "../../lib/api";
import { DailyPage } from "../../pages/DailyPage";
import { TodayDiary } from "./TodayDiary";

vi.mock("../editors/MilkdownEditor", () => ({ MilkdownEditor: ({ defaultValue, onChange }: { defaultValue: string; onChange: (body: string) => void }) =>
  <textarea aria-label="テスト用本文エディタ" defaultValue={defaultValue} onChange={event => onChange(event.target.value)} /> }));
afterEach(() => { cleanup(); sessionStorage.clear(); vi.restoreAllMocks(); });
function mount(dashboard = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><MemoryRouter>{dashboard ? <TodayDiary /> : <DailyPage />}</MemoryRouter></QueryClientProvider>);
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

it("shows only explicit intentions and keeps private diary text out of the dashboard", async () => {
  vi.spyOn(api, "file").mockResolvedValue({ content: "---\ntype: daily\n---\n# 日付\n\n秘密の前文\n\n## 今日やりたいこと\n\n散歩する\n\n## 日記\n\n秘密の感想", doc: { path: "daily/test.md", mtime: "initial", size: 1, meta: {} } });
  mount(true);
  await screen.findByText("散歩する");
  expect(screen.queryByText(/秘密/)).toBeNull();
  expect(screen.getByRole("link", { name: "日記を開く" }).getAttribute("href")).toBe("/daily");
  fireEvent.click(screen.getByRole("button", { name: "今日やりたいことを編集" }));
  expect((screen.getByLabelText("今日やりたいこと（ダッシュボードに表示）") as HTMLTextAreaElement).value).toBe("散歩する");
  expect(screen.queryByText(/秘密/)).toBeNull();
});
it("does not fall back to diary text when the intentions section is absent", async () => {
  mockFile();
  mount(true);
  await screen.findByRole("button", { name: "今日やりたいことを書く" });
  expect(screen.queryByText("朝のメモ")).toBeNull();
  expect(screen.queryByText("夕方のこと")).toBeNull();
});
it("retains public input on a conflict without exposing the rest of the diary", async () => {
  mockFile();
  vi.spyOn(api, "putFile").mockRejectedValue(new ApiError(409, "conflict"));
  mount(true);
  fireEvent.click(await screen.findByRole("button", { name: "今日やりたいことを書く" }));
  fireEvent.change(screen.getByRole("textbox"), { target: { value: "買い物に行く" } });
  fireEvent.click(screen.getByRole("button", { name: "保存する" }));
  await screen.findByRole("alert");
  expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("買い物に行く");
  expect(screen.queryByText("朝のメモ")).toBeNull();
});

it("keeps the original revision for recovered input and rebases explicitly after a conflict", async () => {
  const date = new Intl.DateTimeFormat("sv-SE", { timeZone: "Asia/Tokyo" }).format(new Date());
  sessionStorage.setItem(`kensan:intentions:${date}`, JSON.stringify({ text: "回収した予定", mtime: "old" }));
  vi.spyOn(api, "file").mockResolvedValue({ content: "## 今日やりたいこと\n\n他からの予定\n\n## 日記\n\n新しい個人の記録", doc: { path: "daily/test.md", mtime: "new", size: 1, meta: {} } });
  const put = vi.spyOn(api, "putFile").mockRejectedValue(new ApiError(409, "conflict"));
  mount(true);
  fireEvent.click(await screen.findByRole("button", { name: "今日やりたいことを編集" }));
  expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("回収した予定");
  fireEvent.click(screen.getByRole("button", { name: "保存する" }));
  await screen.findByRole("alert");
  expect(put.mock.calls[0][2]).toBe("old");
  fireEvent.click(screen.getByRole("button", { name: "入力を残して最新の内容を取り込む" }));
  await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
  fireEvent.click(screen.getByRole("button", { name: "保存する" }));
  await waitFor(() => expect(put).toHaveBeenCalledTimes(2));
  expect(put.mock.calls[1][2]).toBe("new");
  expect(put.mock.calls[1][1]).toContain("新しい個人の記録");
  expect(screen.queryByText("新しい個人の記録")).toBeNull();
});
