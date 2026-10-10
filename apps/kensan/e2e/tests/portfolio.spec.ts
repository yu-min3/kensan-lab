import { expect, test } from "@playwright/test";

// ダッシュボード（/）: 地図・羅針盤・年報と、証拠の候補の採用。
// 契約: kensan-workspace projects/kensan-workspace/docs/portfolio-dashboard.md

test("/ に地図と羅針盤が出て、マスの判定が @public から決まる", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "地図 · どこが埋まっているか" })).toBeVisible();
  await expect(page.getByRole("button", { name: "投資を決める × インフラ: 外から見える" })).toBeVisible();
  await expect(page.getByRole("button", { name: "技術を選ぶ × AI: 社内だけ" })).toBeVisible();
  await expect(page.getByRole("button", { name: "事業に効かせる × インフラ: 空き" })).toBeVisible();
  await page.getByRole("button", { name: "投資を決める × インフラ: 外から見える" }).click();
  await expect(page.getByText("E2E の公開済み証拠")).toHaveCount(2); // マスの見出しと詳細
  await expect(page.getByRole("heading", { name: "羅針盤 · 時間がどこへ向いているか" })).toBeVisible();
  await expect(page.getByRole("img", { name: /today-demo 実際 \d+% 狙い 60%/ })).toBeVisible();
});

test("証拠の候補を採用すると地図のマスに載り、候補から消える", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: /証拠の候補 · 1 件/ })).toBeVisible();
  await page.getByRole("button", { name: "採用" }).click();
  await expect(page.getByText("地図に載せました", { exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: /証拠の候補/ })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "事業に効かせる × AI: 社内だけ" })).toBeVisible();
  const file = await (await page.request.get("/api/v1/files/portfolio.md")).json();
  expect(file.content).toContain("## 事業に効かせる × AI\n\n- 2026-10-10 E2E で採用する候補 — 日記から拾った → 地図に載る @project(today-demo)");
});

test("年報タブで章が出て、今日画面は /today に移っている", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("tab", { name: /年報/ }).click();
  await expect(page.getByRole("heading", { name: "作って、決める" })).toBeVisible();
  await page.getByRole("link", { name: "今日", exact: true }).click();
  await expect(page).toHaveURL(/\/today$/);
  await expect(page.getByRole("heading", { name: "今日の一歩を確かめる" }).first()).toBeVisible();
});

for (const mode of ["light", "dark"] as const) {
  test(`${mode}: ダッシュボードはスマホ幅で横にはみ出さない`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/");
    await page.evaluate(dark => document.documentElement.classList.toggle("dark", dark), mode === "dark");
    await expect(page.getByRole("heading", { name: "地図 · どこが埋まっているか" })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  });
}
