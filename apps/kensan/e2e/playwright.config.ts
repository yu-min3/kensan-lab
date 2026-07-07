import { defineConfig } from "@playwright/test";

// smoke E2E: frontend を build 済み前提で Go backend を単一プロセス起動し、
// フィクスチャ workspace を KENSAN_DATA_DIR に指してブラウザから叩く。
// 起動は global-setup.ts が担う（build → go run を子プロセスで立てる）。
const PORT = process.env.KENSAN_E2E_PORT ?? "8099";

export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? "line" : "list",
  globalSetup: "./global-setup.ts",
  use: {
    baseURL: `http://127.0.0.1:${PORT}`,
    trace: "on-first-retry",
  },
});
