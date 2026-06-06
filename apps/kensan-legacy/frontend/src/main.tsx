import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Toaster } from 'sonner'
import './index.css'
import App from './App.tsx'
import { httpClient } from './api/client'
import { useAuthStore } from './stores/useAuthStore'
import { initTelemetry } from './api/telemetry'

// Initialize OpenTelemetry tracing
initTelemetry()

// 401エラー時にログアウト処理を実行
httpClient.setOnUnauthorized(() => {
  useAuthStore.getState().logout()
})

async function enableMocking() {
  // MSWを有効化: 明示的にVITE_ENABLE_MSW=trueの場合のみ
  if (import.meta.env.VITE_ENABLE_MSW === 'true') {
    const { worker } = await import('./mocks/browser')
    return worker.start({
      onUnhandledRequest: 'bypass',
    })
  }
}

enableMocking().then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <App />
      <Toaster richColors position="top-right" theme={document.documentElement.classList.contains('dark') ? 'dark' : 'light'} />
    </StrictMode>,
  )
})
