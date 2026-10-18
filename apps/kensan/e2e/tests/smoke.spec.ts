import { expect, test } from "@playwright/test";

// 単一 image（Go が dist を SPA fallback 配信）で主要導線が壊れていないことを確認する
// smoke E2E。cutover 後に唯一のアプリになった kensan の最低限の安全網。

test("ダッシュボードが表示され、フィクスチャの North Star が出る", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("E2E テスト用の North Star")).toBeVisible();
});

test("ホームには独立したタスク管理の入口がない", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("link", { name: "タスク", exact: true })).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "かんばん", exact: true })).toHaveCount(0);
});

test("日記ページで未作成の日を作成できる", async ({ page }) => {
  // フィクスチャに存在しない過去日を指定 → 空状態 → 作成
  await page.goto("/daily?date=2026-01-15");
  const createButton = page.getByRole("button", { name: "日記を書く" });
  await expect(createButton).toBeVisible();
  await createButton.click();
  // 作成後は空状態のボタンが消え、エディタ（日記セクション）に切り替わる
  await expect(createButton).toBeHidden();
});
