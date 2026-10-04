import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  testMatch: "sense-dev.spec.ts",
  fullyParallel: false,
  workers: 1,
  reporter: "list",
  use: { channel: "chrome", trace: "on-first-retry" },
});
