import { expect, test } from "@playwright/test";

test("目標カードから既存のプロジェクト詳細へ進める", async ({ page }) => {
  await page.goto("/today");
  await page.getByRole("link", { name: "today-demo", exact: true }).first().click();
  await expect(page).toHaveURL(/\/projects\?name=today-demo$/);
  await expect(page.getByText("ページが見つかりません", { exact: true })).toHaveCount(0);
  await expect(page.getByText("今日の一歩を確かめる", { exact: true }).first()).toBeVisible();
});

test("タスク画面での完了と取消は戻った今日画面にも反映する", async ({ page }) => {
  await page.goto("/today");
  await page.getByRole("link", { name: "全タスクを開く →", exact: true }).first().click();
  await page.getByRole("checkbox", { name: "履歴を残すテスト を完了にする", exact: true }).click();
  await expect(page.getByRole("checkbox", { name: "履歴を残すテスト を完了にする", exact: true })).toHaveCount(0);
  await page.getByRole("link", { name: "今日", exact: true }).click();
  const card = page.locator("section").filter({ has: page.getByRole("heading", { name: "今日の一歩を確かめる" }) }).last();
  await card.locator("summary").filter({ hasText: "完了済み" }).click();
  await expect(page.getByRole("checkbox", { name: "履歴を残すテスト", exact: true })).toBeChecked();
  await page.getByRole("checkbox", { name: "履歴を残すテスト", exact: true }).click();
  await expect(page.getByRole("checkbox", { name: "履歴を残すテスト", exact: true })).not.toBeChecked();
});

test("空のプロジェクト一覧でも次の操作を案内する", async ({ page }) => {
  await page.route("**/api/v1/today", async route => {
    const response = await route.fetch();
    const view = await response.json();
    await route.fulfill({ json: { ...view, projects: [], routines: [], board: { today: [], week: [], month: [], later: [], milestones: [] }, triage: null, skipped: [] } });
  });
  await page.goto("/today");
  await expect(page.getByText("目標をひとつ置いてみましょう", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "プロジェクトを作る", exact: true })).toHaveAttribute("href", "/projects");
});

test("モバイルの最上段から1行日記を残し、日別実績もタッチで確認できる", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/today");
  const input = page.getByLabel("今日の日記（1 行）");
  await expect(input).toBeInViewport();
  expect((await input.boundingBox())!.y).toBeLessThan(320);
  expect((await page.locator("main header").first().boundingBox())!.height).toBeLessThan(250);
  const date = (await (await page.request.get("/api/v1/today")).json()).date;
  await input.fill("E2Eの1行日記");
  await input.press("Enter");
  await expect(page.getByText("日記に追記しました", { exact: true })).toBeVisible();
  await expect(input).toHaveValue("");
  await expect(page.getByTestId("diary-days")).toContainText("今日書いた");
  const daily = await (await page.request.get(`/api/v1/daily?date=${date}`)).json();
  expect(daily.content).toMatch(/## 日記\n\n- \d{2}:\d{2} E2Eの1行日記\n/);
  await page.getByLabel("日別の記録を確認").fill(date);
  await expect(page.getByRole("status").filter({ hasText: date })).toBeVisible();
  await page.getByRole("link", { name: "日記ページで書く", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/daily\\?date=${date}$`));
  await expect(page.getByRole("button", { name: "日記を作成", exact: true })).toHaveCount(0);
});

test("目標カードは現在地を日付付きで出し、次の節目と分ける", async ({ page }) => {
  await page.goto("/today");
  const card = page.locator("section").filter({ has: page.getByRole("heading", { name: "今日の一歩を確かめる" }) }).last();
  const current = card.getByTestId("project-current");
  await expect(current).toContainText("2026-09-09");
  await expect(current).toContainText("記録の経路を先に固める。");
});

test("目標内の完了・取消と習慣をAPIへ保存し、再読込後も保持する", async ({ page }) => {
  await page.goto("/today");
  const task = page.getByRole("checkbox", { name: "履歴を残すテスト" });
  await task.click();
  const card = page.locator("section").filter({ has: page.getByRole("heading", { name: "今日の一歩を確かめる" }) }).last();
  await card.locator("summary").filter({ hasText: "完了済み" }).click();
  await expect(task).toBeChecked();
  await page.reload();
  await card.locator("summary").filter({ hasText: "完了済み" }).click();
  await expect(task).toBeChecked();
  await task.click();
  await expect(task).not.toBeChecked();
  const routine = page.getByRole("checkbox", { name: /テスト読書/ });
  await routine.click();
  await expect(routine).toBeChecked();
  await page.reload();
  await expect(routine).toBeChecked();
  await routine.click();
  await expect(routine).not.toBeChecked();
});

test("仕分けの見送りから復元でき、仕分け済みは再読込後も保持される", async ({ page }) => {
  await page.goto("/today");
  const triage = page.locator("section").filter({ has: page.getByRole("heading", { name: "溜まっているものを、ひとつだけ" }) }).last();
  const label = await triage.locator("p.h-serif").innerText();
  await triage.getByRole("button", { name: "捨てる", exact: true }).click();
  await page.getByText(/見送ったタスク .*復元/).click();
  const skipped = page.locator("details").filter({ has: page.locator("summary", { hasText: "見送ったタスク" }) });
  await expect(skipped.getByText(label, { exact: true })).toBeVisible();
  await skipped.getByRole("button", { name: "戻す", exact: true }).first().click();
  await page.reload();
  await expect(page.getByText("今日の仕分けは済みました。また明日。")).toBeVisible();
});

for (const mode of ["light", "dark"] as const) {
  test(`${mode}: mobile has no horizontal overflow`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.emulateMedia({ colorScheme: mode });
    await page.goto("/today");
    // WhetstoneはOS設定ではなく.darkクラスで切り替わる。実際のトークンを検証する。
    await page.evaluate(dark => document.documentElement.classList.toggle("dark", dark), mode === "dark");
    await expect(page.getByRole("heading", { name: "今日の一歩を確かめる" })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.classList.contains("dark"))).toBe(mode === "dark");
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  });
}

test("やったことを1行で残すと、選んだプロジェクトに予定外の完了として入る", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/today");
  await page.getByRole("tab", { name: "やったこと", exact: true }).click();
  await page.getByLabel("プロジェクト").selectOption("today-demo");
  const before = (await (await page.request.get("/api/v1/today")).json()).doneToday.unplanned;
  const input = page.getByLabel("やったこと（1 行）");
  await input.fill("E2Eで予定外の作業");
  await input.press("Enter");
  await expect(page.getByText("やったことを残しました", { exact: true })).toBeVisible();
  await expect(page.getByTestId("done-split")).toContainText(`予定外 ${before + 1}`);
  const readme = await (await page.request.get("/api/v1/files/projects/today-demo/README.md")).json();
  expect(readme.content).toMatch(/- \[x\] E2Eで予定外の作業 @done\(\d{4}-\d{2}-\d{2}\)/);
});

test("今日の予定が3件を超えると明日へ回すよう促す", async ({ page }) => {
  await page.route("**/api/v1/today", async route => {
    const view = await (await route.fetch()).json();
    const base = view.board.today[0];
    const today = [1, 2, 3, 4].map(i => ({ ...base, id: `t${i}`, line: base.line + i, display: `予定${i}`, state: "todo" }));
    await route.fulfill({ json: { ...view, board: { ...view.board, today } } });
  });
  await page.goto("/today");
  await expect(page.getByText(/今日の予定が\s*4\s*件あります/)).toBeVisible();
  await expect(page.getByRole("link", { name: "タスク画面で明日へ回す →" })).toHaveAttribute("href", "/tasks");
});
