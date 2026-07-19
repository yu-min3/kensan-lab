import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// dev: vite serves the UI and proxies API/images to the Go server (:8090)
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": "http://localhost:8090",
      "/images": "http://localhost:8090",
    },
  },
});
