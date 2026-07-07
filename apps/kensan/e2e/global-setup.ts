import { execSync, spawn, type ChildProcess } from "node:child_process";
import { cpSync, existsSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

// frontend を build し（dist を Go に配信させる）、フィクスチャを一時ディレクトリに
// コピーして KENSAN_DATA_DIR に指し、Go backend を子プロセスで起動する。
// teardown 用のハンドルは globalThis 経由で global-teardown に渡す。

const PORT = process.env.KENSAN_E2E_PORT ?? "8099";
const appDir = resolve(__dirname, "..");
const frontendDir = join(appDir, "frontend");
const backendDir = join(appDir, "backend");

export default async function globalSetup() {
  // 1. frontend build（KENSAN_STATIC_DIR が配信する dist）
  //    ローカル実行では frontend の deps が未 install のことがあるため保証する
  if (!existsSync(join(frontendDir, "node_modules"))) {
    execSync("npm ci", { cwd: frontendDir, stdio: "inherit" });
  }
  execSync("npm run build", { cwd: frontendDir, stdio: "inherit" });

  // 2. フィクスチャを書き込み可能な一時 workspace にコピー（テストが日記を作るため）
  const dataDir = mkdtempSync(join(tmpdir(), "kensan-e2e-"));
  cpSync(join(__dirname, "fixtures"), dataDir, { recursive: true });

  // 3. Go backend を起動
  const proc: ChildProcess = spawn("go", ["run", "./cmd/kensan"], {
    cwd: backendDir,
    env: {
      ...process.env,
      KENSAN_DATA_DIR: dataDir,
      KENSAN_ADDR: `:${PORT}`,
      KENSAN_STATIC_DIR: join(frontendDir, "dist"),
    },
    stdio: "inherit",
  });

  // 4. /healthz が通るまで待つ
  const base = `http://127.0.0.1:${PORT}`;
  const deadline = Date.now() + 60_000;
  for (;;) {
    try {
      const res = await fetch(`${base}/healthz`);
      if (res.ok) break;
    } catch {
      /* まだ起動中 */
    }
    if (Date.now() > deadline) {
      proc.kill("SIGKILL");
      throw new Error("kensan backend が 60s 以内に起動しませんでした");
    }
    await new Promise((r) => setTimeout(r, 300));
  }

  (globalThis as Record<string, unknown>).__kensanE2E = { proc, dataDir };

  return () => {
    const h = (globalThis as Record<string, unknown>).__kensanE2E as
      | { proc: ChildProcess; dataDir: string }
      | undefined;
    if (!h) return;
    h.proc.kill("SIGKILL");
    rmSync(h.dataDir, { recursive: true, force: true });
  };
}
