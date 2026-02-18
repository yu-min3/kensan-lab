// API Configuration

export const API_CONFIG = {
  // Backend service base URLs
  // Production: env vars are empty string → same-origin requests via nginx proxy
  // Local dev: env vars are undefined → fallback to localhost ports
  baseUrls: {
    user: import.meta.env.VITE_USER_SERVICE_URL ?? 'http://localhost:8081',
    task: import.meta.env.VITE_TASK_SERVICE_URL ?? 'http://localhost:8082',
    timeblock: import.meta.env.VITE_TIMEBLOCK_SERVICE_URL ?? 'http://localhost:8084',
    analytics: import.meta.env.VITE_ANALYTICS_SERVICE_URL ?? 'http://localhost:8088',
    ai: import.meta.env.VITE_AI_SERVICE_URL ?? 'http://localhost:8089',
    memo: import.meta.env.VITE_MEMO_SERVICE_URL ?? 'http://localhost:8090',
    note: import.meta.env.VITE_NOTE_SERVICE_URL ?? 'http://localhost:8091',
  },
} as const
