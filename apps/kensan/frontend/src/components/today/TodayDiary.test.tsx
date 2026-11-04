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
  fireEvent.change(screen.getByRole("textbox", { name: /やりたいこと（ダッシュボードに表示）/ }), { target: { value: "買い物に行く" } });
  fireEvent.click(screen.getByRole("button", { name: "保存する" }));
  await screen.findByRole("alert");
  expect((screen.getByRole("textbox", { name: /やりたいこと（ダッシュボードに表示）/ }) as HTMLTextAreaElement).value).toBe("買い物に行く");
  expect(screen.queryByText("朝のメモ")).toBeNull();
});

it("keeps the original revision for recovered input and rebases explicitly after a conflict", async () => {
  const date = new Intl.DateTimeFormat("sv-SE", { timeZone: "Asia/Tokyo" }).format(new Date());
  sessionStorage.setItem(`kensan:intentions:${date}`, JSON.stringify({ text: "回収した予定", mtime: "old" }));
  vi.spyOn(api, "file").mockResolvedValue({ content: "## 今日やりたいこと\n\n他からの予定\n\n## 日記\n\n新しい個人の記録", doc: { path: "daily/test.md", mtime: "new", size: 1, meta: {} } });
  const put = vi.spyOn(api, "putFile").mockRejectedValue(new ApiError(409, "conflict"));
  mount(true);
  fireEvent.click(await screen.findByRole("button", { name: "今日やりたいことを編集" }));
  expect((screen.getByRole("textbox", { name: /やりたいこと（ダッシュボードに表示）/ }) as HTMLTextAreaElement).value).toBe("回収した予定");
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

it("明日の予定を別の日付へ保存し、タブ切替でも入力を保持する",async()=>{
 const date=new Intl.DateTimeFormat("sv-SE",{timeZone:"Asia/Tokyo"}).format(new Date());
 const next=new Date(`${date}T12:00:00Z`);next.setUTCDate(next.getUTCDate()+1);const tomorrow=next.toISOString().slice(0,10);
 vi.spyOn(api,"file").mockImplementation(async path=>({content:`## 今日やりたいこと\n\n${path.includes(tomorrow.replaceAll("-","/"))?"明日の予定A":"今日の予定A"}\n\n## 日記\n\nPRIVATE BODY`,doc:{path,mtime:"base",size:1,meta:{}}}));
 const put=vi.spyOn(api,"putFile").mockResolvedValue({doc:{path:"daily/test.md",mtime:"saved",size:1,meta:{}}});
 mount(true);fireEvent.click(screen.getByRole("tab",{name:"明日やりたいことを書く"}));
 fireEvent.click(await screen.findByRole("button",{name:"明日やりたいことを編集"}));
 fireEvent.change(screen.getByRole("textbox", { name: /やりたいこと（ダッシュボードに表示）/ }),{target:{value:"明日は読書"}});
 fireEvent.click(screen.getByRole("tab",{name:"今日やりたいこと"}));
 await screen.findByText("今日の予定A");fireEvent.click(screen.getByRole("tab",{name:"明日やりたいことを書く"}));
 fireEvent.click(await screen.findByRole("button",{name:"明日やりたいことを編集"}));expect((screen.getByRole("textbox", { name: /やりたいこと（ダッシュボードに表示）/ }) as HTMLTextAreaElement).value).toBe("明日は読書");
 fireEvent.click(screen.getByRole("button",{name:"保存する"}));await waitFor(()=>expect(put).toHaveBeenCalledTimes(1));
 expect(put.mock.calls[0][0]).toBe(`daily/${tomorrow.replaceAll("-","/")}.md`);expect(put.mock.calls[0][1]).toContain("PRIVATE BODY");expect(screen.queryByText("PRIVATE BODY")).toBeNull();
});
it("一行日記を予定から分離して保存し、その場で編集する",async()=>{
 let content="## 今日やりたいこと\n\n散歩する\n\n## 日記\n\nPRIVATE BODY\n";let revision="base";
 vi.spyOn(api,"file").mockImplementation(async path=>({content,doc:{path,mtime:revision,size:1,meta:{}}}));
 vi.spyOn(api,"putFile").mockImplementation(async(path,next,mtime)=>{expect(mtime).toBe(revision);content=next;revision+="1";return {doc:{path,mtime:revision,size:1,meta:{}}};});
 mount(true);await screen.findByText("散歩する");
 fireEvent.change(screen.getByRole("textbox",{name:"一行日記"}),{target:{value:"いい一日だった"}});fireEvent.click(screen.getByRole("button",{name:"日記に残す"}));
 fireEvent.click(await screen.findByRole("button",{name:"今の一行を編集"}));fireEvent.change(screen.getByRole("textbox",{name:"一行日記"}),{target:{value:"読書できてよかった"}});fireEvent.click(screen.getByRole("button",{name:"変更を保存"}));
 await waitFor(()=>expect((screen.getByRole("textbox",{name:"一行日記"}) as HTMLTextAreaElement).value).toBe(""));expect(content).toContain("読書できてよかった");expect(screen.queryByText("読書できてよかった")).toBeNull();expect(screen.getByText("散歩する")).toBeTruthy();expect(content).not.toContain("いい一日だった");expect(content).toContain("散歩する");expect(screen.queryByText("PRIVATE BODY")).toBeNull();
});

it("日記画面の一行追加も予定節へ混入させない", async () => {
 const before="---\ntype: daily\n---\n# 日付\n\n## 今日やりたいこと\n\n散歩する\n";
 vi.spyOn(api,"file").mockResolvedValue({content:before,doc:{path:"daily/test.md",mtime:"base",size:1,meta:{}}});
 const put=vi.spyOn(api,"putFile").mockResolvedValue({doc:{path:"daily/test.md",mtime:"saved",size:1,meta:{}}});
 mount();
 await screen.findByText("散歩する");
 fireEvent.change(screen.getByRole("textbox",{name:"日記（1 行）"}),{target:{value:"個人の日記を追記"}});
 fireEvent.click(screen.getByRole("button",{name:"残す"}));
 await waitFor(()=>expect(put).toHaveBeenCalledTimes(1));
 expect(put.mock.calls[0][1]).toMatch(/散歩する\s+## 日記\s+- \d{2}:\d{2} 個人の日記を追記/);
});
