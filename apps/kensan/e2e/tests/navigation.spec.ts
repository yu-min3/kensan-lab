import { test, expect } from '../fixtures'

test.describe('ナビゲーション', () => {
  test('サイドバー全リンク遷移確認', async ({ layoutPage, page }) => {
    await page.goto('/')

    // Daily (already on it)
    await expect(layoutPage.navDaily).toBeVisible()

    // Navigate to Briefing
    await layoutPage.navigateTo('briefing')
    await expect(page).toHaveURL('/briefing')

    // Navigate to Reflection
    await layoutPage.navigateTo('reflection')
    await expect(page).toHaveURL('/reflection')

    // Navigate to Notes
    await layoutPage.navigateTo('notes')
    await expect(page).toHaveURL('/notes')
    await expect(page.getByRole('heading', { name: 'ノート', exact: true, level: 1 })).toBeVisible()

    // Navigate to Tasks
    await layoutPage.navigateTo('tasks')
    await expect(page).toHaveURL('/tasks')
    await expect(page.getByRole('heading', { name: 'タスク管理' })).toBeVisible()

    // Navigate to Analytics
    await layoutPage.navigateTo('analytics')
    await expect(page).toHaveURL('/analytics')
    await expect(page.getByRole('heading', { name: '分析・レポート' })).toBeVisible()

    // Navigate to AI Explorer
    await layoutPage.navigateTo('interactions')
    await expect(page).toHaveURL('/interactions')

    // Navigate back to Daily
    await layoutPage.navigateTo('daily')
    await expect(page).toHaveURL('/')
  })

  test('Kensanロゴでホームに戻る', async ({ layoutPage, page }) => {
    // Navigate away from home
    await page.goto('/notes')
    await expect(page).toHaveURL('/notes')

    // Click logo
    await layoutPage.logo.click()
    await expect(page).toHaveURL('/')
  })

  test('テーマ切り替えボタン', async ({ layoutPage, page }) => {
    await page.goto('/')

    // Get initial theme state
    const initialClass = await page.locator('html').getAttribute('class')

    // Toggle theme
    await layoutPage.themeToggle.click()

    // Theme class should change
    const newClass = await page.locator('html').getAttribute('class')
    expect(newClass).not.toBe(initialClass)
  })

  test('ユーザーメニュー表示', async ({ layoutPage, page }) => {
    await page.goto('/')

    // Open user menu
    await layoutPage.openUserMenu()

    // Menu items should be visible
    await expect(layoutPage.getSettingsMenuItem()).toBeVisible()
    await expect(layoutPage.getLogoutMenuItem()).toBeVisible()

    // Click settings
    await layoutPage.getSettingsMenuItem().click()
    await expect(page).toHaveURL('/settings')
  })
})
