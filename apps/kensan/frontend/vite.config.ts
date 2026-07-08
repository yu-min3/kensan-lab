import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    fs: {
      // monorepo ルートの packages/design-tokens を読むため
      allow: ["../../.."],
    },
    proxy: {
      // 別ポートの backend（並行ブランチの検証等）は KENSAN_API で切替
      "/api": process.env.KENSAN_API ?? "http://localhost:8080",
    },
  },
});
