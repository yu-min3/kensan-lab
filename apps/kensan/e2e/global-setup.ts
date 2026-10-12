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
  //    前回の backend が port に残っていると、古い fixture のまま healthz が通ってしまう
  const base = `http://127.0.0.1:${PORT}`;
  if (await healthy(base)) {
    throw new Error(
      `port ${PORT} で既に何かが応答している。前回の backend の残りなら止める: lsof -ti tcp:${PORT} -sTCP:LISTEN | xargs kill`,
    );
  }
  // go run は build した binary を子として起動するので、go だけ kill すると binary が残り
  // port と stdout を握り続ける（playwright がパイプ越しだと終わらない）。
  // 別の process group で起動し、group ごと止める。
  const proc: ChildProcess = spawn("go", ["run", "./cmd/kensan"], {
    detached: true,
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
  const deadline = Date.now() + 60_000;
  for (;;) {
    if (await healthy(base)) break;
    if (Date.now() > deadline) {
      stop(proc);
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
    stop(h.proc);
    rmSync(h.dataDir, { recursive: true, force: true });
  };
}

async function healthy(base: string): Promise<boolean> {
  try {
    return (await fetch(`${base}/healthz`)).ok;
  } catch {
    return false;
  }
}

function stop(proc: ChildProcess) {
  if (proc.pid === undefined) return;
  try {
    process.kill(-proc.pid, "SIGKILL");
  } catch {
    /* 既に終了している */
  }
}
