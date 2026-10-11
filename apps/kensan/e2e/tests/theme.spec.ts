import { expect, test } from "@playwright/test";

// 明暗の切り替え: 既定は OS に合わせ、選んだものはこのブラウザに残る。

test("OS が暗ければ最初から暗く表示される", async ({ browser }) => {
  const ctx = await browser.newContext({ colorScheme: "dark" });
  const page = await ctx.newPage();
  await page.goto("/");
  await expect(page.locator("html")).toHaveClass(/dark/);
  await ctx.close();
});

test("暗い表示を選ぶと再読み込み後も残り、自動に戻すと OS（明）に従う", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await page.goto("/");
  await expect(page.locator("html")).not.toHaveClass(/dark/);
  await page.getByRole("radio", { name: "暗い表示" }).click();
  await expect(page.locator("html")).toHaveClass(/dark/);
  await page.reload();
  await expect(page.locator("html")).toHaveClass(/dark/);
  await expect(page.getByRole("radio", { name: "暗い表示" })).toHaveAttribute("aria-checked", "true");
  await page.getByRole("radio", { name: "自動（OS に合わせる）" }).click();
  await expect(page.locator("html")).not.toHaveClass(/dark/);
});
