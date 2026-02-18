import { test, expect } from '../fixtures'

test.describe('設定', () => {
  test('設定ページが表示される', async ({ settingsPage, page }) => {
    await settingsPage.goto()

    await expect(settingsPage.timezoneLabel).toBeVisible()
    await expect(settingsPage.themeLabel).toBeVisible()
    await expect(settingsPage.saveButton).toBeVisible()
    // Cancel button should be visible (since user is already configured)
    await expect(settingsPage.cancelButton).toBeVisible()
  })

  test('テーマ変更', async ({ settingsPage, page }) => {
    await settingsPage.goto()

    // Change theme to dark
    await settingsPage.selectTheme('ダーク')

    // Save settings
    await settingsPage.saveButton.click()

    // Should navigate to home
    await page.waitForURL('/')

    // Verify dark mode is applied (html element should have class 'dark')
    const htmlClass = await page.locator('html').getAttribute('class')
    expect(htmlClass).toContain('dark')
  })
})
