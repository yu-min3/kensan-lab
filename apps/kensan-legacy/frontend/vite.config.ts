import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// In Docker containers, use internal network hostnames; on host, use localhost
const lokiTarget = process.env.LOKI_URL || 'http://localhost:3100'
const tempoTarget = process.env.TEMPO_URL || 'http://localhost:3200'
const otlpTarget = process.env.OTLP_URL || 'http://localhost:4318'

const observabilityProxy = {
  '/loki': {
    target: lokiTarget,
    changeOrigin: true,
    rewrite: (path: string) => path.replace(/^\/loki/, ''),
  },
  '/tempo': {
    target: tempoTarget,
    changeOrigin: true,
    rewrite: (path: string) => path.replace(/^\/tempo/, ''),
  },
  '/otlp': {
    target: otlpTarget,
    changeOrigin: true,
    rewrite: (path: string) => path.replace(/^\/otlp/, ''),
  },
}

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: observabilityProxy,
    allowedHosts: true,
  },
  preview: {
    proxy: observabilityProxy,
    allowedHosts: true,
  },
})
