import { expect, test } from "@playwright/test";

// AppShell のレイアウト崩壊（2026-07-27）の回帰テスト。
// 症状: サイドバーが表示されず、本文が左端 200px の帯に潰れて日本語が 1 文字ずつ折り返す。
// 原因: grid の暗黙 auto-placement 頼みで、aside がフローから外れると main が 1 列目に繰り上がる。
// spec: projects/kensan-lab/docs/kensan-appshell-layout-collapse-fix.md（kensan-workspace）

const DESKTOP = { width: 1280, height: 900 };
const SIDEBAR_WIDTH = 200;

async function boxOf(locatorPromise: Promise<{ x: number; width: number } | null>) {
  const box = await locatorPromise;
  if (!box) throw new Error("要素の bounding box が取得できない（非表示?）");
  return box;
}

test("デスクトップ幅では sidebar が見え、main が 2 列目に置かれる", async ({ page }) => {
  await page.setViewportSize(DESKTOP);
  await page.goto("/");

  const sidebar = page.getByTestId("app-sidebar");
  await expect(sidebar).toBeVisible();

  const main = await boxOf(page.locator("main").boundingBox());
  expect(main.x).toBeGreaterThanOrEqual(SIDEBAR_WIDTH);
  expect(main.width).toBeGreaterThan(DESKTOP.width / 2);
});

test("sidebar が消えても main は 2 列目に留まる", async ({ page }) => {
  await page.setViewportSize(DESKTOP);
  await page.goto("/");

  // 拡張機能の cosmetic filter などで aside だけが消えた状況を再現する。
  // 修正前はここで main が x=0 / width=136px まで潰れていた。
  await page.addStyleTag({
    content: '[data-testid="app-sidebar"] { display: none !important; }',
  });

  const main = await boxOf(page.locator("main").boundingBox());
  expect(main.x).toBeGreaterThanOrEqual(SIDEBAR_WIDTH);
  expect(main.width).toBeGreaterThan(DESKTOP.width / 2);
});

test("モバイル幅では main が全幅を使う", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");

  // < md では grid にならないので、明示配置（md:col-start-2）が漏れていないことを確認する
  const main = await boxOf(page.locator("main").boundingBox());
  expect(main.x).toBe(0);
  expect(main.width).toBe(390);
});
