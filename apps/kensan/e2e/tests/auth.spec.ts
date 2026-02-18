import { test as base, expect } from '@playwright/test'
import { LoginPage } from '../pages/login.page'

// Auth tests use empty storageState (unauthenticated)
const test = base.extend<{ loginPage: LoginPage }>({
  storageState: { cookies: [], origins: [] },
  loginPage: async ({ page }, use) => {
    await use(new LoginPage(page))
  },
})

test.describe('認証', () => {
  test('ログインページが表示される', async ({ loginPage }) => {
    await loginPage.goto()

    await expect(loginPage.heading).toBeVisible()
    await expect(loginPage.emailInput).toBeVisible()
    await expect(loginPage.passwordInput).toBeVisible()
    await expect(loginPage.loginButton).toBeVisible()
  })

  test('正しい認証情報でログイン成功', async ({ loginPage, page }) => {
    await loginPage.goto()
    await loginPage.login('test@kensan.dev', 'password123')

    await page.waitForURL('/')
    // Daily page greeting should be visible
    await expect(page.locator('h1').first()).toBeVisible()
  })

  test('不正な認証情報でエラー表示', async ({ loginPage }) => {
    await loginPage.goto()
    await loginPage.login('wrong@example.com', 'wrongpassword')

    await expect(loginPage.errorMessage).toBeVisible()
  })

  test('ログアウト', async ({ loginPage, page }) => {
    // First, login
    await loginPage.goto()
    await loginPage.login('test@kensan.dev', 'password123')
    await page.waitForURL('/')

    // Open user menu and click logout
    const userMenuTrigger = page.locator('header button').filter({ has: page.locator('.rounded-full') })
    await userMenuTrigger.click()
    await page.getByRole('menuitem').filter({ hasText: 'ログアウト' }).click()

    await page.waitForURL('/login')
    await expect(loginPage.loginButton).toBeVisible()
  })
})
