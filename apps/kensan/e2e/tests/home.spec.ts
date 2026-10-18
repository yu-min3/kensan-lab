import { expect, test } from "@playwright/test";

const editor = (page: import("@playwright/test").Page) => page.getByLabel("日記を編集").locator('[contenteditable="true"]');

async function createDiary(page: import("@playwright/test").Page, date: string) {
  await page.goto(`/?date=${date}`);
  const write = page.getByRole("button", { name: "日記を書く", exact: true });
  if (await write.isVisible()) await write.click();
  else {
    await expect(page.getByRole("heading", { name: `${date} の日記` })).toBeVisible();
    if (await write.isVisible()) await write.click();
    else await page.getByRole("button", { name: "編集・追記", exact: true }).click();
  }
  await expect(editor(page)).toBeVisible();
}

test("ホームから日記の作成・編集・追記・再読込を完結できる", async ({ page }) => {
  const date = "2026-01-16";
  await createDiary(page, date);
  await editor(page).press("ControlOrMeta+End");
  await page.keyboard.press("Enter");
  await page.keyboard.insertText("朝に書いた自由な日記");
  await page.getByRole("button", { name: "編集を終える" }).click();
  await expect(page.getByLabel("今日の日記の本文")).toContainText("朝に書いた自由な日記");
  await page.getByRole("button", { name: "編集・追記" }).click();
  await editor(page).press("ControlOrMeta+End");
  await page.keyboard.press("Enter");
  await page.keyboard.insertText("夕方に追記したこと");
  await page.getByRole("button", { name: "編集を終える" }).click();
  await page.reload();
  await expect(page.getByLabel("今日の日記の本文")).toContainText("夕方に追記したこと");
  const daily = await (await page.request.get(`/api/v1/daily?date=${date}`)).json();
  expect(daily.content).toContain("朝に書いた自由な日記");
  expect(daily.content).not.toContain("## 日記");
});

test("入力直後の日付移動は保存を待ち、別の日記へ混ぜない", async ({ page }) => {
  const date = "2026-01-17";
  await createDiary(page, date);
  await page.route("**/api/v1/files/daily/**", async route => {
    if (route.request().method() === "PUT") await new Promise(resolve => setTimeout(resolve, 500));
    await route.continue();
  });
  await editor(page).press("ControlOrMeta+End");
  await page.keyboard.press("Enter");
  await page.keyboard.insertText("移動する前の日の入力");
  await page.getByRole("button", { name: "前日", exact: true }).click();
  await expect(page.getByLabel("日記の日付", { exact: true })).toHaveValue("2026-01-16");
  const previous = await (await page.request.get(`/api/v1/daily?date=${date}`)).json();
  expect(previous.content).toContain("移動する前の日の入力");
  const next = await (await page.request.get("/api/v1/daily?date=2026-01-16")).json();
  expect(next.content).not.toContain("移動する前の日の入力");
});

test("日記の取得失敗を未記入と区別する", async ({ page }) => {
  await page.route("**/api/v1/files/daily/**", route => route.fulfill({ status: 500, json: { error: "読取失敗" } }));
  await page.goto("/");
  await expect(page.getByText("HTTP 500: 読取失敗", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "日記を書く" })).toHaveCount(0);
});

test("保存競合では入力を残し、日付を勝手に切り替えない", async ({ page }) => {
  await createDiary(page, "2026-01-18");
  await page.route("**/api/v1/files/daily/**", async route => {
    if (route.request().method() === "PUT") await route.fulfill({ status: 409, json: { error: "別の場所で編集済み" } });
    else await route.continue();
  });
  await editor(page).press("ControlOrMeta+End");
  await page.keyboard.press("Enter");
  await page.keyboard.insertText("消したくない入力");
  await page.getByRole("button", { name: "前日", exact: true }).click();
  await expect(page.getByLabel("保存できていない入力")).toContainText("消したくない入力");
  await expect(page.getByLabel("日記の日付", { exact: true })).toHaveValue("2026-01-18");
});

test("プロジェクトを選んでも日記の日付を保持し、同じホームで詳細を編集できる", async ({ page }) => {
  await page.goto("/?date=2026-01-16&panel=projects");
  await page.locator("#projects").getByRole("button", { name: /today-demo/ }).click();
  await expect(page).toHaveURL(/date=2026-01-16/);
  await expect(page).toHaveURL(/project=today-demo/);
  await expect(page.getByText("今日の一歩を確かめる", { exact: true }).first()).toBeVisible();
  await page.locator("#projects").locator("summary").first().click();
  await expect(page.locator("#projects")).not.toHaveAttribute("open", "");
  await page.locator("#projects").locator("summary").first().click();
  await expect(page.getByText("今日の一歩を確かめる", { exact: true }).first()).toBeVisible();
});

test("タスクをホーム内で完了・取消できる", async ({ page }) => {
  await page.goto("/tasks");
  await expect(page).toHaveURL(/\/\?panel=tasks#tasks$/);
  const task = page.getByRole("checkbox", { name: "履歴を残すテスト を完了にする", exact: true });
  await task.click();
  await expect(task).toHaveCount(0);
  await page.getByRole("button", { name: /完了済み/ }).first().click();
  const undo = page.getByRole("checkbox", { name: "履歴を残すテスト を未完了に戻す", exact: true });
  await undo.click();
  await expect(task).toBeVisible();

});

test("やったことと習慣をホームから記録し、再読込で保持する", async ({ page }) => {
  await page.goto("/today");
  await expect(page).toHaveURL(/\/\?panel=activity#activity$/);
  await page.getByLabel("プロジェクト", { exact: true }).selectOption("today-demo");
  const input = page.getByLabel("やったこと（1 行）");
  await input.fill("ホームで記録した実績");
  await input.press("Enter");
  await expect(page.getByText("やったことを残しました", { exact: true })).toBeVisible();
  const routine = page.getByRole("checkbox", { name: /テスト読書/ });
  await routine.check();
  await page.reload();
  await expect(routine).toBeChecked();
  await routine.uncheck();
  await expect(page.getByText(/今日の予定が/)).toHaveCount(0);
});

test("古いプロジェクトと日記のURLは対応するホームへ移動する", async ({ page }) => {
  await page.goto("/projects?name=today-demo");
  await expect(page).toHaveURL(/project=today-demo/);
  await expect(page.getByText("今日の一歩を確かめる", { exact: true }).first()).toBeVisible();
  await page.goto("/daily?date=2026-01-16");
  await expect(page).toHaveURL(/\/\?date=2026-01-16#diary$/);
  await expect(page.getByLabel("今日の日記の本文")).toContainText("朝に書いた自由な日記");
});

for (const mode of ["light", "dark"] as const) {
  test(`${mode}: ホームと展開した詳細はスマホ幅に収まる`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/?date=2026-01-16&panel=projects&project=today-demo");
    await page.evaluate(dark => document.documentElement.classList.toggle("dark", dark), mode === "dark");
    await expect(page.getByText("今日の一歩を確かめる", { exact: true }).first()).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
    await page.screenshot({ path: `../../../temp/kensan-home-${mode}.png`, fullPage: true });
  });
}

test("日記入力中にプロジェクトを選んでもドラフトを保持する", async ({ page }) => {
  await createDiary(page, "2026-01-19");
  await editor(page).press("ControlOrMeta+End");
  await page.keyboard.press("Enter");
  await page.keyboard.insertText("詳細を開いても保持する本文");
  await page.locator("#projects > summary").click();
  await page.locator("#projects").getByRole("button", { name: /today-demo/ }).click();
  await expect(editor(page)).toContainText("詳細を開いても保持する本文");
  await expect(page.getByLabel("日記の日付", { exact: true })).toHaveValue("2026-01-19");
});


test("日記を開いて読むだけではファイルを書き換えない", async ({ page }) => {
  const date = "2026-01-16";
  const before = await (await page.request.get(`/api/v1/daily?date=${date}`)).json();
  let writes = 0;
  page.on("request", request => { if (request.method() === "PUT" && request.url().includes("/files/daily/")) writes++; });
  await page.goto(`/?date=${date}`);
  await page.getByRole("button", { name: "編集・追記", exact: true }).click();
  await expect(editor(page)).toBeVisible();
  await page.getByRole("button", { name: "編集を終える", exact: true }).click();
  await page.getByRole("button", { name: "前日", exact: true }).click();
  const after = await (await page.request.get(`/api/v1/daily?date=${date}`)).json();
  expect(writes).toBe(0);
  expect(after.content).toBe(before.content);
  expect(after.doc.mtime).toBe(before.doc.mtime);
});

test("競合から最新を読み込んだ後も元の入力を回収できる", async ({ page }) => {
  await createDiary(page, "2026-01-20");
  await page.route("**/api/v1/files/daily/**", async route => {
    if (route.request().method() === "PUT") await route.fulfill({ status: 409, json: { error: "conflict" } });
    else await route.continue();
  });
  await editor(page).press("ControlOrMeta+End");
  await page.keyboard.press("Enter");
  await page.keyboard.insertText("回収する本文");
  await page.getByRole("button", { name: "編集を終える", exact: true }).click();
  await expect(page.getByLabel("保存できていない入力")).toContainText("回収する本文");
  await page.getByRole("button", { name: "入力を控えて最新の内容を読み込む" }).click();
  await expect(page.getByLabel("読み込み前の入力")).toContainText("回収する本文");
  await page.reload();
  await expect(page.getByLabel("読み込み前の入力")).toContainText("回収する本文");
});


test("同じプロジェクトのバッジから閉じた詳細を開き直せる", async ({ page }) => {
  await page.goto("/?panel=projects&project=today-demo");
  await expect(page.getByText("今日の一歩を確かめる", { exact: true }).first()).toBeVisible();
  await page.locator("#projects > summary").click();
  await page.locator("#tasks > summary").click();
  const badge = page.locator("#tasks").getByRole("link", { name: "today-demo", exact: true }).first();
  await badge.click();
  await expect(page.getByText("今日の一歩を確かめる", { exact: true }).first()).toBeVisible();
  await page.locator("#projects > summary").click();
  await badge.click();
  await expect(page.getByText("今日の一歩を確かめる", { exact: true }).first()).toBeVisible();
});

test("保存に失敗したプロジェクトの自由メモは戻ったエディタにも残る", async ({ page }) => {
  await page.goto("/?panel=projects&project=today-demo");
  const projects = page.locator("#projects");
  const memo = projects.locator('[contenteditable="true"]');
  await expect(memo).toBeVisible();
  await page.route("**/api/v1/files/projects/**", async route => {
    if (route.request().method() === "PUT") {
      await new Promise(resolve => setTimeout(resolve, 300));
      await route.fulfill({ status: 500, json: { error: "一時的な保存失敗" } });
    } else await route.continue();
  });
  await memo.fill("残しておきたいメモ");
  await page.getByRole("link", { name: "メモ", exact: true }).click();
  await page.getByRole("link", { name: "ダッシュボード", exact: true }).click();
  await page.locator("#projects > summary").click();
  await projects.getByRole("button", { name: /today-demo/ }).click();
  await expect(memo).toContainText("残しておきたいメモ");
  await expect(projects.getByLabel("保存できていない入力")).toContainText("残しておきたいメモ");
});
