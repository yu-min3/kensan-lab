import { expect, test } from "@playwright/test";

test("目標内の完了・取消と習慣をAPIへ保存し、再読込後も保持する", async ({ page }) => {
  await page.goto("/");
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
  await page.goto("/");
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
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "今日の一歩を確かめる" })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  });
}
