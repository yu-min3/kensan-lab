import { expect, test } from "@playwright/test";
import { spawn, execFileSync, ChildProcess } from "node:child_process";
import { randomBytes } from "node:crypto";
import { mkdtempSync, writeFileSync, chmodSync, rmSync } from "node:fs";
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
}
