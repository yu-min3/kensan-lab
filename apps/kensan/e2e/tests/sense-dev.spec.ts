import { expect, test } from "@playwright/test";
import { spawn, execFileSync, ChildProcess } from "node:child_process";
import { randomBytes } from "node:crypto";
import { mkdtempSync, writeFileSync, readFileSync, chmodSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { createServer } from "node:net";

const appRoot = resolve(__dirname, "../../../sense-dev");
const token = randomBytes(32).toString("hex");
let tempDir: string;
let processHandle: ChildProcess;
let baseURL: string;

async function freePort(): Promise<number> {
  const server = createServer();
  return new Promise((resolvePort, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const addr = server.address();
      if (!addr || typeof addr === "string") return reject(new Error("no port"));
      server.close(() => resolvePort(addr.port));
    });
  });
}

test.beforeAll(async () => {
  tempDir = mkdtempSync(join(tmpdir(), "sense-dev-e2e-"));
  chmodSync(tempDir, 0o700);
  const tokenFile = join(tempDir, "admin-token");
  writeFileSync(tokenFile, token, { mode: 0o600 });
  execFileSync("go", ["run", "./e2e/seed-mobile.go", "-data", join(tempDir, "state")], { cwd: appRoot });
  const binary = join(tempDir, "sense-dev");
  execFileSync("go", ["build", "-o", binary, "./cmd/sense-dev"], { cwd: appRoot });
  const port = await freePort();
  baseURL = `http://127.0.0.1:${port}`;
  processHandle = spawn(binary, [
    "-listen", `127.0.0.1:${port}`,
    "-data", join(tempDir, "state"),
    "-admin-token-file", tokenFile,
    "-tokens-css", resolve(appRoot, "../../packages/design-tokens/tokens.css"),
    "-mock-worker",
  ], { cwd: appRoot, stdio: "ignore" });
  for (let i = 0; i < 100; i++) {
    if (processHandle.exitCode !== null) throw new Error("sense-dev exited before browser test");
    try {
      const response = await fetch(`${baseURL}/login`);
      if (response.ok) return;
    } catch { /* waiting for loopback listener */ }
    await new Promise((done) => setTimeout(done, 100));
  }
  throw new Error("sense-dev did not become ready");
});

test.afterAll(async () => {
  if (processHandle?.exitCode === null) {
    const stopped = new Promise<void>((done) => processHandle.once("exit", () => done()));
    processHandle.kill("SIGTERM");
    await Promise.race([stopped, new Promise<void>((done) => setTimeout(done, 5000))]);
  }
  if (tempDir) rmSync(tempDir, { recursive: true, force: true });
});

for (const width of [360, 390, 430]) {
  test(`${width}px の案件詳細は拡大・再接続後も操作できる`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 844 });
    await page.goto(`${baseURL}/login`);
    await page.emulateMedia({ colorScheme: "dark" });
    await expect.poll(() => page.locator("html").getAttribute("class")).toContain("dark");
    await page.emulateMedia({ colorScheme: "light" });
    await page.getByLabel("管理トークン").fill(token);
    await page.getByRole("button", { name: "開く" }).click();
    await page.getByRole("heading", { name: "Golden Path 契約の修正" }).getByRole("link").click();
    await expect(page.getByRole("heading", { name: "会話と成果物" })).toBeVisible();
    await expect(page.getByText("acceptance_failed", { exact: true })).toBeVisible();
    await expect(page.getByText("acceptance_passed", { exact: true })).toBeVisible();
    const link = page.getByRole("link", { name: "開発の現在地へ戻る" });
    const linkBox = await link.boundingBox();
    expect(linkBox?.height).toBeGreaterThanOrEqual(44);
    for (const button of await page.getByRole("button").all()) {
      if (!(await button.isVisible())) continue;
      expect((await button.boundingBox())?.height).toBeGreaterThanOrEqual(44);
    }
    await page.emulateMedia({ colorScheme: "dark" });
    await expect.poll(() => page.locator("html").getAttribute("class")).toContain("dark");
    const darkBackground = await page.locator("body").evaluate((body) => getComputedStyle(body).backgroundColor);
    await page.emulateMedia({ colorScheme: "light" });
    await expect.poll(() => page.locator("html").getAttribute("class")).not.toContain("dark");
    const lightBackground = await page.locator("body").evaluate((body) => getComputedStyle(body).backgroundColor);
    expect(darkBackground).not.toBe(lightBackground);
    await page.evaluate(() => { document.body.style.fontSize = "32px"; });
    const enlarged = await page.evaluate(() => ({ page: document.documentElement.scrollWidth, viewport: window.innerWidth }));
    const overflow = await page.evaluate(() => [...document.querySelectorAll('body *')]
      .filter((element) => element.getBoundingClientRect().right > window.innerWidth + 1)
      .slice(0, 8).map((element) => ({ tag: element.tagName, className: element.className, text: element.textContent?.trim().slice(0, 40) })));
    expect(enlarged.page, JSON.stringify(overflow)).toBeLessThanOrEqual(enlarged.viewport);
    await page.screenshot({ path: testInfo.outputPath("detail-200-percent.png"), fullPage: true });
    await page.evaluate(() => { document.body.style.fontSize = ""; });
    await page.context().setOffline(true);
    await page.reload().catch(() => {});
    await page.context().setOffline(false);
    await page.reload();
    await expect(page.getByRole("heading", { name: "会話と成果物" })).toBeVisible();
    await link.focus();
    const outline = await link.evaluate((element) => getComputedStyle(element).outlineStyle);
    expect(outline).not.toBe("none");
  });

  test(`${width}px で依頼・停止・日報プレビューが横にはみ出さない`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 844 });
    await page.goto(`${baseURL}/login`);
    await page.getByLabel("管理トークン").fill(token);
    await page.getByRole("button", { name: "開く" }).click();
    await expect(page.getByRole("heading", { name: "開発の現在地" })).toBeVisible();
    const title = `画面幅 ${width} の模擬案件`;
    await page.getByLabel("依頼内容").fill(title);
    await page.getByRole("button", { name: "依頼を登録" }).click();
    await expect(page.getByText(title)).toBeVisible();
    await page.getByRole("button", { name: "Mac 優先を2時間" }).click();
    await expect(page.getByText(/Mac 優先：/)).toBeVisible();
    await page.getByRole("button", { name: "Mac 優先を解除" }).click();
    await page.getByRole("button", { name: "新規実行を停止" }).click();
    await expect(page.getByText("新規実行を停止中")).toBeVisible();
    await page.getByRole("button", { name: "新規実行を再開" }).click();
    await page.getByRole("button", { name: "今日のプレビューを保存" }).click();
    await expect(page.getByText(/not_configured/)).toBeVisible();
    const sizes = await page.evaluate(() => ({ page: document.documentElement.scrollWidth, viewport: window.innerWidth }));
    expect(sizes.page).toBeLessThanOrEqual(sizes.viewport);
    await page.screenshot({ path: testInfo.outputPath("mobile.png"), fullPage: true });
  });

  test(`${width}px で team と成果物の交換を区別して追える`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 844 });
    await page.goto(`${baseURL}/login`);
    await page.getByLabel("管理トークン").fill(token);
    await page.getByRole("button", { name: "開く" }).click();
    const exchange = page.locator("section").filter({ has: page.getByRole("heading", { name: "成果物の交換" }) });
    await expect(exchange.getByText("acceptance_failed", { exact: true })).toBeVisible();
    await expect(exchange.getByText("acceptance_passed", { exact: true })).toBeVisible();
    await expect(exchange.getByText("Platform → App", { exact: true }).first()).toBeVisible();
    await expect(exchange.getByText("App → Platform", { exact: true }).first()).toBeVisible();
    const corrected = exchange.locator("article").filter({ hasText: "corrected_contract" });
    await corrected.getByText("成果物の種類・版・SHA-256").click();
    await expect(corrected.getByText("corrected_contract · v1")).toBeVisible();
    await expect(corrected.getByText(/SHA-256 [a-f0-9]{64}/)).toBeVisible();
    await expect(exchange.getByText("fixture corrected contract")).toHaveCount(0);
    await expect(corrected.locator('a[href^="#task-"]')).toHaveCount(2);
    await expect(corrected.getByRole("link", { name: "Golden Path 契約の修正" })).toBeVisible();
    await expect(corrected.getByRole("link", { name: "既存 App の受入再試験" })).toBeVisible();
    await expect(corrected.locator('a[href^="#message-"]')).toHaveCount(1);
    const sizes = await page.evaluate(() => ({ page: document.documentElement.scrollWidth, viewport: window.innerWidth }));
    expect(sizes.page).toBeLessThanOrEqual(sizes.viewport);
    if (width === 390) await corrected.screenshot({ path: testInfo.outputPath("exchange-card.png") });
  });

  test(`${width}px で質問への回答と操作判断を記録できる`, async ({ page }) => {
    await page.setViewportSize({ width, height: 844 });
    await page.goto(`${baseURL}/login`);
    await page.getByLabel("管理トークン").fill(token);
    await page.getByRole("button", { name: "開く" }).click();
    const prompt = `画面幅 ${width} の契約確認`;
    const question = page.locator("article").filter({ hasText: prompt });
    await page.context().setOffline(true);
    await question.getByLabel("回答").fill(`回答 ${width}`);
    await expect(question.getByLabel("回答")).toHaveValue(`回答 ${width}`);
    await page.context().setOffline(false);
    await page.reload();
    const restored = page.locator("article").filter({ hasText: prompt });
    await expect(restored.getByLabel("回答")).toHaveValue(`回答 ${width}`);
    await restored.getByRole("button", { name: "回答を送る" }).click();
    await expect(page.locator("article").filter({ hasText: prompt }).getByText(`回答済み：回答 ${width}`)).toBeVisible();
    const approval = page.locator("article").filter({ hasText: `画面幅 ${width} の判断（模擬）` });
    await approval.getByRole("button", { name: "承認を記録" }).click();
    await expect(page.locator("article").filter({ hasText: `画面幅 ${width} の判断（模擬）` }).getByText("approved")).toBeVisible();
    const state = JSON.parse(readFileSync(join(tempDir, "state", "state.json"), "utf8"));
    expect(Object.keys(state.intents)).toHaveLength(0);
    const sizes = await page.evaluate(() => ({ page: document.documentElement.scrollWidth, viewport: window.innerWidth }));
    expect(sizes.page).toBeLessThanOrEqual(sizes.viewport);
  });
}

test("390px の模擬依頼から判断・停止・日報復帰まで一巡する", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`${baseURL}/login`);
  await page.getByLabel("管理トークン").fill(token);
  await page.getByRole("button", { name: "開く" }).click();
  await page.getByLabel("依頼内容").fill("模擬 canary の確認");
  await page.getByRole("button", { name: "依頼を登録" }).click();
  await expect(page.getByRole("heading", { name: "模擬 canary の確認" })).toBeVisible();

  const question = page.locator("article").filter({ hasText: "一巡確認の契約質問" });
  await question.getByLabel("回答").fill("契約版を確認済み");
  await question.getByRole("button", { name: "回答を送る" }).click();
  await expect(page.locator("article").filter({ hasText: "一巡確認の契約質問" }).getByText(/回答済み/)).toBeVisible();

  await page.getByRole("heading", { name: "Golden Path 契約の修正" }).getByRole("link").click();
  await expect(page.getByText("acceptance_failed", { exact: true })).toBeVisible();
  await expect(page.getByText("acceptance_passed", { exact: true })).toBeVisible();
  await page.getByText("成果物 1 件を確認").first().click();
  await expect(page.getByText(/SHA-256 [a-f0-9]{64}/).first()).toBeVisible();
  await page.getByRole("link", { name: "開発の現在地へ戻る" }).click();

  const approval = page.locator("article").filter({ hasText: "一巡確認 の判断（模擬）" });
  const action = await approval.locator("form.decision-form").getAttribute("action");
  const stale = await page.context().request.post(baseURL + action, { form: {
    csrf: await approval.locator('input[name="csrf"]').inputValue(),
    action_id: await approval.locator('input[name="action_id"]').inputValue(),
    operation: await approval.locator('input[name="operation"]').inputValue(),
    sha: "stale-sha", verdict: "approved",
  } });
  expect(stale.status()).toBe(409);
  await expect(approval.getByRole("button", { name: "承認を記録" })).toBeVisible();
  await approval.getByRole("button", { name: "承認を記録" }).click();
  await expect(page.locator("article").filter({ hasText: "一巡確認 の判断（模擬）" }).getByText("approved")).toBeVisible();
  const returned = page.locator("article").filter({ hasText: "一巡差し戻し の判断（模擬）" });
  await returned.getByRole("button", { name: "却下" }).click();
  await expect(page.locator("article").filter({ hasText: "一巡差し戻し の判断（模擬）" }).getByText("denied")).toBeVisible();

  await page.getByRole("button", { name: "Mac 優先を2時間" }).click();
  await expect(page.getByText(/Mac 優先：/)).toBeVisible();
  await page.getByRole("button", { name: "Mac 優先を解除" }).click();
  await page.getByRole("button", { name: "新規実行を停止" }).click();
  await expect(page.getByText("新規実行を停止中")).toBeVisible();
  await page.getByRole("button", { name: "新規実行を再開" }).click();
  const unknownReport = page.locator("#daily-2026-09-27");
  await expect(unknownReport.getByText("unknown")).toBeVisible();
  await expect(unknownReport.getByText(/送信結果が不明/)).toBeVisible();
  await unknownReport.getByRole("link").first().click();
  await expect(page).toHaveURL(/#task-/);
  await expect(page.locator(page.url().split("#")[1].replace(/^/, "#"))).toBeVisible();
});
