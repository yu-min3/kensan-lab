// Real browser → Konro → separate runtime HTTP process. Only model inference is
// substituted by runtime's opt-in generic fixture (no paid calls or real data).
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdtemp, mkdir, writeFile, readFile } from "node:fs/promises";
import { createWriteStream } from "node:fs";
import net from "node:net";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "../frontend/node_modules/playwright/index.mjs";

const app = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const runtimeBinary = process.env.AGENT_RUNTIME_TEST_BINARY;
const konroBinary = process.env.KONRO_TEST_BINARY;
assert(runtimeBinary && konroBinary, "Set AGENT_RUNTIME_TEST_BINARY and KONRO_TEST_BINARY to independently built binaries");
await mkdir(path.join(app, "temp"), { recursive: true });
const artifacts = await mkdtemp(path.join(app, "temp", "import-e2e-"));
const state = path.join(artifacts, "runtime");
const data = path.join(artifacts, "recipes");
await mkdir(data);
const control = path.join(artifacts, "inference.json");
const token = "local-test-only-no-provider-credentials-12345";
const tokenFile = path.join(artifacts, "service-token");
await writeFile(tokenFile, token, { mode: 0o600 });
const freePort = () => new Promise((resolve) => { const s = net.createServer(); s.listen(0, "127.0.0.1", () => { const p = s.address().port; s.close(() => resolve(p)); }); });
const runtimePort = await freePort(), appPort = await freePort();
const runtimeURL = `http://127.0.0.1:${runtimePort}`, appURL = `http://127.0.0.1:${appPort}`;
const output = { assessment:"本文から材料と調理手順を読み取れました。分量を確認して保存してください。", title: "鮭の煮物", servings: "2人分", ingredients: ["鮭 2切れ", "水 100ml"], steps: ["水を沸かす", "鮭を10分煮る"] };
const fixture = async (mode, value = output) => writeFile(control, JSON.stringify({ mode, output: value }));
await fixture("success");
const launch = (binary, args, env, name) => {
  const log = createWriteStream(path.join(artifacts, `${name}.log`), { flags: "a" });
  const child = spawn(binary, args, { env: { ...process.env, ...env }, stdio: ["ignore", "pipe", "pipe"] });
  child.stdout.pipe(log); child.stderr.pipe(log); child.on("error", (e) => log.write(String(e)));
  return child;
};
const waitHTTP = async (url, headers = {}) => {
  for (let i = 0; i < 100; i++) { try { const r = await fetch(url, { headers }); if (r.status < 500) return; } catch {} await new Promise((r) => setTimeout(r, 100)); }
  throw new Error(`service did not start: ${url}`);
};
const startRuntime = async () => {
  const p = launch(runtimeBinary, ["-test.run=^TestHTTPProcess$"], {
    AGENT_RUNTIME_TEST_ADDR: `127.0.0.1:${runtimePort}`, AGENT_RUNTIME_TEST_STATE: state,
    AGENT_RUNTIME_TEST_CONTROL: control, AGENT_RUNTIME_TEST_TOKEN: token,
  }, "runtime");
  await waitHTTP(`${runtimeURL}/v1/jobs/missing`, { Authorization: `Bearer ${token}` }); return p;
};
const stop = async (p) => { if (p && p.exitCode === null && !p.signalCode) { const exited = new Promise((r) => p.once("exit", r)); p.kill("SIGTERM"); await exited; } };
let runtime, backend, browser;
const results = [];
try {
  runtime = await startRuntime();
  backend = launch(konroBinary, [], {
    KONRO_DATA_DIR: data, KONRO_STATIC_DIR: path.join(app, "frontend/dist"), KONRO_ADDR: `127.0.0.1:${appPort}`,
    KONRO_AGENT_URL: runtimeURL, KONRO_AGENT_TOKEN_FILE: tokenFile,
  }, "konro");
  await waitHTTP(`${appURL}/healthz`);
  browser = await chromium.launch({ headless: true, executablePath: process.env.CHROME_BINARY || undefined });
  const page = await browser.newPage({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 1 });
  const errors = []; page.on("pageerror", (e) => errors.push(String(e)));
  const button = (name) => page.getByRole("button", { name, exact: true });
  const source = "鮭の煮物 2人分\n材料 鮭2切れ 水100ml\n水を沸かす。鮭を10分煮る。";
  const create = async () => {
    await page.goto(appURL);
    await page.locator(".recipe-add-menu > summary").click();await page.getByLabel("レシピの追加方法").getByRole("button",{name:"URL・本文から取り込む",exact:true}).click();
    await page.getByRole("button", { name: /^(本文を貼り付ける|別のレシピを取り込む)$/ }).first().waitFor();
    // Every scenario starts a fresh import while older source files stay intact.
    if (await button("別のレシピを取り込む").count()) await button("別のレシピを取り込む").click();
    await button("本文を貼り付ける").click();
    await page.getByLabel("レシピの本文", { exact: true }).fill(source);
    await button("取り込む").click();
    await page.locator(".import-status").waitFor();
  };
  const waitText = async (text) => page.getByText(text, { exact: false }).first().waitFor({ timeout: 30000 });
  await create();
  await page.getByLabel("タイトル", { exact: true }).fill("手入力の鮭");
  await page.getByLabel("材料（1行に1項目）", { exact: true }).fill("鮭 2切れ\n水 100ml");
  await page.getByLabel("手順（1行に1つの手順）", { exact: true }).fill("水を沸かす\n10分煮る");
  await button("下書きを保存して戻る").click();
  await page.locator(".recipe-add-menu > summary").waitFor();
  await page.reload(); await page.locator(".recipe-add-menu > summary").click();await page.getByLabel("レシピの追加方法").getByRole("button",{name:"URL・本文から取り込む",exact:true}).click();
  await page.getByLabel("タイトル", { exact: true }).waitFor();
  assert.equal(await page.getByLabel("タイトル", { exact: true }).inputValue(), "手入力の鮭");
  assert.equal(await button("レシピとして保存").isEnabled(), false);
  await page.getByRole("checkbox").check(); await button("レシピとして保存").click();
  await button("1品でコンロを開始").click();
  await waitText("10分煮る");
  await page.screenshot({ path: path.join(artifacts, "cooking-mobile.png"), fullPage: true });
  console.log("passed step", results.length + 1); results.push("manual draft persists after reload; confirmed recipe enters existing cooking session");
  await page.evaluate(() => localStorage.clear());

  await create();
  assert.equal(await page.getByRole("combobox").count(), 0);
  assert.equal(await page.getByRole("button",{name:/AI/}).count(),0);

  await waitText("取り込めました");
  assert.equal(await page.getByLabel("タイトル", { exact: true }).inputValue(), output.title);
  await page.getByText(output.assessment,{exact:true}).waitFor();
  await page.getByText("取り込み元を確認", { exact: true }).click();
  assert.equal(await page.locator("pre").first().textContent(), source);
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
  await page.screenshot({ path: path.join(artifacts, "ai-draft-mobile.png"), fullPage: true });
  await page.getByRole("checkbox").check(); await button("レシピとして保存").click();
  await page.locator(".recipe-add-menu > summary").waitFor();
  console.log("passed step", results.length + 1); results.push("AI normalization over real HTTP; source comparison; confirmation save; no horizontal mobile overflow");

  await fixture("success",{...output,steps:[]});await create();
  await waitText("一部を取り込めませんでした");
  assert.equal(await page.getByLabel("手順（1行に1つの手順）",{exact:true}).inputValue(),"");
  assert((await page.getByLabel("材料（1行に1項目）",{exact:true}).inputValue()).includes("鮭 2切れ"));
  await page.getByRole("checkbox").check();assert.equal(await button("レシピとして保存").isEnabled(),false);
  console.log("passed step",results.length+1);results.push("partial AI result keeps known ingredients and blocks saving until missing steps are entered");

  await fixture("wait"); await create();
  await waitText("利用枠が空くのを待っています");
  await fixture("success"); await button("取り込みを再開").click(); await waitText("取り込めました");
  console.log("passed step", results.length + 1); results.push("quota waits visibly and retries explicitly");

  await fixture("success", { ...output, steps: ["200度で焼く"] });
  await create(); await waitText("原文にない数値を検出");
  assert.equal(await page.getByLabel("タイトル", { exact: true }).inputValue(), "");
  console.log("passed step", results.length + 1); results.push("new temperature rejected without replacing original draft");

  await fixture("success", { ...output, steps: "invalid" });
  await create(); await waitText("取り込みに失敗しました");
  await waitText("処理は停止しています");
  assert.equal(await button("取り込みを再試行").isEnabled(), true);
  await page.screenshot({ path: path.join(artifacts, "ai-failure-mobile.png"), fullPage: true });
  assert.equal(await page.getByLabel("タイトル", { exact: true }).inputValue(), "");
  await fixture("success"); await button("取り込みを再試行").click(); await waitText("取り込めました");
  console.log("passed step", results.length + 1); results.push("runtime output-schema failure is visibly stopped and explicitly retries");

  await fixture("delay"); await create();
  await waitText("この画面を閉じても処理は続きます"); await button("取り込みを中止").click(); await waitText("取り込みを中止しました");
  assert.equal(await page.getByLabel("タイトル", { exact: true }).isEnabled(), true);
  console.log("passed step", results.length + 1); results.push("running job cancellation unlocks editor");

  await create(); await waitText("この画面を閉じても処理は続きます");
  await stop(runtime); runtime = await startRuntime(); await waitText("抽出サービスとの接続が中断しました");
  console.log("passed step", results.length + 1); results.push("runtime restart reports interrupted, never success or automatic rerun");

  await create(); await waitText("この画面を閉じても処理は続きます");
  await stop(runtime); await waitText("取り込みの状態を確認できません");
  assert.equal(await page.locator(".import-spinner").count(), 0);
  await button("手入力に切り替える").click(); await waitText("後から届く抽出結果で上書きされることはありません");
  await page.getByLabel("タイトル", { exact: true }).fill("接続なしでも編集");
  assert.equal(await page.getByLabel("タイトル", { exact: true }).isEnabled(), true);
  console.log("passed step", results.length + 1); results.push("unavailable runtime can be detached locally for manual editing");
  assert.deepEqual(errors, []);
  await writeFile(path.join(artifacts, "results.json"), JSON.stringify({ status: "passed", results, browserErrors: errors }, null, 2));
  console.log(JSON.stringify({ status: "passed", checks: results.length, artifacts }, null, 2));
} catch (e) {
  await writeFile(path.join(artifacts, "failure.txt"), String(e.stack || e));
  console.error(`Artifacts: ${artifacts}`); throw e;
} finally { await browser?.close(); await stop(backend); await stop(runtime); }
