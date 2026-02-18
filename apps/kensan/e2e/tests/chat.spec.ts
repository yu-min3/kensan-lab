import { test, expect } from '../fixtures'

test.describe('AIチャット', () => {
  test('AIボタンでチャットパネル開閉', async ({ chatPanelPage, page }) => {
    await page.goto('/')

    // Panel should not be visible initially
    await expect(chatPanelPage.headerTitle).not.toBeVisible()

    // Open panel
    await chatPanelPage.open()
    await expect(chatPanelPage.headerTitle).toBeVisible()

    // Close panel via close button in the header
    // Find the X button within the chat panel's header
    const closeBtn = page.locator('.border-l .border-b button').filter({ has: page.locator('svg.lucide-x') })
    await closeBtn.click()
    await expect(chatPanelPage.headerTitle).not.toBeVisible()
  })

  test('パネルにウェルカムメッセージ表示', async ({ chatPanelPage, page }) => {
    await page.goto('/')

    await chatPanelPage.open()
    // WelcomeMessage shows greeting + tasks/schedules summary + briefing link
    await expect(chatPanelPage.emptyState).toBeVisible()
  })

  test('閉じるボタン', async ({ chatPanelPage, page }) => {
    await page.goto('/')

    // Open
    await chatPanelPage.open()
    await expect(chatPanelPage.headerTitle).toBeVisible()

    // Close via X button
    const closeBtn = page.locator('.border-l .border-b button').filter({ has: page.locator('svg.lucide-x') })
    await closeBtn.click()

    await expect(chatPanelPage.headerTitle).not.toBeVisible()
  })
})
