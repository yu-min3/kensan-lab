import { expect, test } from "@playwright/test";

const editor = (page: import("@playwright/test").Page) => page.getByLabel("日記を編集").locator('[contenteditable="true"]');

async function createDiary(page: import("@playwright/test").Page, date: string) {
  await page.goto(`/daily?date=${date}`);
  const write = page.getByRole("button", { name: "日記を書く", exact: true });
  if (await write.isVisible()) await write.click();
  else {
    await expect(page.getByRole("heading", { name: `${date} の日記` })).toBeVisible();
    if (await write.isVisible()) await write.click();
    else await page.getByRole("button", { name: "編集・追記", exact: true }).click();
  }
  await expect(editor(page)).toBeVisible();
}

test("日記画面で日記の作成・編集・追記・再読込を完結できる", async ({ page }) => {
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









for (const mode of ["light", "dark"] as const) {
  test(`${mode}: 日記と元のプロジェクトカードはスマホ幅に収まる`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/?date=2026-01-16");
    await page.evaluate(dark => document.documentElement.classList.toggle("dark", dark), mode === "dark");
    await expect(page.getByText("E2E で目指すことを 1 行で見せる", { exact: true }).first()).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
    await page.screenshot({ path: `../../../temp/kensan-home-${mode}.png`, fullPage: true });
  });
}




test("日記を開いて読むだけではファイルを書き換えない", async ({ page }) => {
  const date = "2026-01-16";
  const before = await (await page.request.get(`/api/v1/daily?date=${date}`)).json();
  let writes = 0;
  page.on("request", request => { if (request.method() === "PUT" && request.url().includes("/files/daily/")) writes++; });
  await page.goto(`/daily?date=${date}`);
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







test("元のプロジェクトカードがあり、管理機能はロードされない", async ({ page }) => {
  const management: string[] = [];
  page.on("request", request => {
    if (/\/api\/v1\/(today|tasks|projects)(?:[/?]|$)/.test(request.url())) management.push(request.url());
  });
  await page.goto("/");
  await expect(page.getByText("E2E で目指すことを 1 行で見せる", { exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "地図 · どこが埋まっているか" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "技術レーダー · 何を採り、何をやめたか" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "やったこと", exact: true })).toHaveCount(0);
  await expect(page.locator("#activity, #tasks, #projects")).toHaveCount(0);
  expect(management).toEqual([]);
});

test("旧管理URLから機能が復活せず、プロジェクト概要へ移動する", async ({ page }) => {
  for (const url of ["/today", "/tasks"]) {
    await page.goto(url);
    await expect(page).toHaveURL(/\/$/);
    await expect(page.getByText("E2E で目指すことを 1 行で見せる", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "新規", exact: true })).toHaveCount(0);
  }
  await page.goto("/projects?name=today-demo&date=2026-01-16");
  await expect(page).toHaveURL(/date=2026-01-16#project-today-demo$/);
  await expect(page.locator("#project-today-demo")).toBeInViewport();
  await page.goto("/daily?date=2026-01-16");
  await expect(page.getByLabel("今日の日記の本文")).toContainText("朝に書いた自由な日記");
});

test("日記からダッシュボードへ戻っても保存した内容が残る", async ({ page }) => {
  await createDiary(page, "2026-01-21");
  await editor(page).press("ControlOrMeta+End");
  await page.keyboard.press("Enter");
  await page.keyboard.insertText("個人的な考えごと");
  await page.getByRole("button", { name: "編集を終える" }).click();
  await page.getByRole("link", { name: "ダッシュボードへ戻る" }).click();
  await expect(page.getByText("個人的な考えごと")).toHaveCount(0);
  await page.goto("/daily?date=2026-01-21");
  await expect(page.getByLabel("今日の日記の本文")).toContainText("個人的な考えごと");
});


test("カードがないプロジェクトには空のリンクを出さず、旧URLはホームへ戻す", async ({ page }) => {
  await page.route("**/api/v1/portfolio", async route => {
    const view = await (await route.fetch()).json();
    await route.fulfill({ json: { ...view, compass: { ...view.compass,
      allocs: [...view.compass.allocs, { ...view.compass.allocs[0], project: "not-on-dashboard" }],
    } } });
  });
  await page.goto("/");
  await expect(page.getByText("not-on-dashboard", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "not-on-dashboard", exact: true })).toHaveCount(0);
  await page.goto("/projects?name=not-on-dashboard");
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("heading", { name: "地図 · どこが埋まっているか" })).toBeVisible();
});


test("1行追加で日記を作成し、本文編集後にも自由な本文へ追記できる", async ({ page }) => {
  await page.clock.install({ time: new Date("2026-10-21T00:05:00Z") });
  const date = "2026-01-10";
  await page.goto(`/daily?date=${date}`);
  await page.getByLabel("日記（1 行）", { exact: true }).fill("朝は散歩に出たい");
  await page.getByRole("button", { name: "残す", exact: true }).click();
  await expect(page.getByLabel("今日の日記の本文")).toContainText("朝は散歩に出たい");
  await page.getByRole("button", { name: "編集・追記", exact: true }).click();
  await editor(page).press("ControlOrMeta+End");
  await page.keyboard.press("Enter");
  await page.keyboard.insertText("本文からも書ける");
  await page.getByRole("button", { name: "編集を終える" }).click();
  await page.getByLabel("日記（1 行）", { exact: true }).fill("夕方は読書した");
  await page.getByLabel("日記（1 行）", { exact: true }).press("Enter");
  await expect(page.getByLabel("今日の日記の本文")).toContainText("夕方は読書した");
  await page.reload();
  const daily = await (await page.request.get(`/api/v1/daily?date=${date}`)).json();
  expect(daily.content).toContain("09:05 朝は散歩に出たい");
  expect(daily.content).toContain("本文からも書ける");
  expect(daily.content).toContain("09:05 夕方は読書した");
  expect(daily.content).not.toContain("## 日記");
});

test("1行追加の保存失敗では本文ドラフトに入力が残る", async ({ page }) => {
  await createDiary(page, "2026-01-09");
  await page.getByRole("button", { name: "編集を終える" }).click();
  await page.route("**/api/v1/files/daily/**", async route => {
    if (route.request().method() === "PUT") await route.fulfill({ status: 409, json: { error: "別の場所で編集済み" } });
    else await route.continue();
  });
  await page.getByLabel("日記（1 行）", { exact: true }).fill("消さずに残したい一行");
  await page.getByRole("button", { name: "残す", exact: true }).click();
  await expect(page.getByLabel("保存できていない入力")).toContainText("消さずに残したい一行");
  await page.reload();
  await expect(page.getByLabel("保存できていない入力")).toContainText("消さずに残したい一行");
});


test("未作成日への1行追加が遅延して失敗しても、別画面から戻ると入力を回収できる", async ({ page }) => {
  const date = "2026-01-08";
  await page.goto(`/daily?date=${date}`);
  let release!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  await page.route("**/api/v1/files", async route => {
    if (route.request().method() === "POST") {
      await held;
      await route.fulfill({ status: 500, json: { error: "作成失敗" } }).catch(() => {});
    } else await route.continue();
  });
  await page.getByLabel("日記（1 行）", { exact: true }).fill("失敗しても残す一行");
  const request = page.waitForRequest(req => req.method() === "POST" && req.url().endsWith("/api/v1/files"));
  await page.getByRole("button", { name: "残す", exact: true }).click();
  await request;
  await page.getByRole("link", { name: "メモ", exact: true }).click();
  release();
  await page.goto(`/daily?date=${date}`);
  await expect(page.getByLabel("日記（1 行）", { exact: true })).toHaveValue("失敗しても残す一行");
  await page.reload();
  await expect(page.getByLabel("日記（1 行）", { exact: true })).toHaveValue("失敗しても残す一行");
});


test("空行を含む箇条書きも項目内の段落余白を重ねない", async ({ page }) => {
  await page.route("**/api/v1/files/daily/**", route => route.fulfill({ json: {
    doc: { path: "daily/2026/01/07.md", mtime: "2026-01-07T00:00:00Z" },
    content: "# 2026-01-07\n\n- 一つ目\n\n- 二つ目\n\n  補足の段落\n\n  - 子の項目\n\n通常の段落\n",
  } }));
  await page.goto("/daily?date=2026-01-07");
  const body = page.getByLabel("今日の日記の本文");
  const paragraphs = body.locator("li > p");
  await expect(paragraphs.first()).toBeVisible();
  for (const paragraph of await paragraphs.all()) {
    await expect(paragraph).toHaveCSS("margin-top", "0px");
    await expect(paragraph).toHaveCSS("margin-bottom", "0px");
  }
  await expect(body.getByText("通常の段落", { exact: true })).toHaveCSS("margin-top", "12px");
  await expect(body.locator("li ul")).toHaveCSS("margin-top", "4px");
});


test("常時表示は明示した欄だけを表示し、保存で日記を保持する", async ({ page }) => {
  const original = "---\ntype: daily\n---\n# 今日\n\n見せない前文\n\n## 今日やりたいこと\n\n- 散歩する\n\n## 日記\n\n個人的な感想\n";
  let content = original;
  let writes = 0;
  await page.route("**/api/v1/files/daily/**", async route => {
    if (route.request().method() === "PUT") {
      writes++;
      const body = route.request().postDataJSON();
      content = body.content;
      await route.fulfill({ json: { doc: { path: "daily/test.md", mtime: "saved" } } });
    } else await route.fulfill({ json: { doc: { path: "daily/test.md", mtime: "initial" }, content } });
  });
  await page.goto("/");
  await expect(page.getByLabel("今日やりたいことの本文")).toContainText("散歩する");
  await expect(page.getByText(/見せない前文|個人的な感想/)).toHaveCount(0);
  await page.getByRole("button", { name: "今日やりたいことを編集" }).click();
  await page.getByLabel("今日やりたいこと（ダッシュボードに表示）").fill("- 図書館へ行く");
  await page.getByRole("button", { name: "保存する" }).click();
  await expect(page.getByLabel("今日やりたいことの本文")).toContainText("図書館へ行く");
  expect(content).toBe(original.replace("- 散歩する", "- 図書館へ行く"));
  expect(writes).toBe(1);
  await page.getByRole("link", { name: "日記を開く" }).click();
  await expect(page.getByLabel("今日の日記の本文")).toContainText("個人的な感想");
});

test("表示欄のない日やコード例だけの日は本文を出さず、取消でも保存しない", async ({ page }) => {
  let writes = 0;
  await page.route("**/api/v1/files/daily/**", async route => {
    if (route.request().method() === "PUT") writes++;
    await route.fulfill({ json: { doc: { path: "daily/test.md", mtime: "initial" },
      content: "# 今日\n\n秘密の日記\n\n```md\n## 今日やりたいこと\nサンプル\n```\n" } });
  });
  await page.goto("/");
  await expect(page.getByRole("button", { name: "今日やりたいことを書く" })).toBeVisible();
  await expect(page.getByText(/秘密の日記|サンプル/)).toHaveCount(0);
  await page.getByRole("button", { name: "今日やりたいことを書く" }).click();
  await page.getByLabel("今日やりたいこと（ダッシュボードに表示）").fill("取消する入力");
  await page.getByRole("button", { name: "キャンセル" }).click();
  expect(writes).toBe(0);
});

test("ホームの欄はJSTの日付またぎで今日へ移る", async ({ page }) => {
  await page.clock.install({ time: new Date("2026-10-28T14:59:50Z") });
  await page.route("**/api/v1/files/daily/**", async route => {
    const tomorrow = route.request().url().includes("/29.md");
    await route.fulfill({ json: { doc: { path: "daily/test.md", mtime: "initial" },
      content: `## 今日やりたいこと\n\n${tomorrow ? "今日の予定" : "昨日の予定"}\n\n## 日記\n秘密` } });
  });
  await page.goto("/");
  await expect(page.getByLabel("今日やりたいことの本文")).toContainText("昨日の予定");
  await page.clock.fastForward(35_000);
  await expect(page.getByLabel("今日やりたいことの本文")).toContainText("今日の予定");
  await expect(page.getByText("秘密", { exact: true })).toHaveCount(0);
});


for (const mode of ["light", "dark"] as const) {
  test(`${mode}: 今日の欄と整備枠をPCとスマホで表示する`, async ({ page }) => {
    await page.route("**/api/v1/files/daily/**", route => route.fulfill({ json: {
      doc: { path: "daily/test.md", mtime: "initial" }, content: "## 今日やりたいこと\n\n- 公園へ行く\n- 作り置きをする\n\n## 日記\n個人的な考えごと",
    } }));
    await page.route("**/api/v1/portfolio", async route => {
      const view = await (await route.fetch()).json();
      await route.fulfill({ json: { ...view, compass: { ...view.compass, support: [{ project: "kensan-workspace", count: 12, target: 0, actual: 0, weeks: [0, 3, 4, 5], aims: [] }] } } });
    });
    await page.goto("/");
    await page.evaluate(dark => document.documentElement.classList.toggle("dark", dark), mode === "dark");
    await expect(page.getByRole("heading", { name: "活動を支える整備" })).toBeVisible();
    await expect(page.getByText("個人的な考えごと")).toHaveCount(0);
    for (const width of [1440, 390]) {
      await page.setViewportSize({ width, height: 1000 });
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
      await page.screenshot({ path: `../../../temp/kensan-display-${mode}-${width}.png`, fullPage: true });
    }
  });
}
