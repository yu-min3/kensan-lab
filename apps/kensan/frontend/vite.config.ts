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
      "/api": "http://localhost:8080",
    },
  },
});
