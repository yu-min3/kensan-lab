import { chromium, type FullConfig } from '@playwright/test'
import path from 'node:path'
import fs from 'node:fs'

const AUTH_DIR = path.join(import.meta.dirname, '.auth')
const STORAGE_STATE_PATH = path.join(AUTH_DIR, 'user.json')

const BACKEND_SERVICES = [
  { name: 'user-service', url: 'http://localhost:8081/health' },
  { name: 'task-service', url: 'http://localhost:8082/health' },
  { name: 'timeblock-service', url: 'http://localhost:8084/health' },
  { name: 'analytics-service', url: 'http://localhost:8088/health' },
  { name: 'memo-service', url: 'http://localhost:8090/health' },
  { name: 'note-service', url: 'http://localhost:8091/health' },
]

async function checkBackendServices() {
  const failures: string[] = []

  for (const service of BACKEND_SERVICES) {
    try {
      const res = await fetch(service.url, { signal: AbortSignal.timeout(5_000) })
      if (!res.ok) {
        failures.push(`${service.name} (HTTP ${res.status})`)
      }
    } catch {
      failures.push(`${service.name} (unreachable)`)
    }
  }

  if (failures.length > 0) {
    throw new Error(
      `Backend services not ready. Run 'make dev-backend' first.\n` +
        `  Failed: ${failures.join(', ')}`
    )
  }
}

async function globalSetup(_config: FullConfig) {
  await checkBackendServices()

  fs.mkdirSync(AUTH_DIR, { recursive: true })

  const browser = await chromium.launch()
  const page = await browser.newPage()

  await page.goto('http://localhost:5174/login')
  await page.locator('#email').fill('test@kensan.dev')
  await page.locator('#password').fill('password123')
  await page.getByRole('button', { name: 'ログイン' }).click()

  // Wait for redirect to home page after login
  await page.waitForURL('http://localhost:5174/')

  await page.context().storageState({ path: STORAGE_STATE_PATH })
  await browser.close()
}

export default globalSetup
