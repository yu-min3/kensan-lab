import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The backend serves the built assets, so the output goes where the Dockerfile
// copies it from. `base: './'` keeps the asset URLs relative, which matters
// because the app is reached through the gateway at a hostname it never knows.
export default defineConfig({
  plugins: [react()],
  base: './',
  build: { outDir: 'dist', emptyOutDir: true },
  // Only used by `npm run dev`; in the cluster there is no dev server.
  server: {
    proxy: { '/api': 'http://localhost:8000' },
  },
})
