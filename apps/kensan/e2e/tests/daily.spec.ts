import { test, expect } from '../fixtures'

test.describe('デイリーページ', () => {
  test('デイリーページが表示される', async ({ dailyPage }) => {
    await dailyPage.goto()

    // Greeting should be visible
    await expect(dailyPage.greeting).toBeVisible()
    // Memo section
    await expect(dailyPage.todayMemo).toBeVisible()
    // Record section links
    await expect(dailyPage.createLearningRecord).toBeVisible()
    await expect(dailyPage.createDiary).toBeVisible()
  })

  test('タイムブロックの新規作成', async ({ dailyPage, page }) => {
    await dailyPage.goto()

    // Look for the add button in the time block section
    const addButton = page.getByRole('button', { name: /追加/ }).first()
    if (await addButton.isVisible()) {
      await addButton.click()
      // A dialog or inline form should appear
      await expect(page.locator('[role="dialog"], form')).toBeVisible()
    }
  })

  test('タイムブロックの削除', async ({ dailyPage, page }) => {
    await dailyPage.goto()

    // If there are existing time blocks, try to delete one
    const timeBlock = page.locator('[data-timeline-container] [role="button"]').first()
    if (await timeBlock.isVisible({ timeout: 3000 }).catch(() => false)) {
      await timeBlock.click()
      const deleteButton = page.getByRole('button', { name: '削除' })
      if (await deleteButton.isVisible({ timeout: 2000 }).catch(() => false)) {
        await deleteButton.click()
      }
    }
  })

  test('日付ナビゲーション', async ({ dailyPage, page }) => {
    await dailyPage.goto()

    // Get initial greeting text
    const initialText = await dailyPage.greeting.textContent()

    // Navigate to previous day
    const prevButton = page.locator('button').filter({ has: page.locator('svg') }).filter({ hasText: '' })
    // Look for chevron-left type buttons in the time block section
    const dateNav = page.locator('button[title="前日"], button:has(svg.lucide-chevron-left)').first()
    if (await dateNav.isVisible({ timeout: 3000 }).catch(() => false)) {
      await dateNav.click()
      // Greeting text should change (showing the date instead of greeting)
      await expect(dailyPage.greeting).not.toHaveText(initialText ?? '')
    }
  })

  test('記録セクションのリンク確認', async ({ dailyPage, page }) => {
    await dailyPage.goto()

    // Check learning record link
    await expect(dailyPage.createLearningRecord).toHaveAttribute('href', '/notes/new?type=learning')
    // Check diary link
    await expect(dailyPage.createDiary).toHaveAttribute('href', '/notes/new?type=diary')
  })
})
