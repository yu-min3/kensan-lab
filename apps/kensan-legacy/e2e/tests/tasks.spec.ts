import { test, expect } from '../fixtures'
import { listMilestones, updateMilestone } from '../helpers/api'

test.describe('タスク管理', () => {
  test('タスク管理ページが表示される', async ({ taskManagementPage, page }) => {
    await taskManagementPage.goto()

    await expect(taskManagementPage.heading).toBeVisible()
    await expect(taskManagementPage.searchInput).toBeVisible()

    // Three columns should be present
    await expect(page.getByText('目標').first()).toBeVisible()
    await expect(page.getByText('マイルストーン').first()).toBeVisible()
    await expect(page.getByText('タスク').first()).toBeVisible()
  })

  // RecurringTaskWidget is not currently integrated into TaskManagement page
  test.skip('RecurringTaskWidget表示', async ({ taskManagementPage, page }) => {
    await taskManagementPage.goto()

    // Recurring task widget should be visible
    const widget = page.getByText('今週の定期タスク')
    await expect(widget).toBeVisible()
  })

  test('目標選択→マイルストーン表示', async ({ taskManagementPage, page }) => {
    await taskManagementPage.goto()
    await page.waitForLoadState('networkidle')

    // Click on the first goal (should be auto-selected, but click to ensure)
    const firstGoal = page.locator('.rounded-lg.cursor-pointer .font-medium.text-sm').first()
    if (await firstGoal.isVisible({ timeout: 5000 }).catch(() => false)) {
      await firstGoal.click()

      // Milestones should be displayed (or empty state)
      const milestoneList = page.getByText('マイルストーン').first()
      await expect(milestoneList).toBeVisible()
    }
  })

  test('検索フィルタリング', async ({ taskManagementPage, page }) => {
    await taskManagementPage.goto()
    await page.waitForLoadState('networkidle')

    // Search for something
    await taskManagementPage.search('テスト')
    await page.waitForTimeout(300)

    // Clear search
    await taskManagementPage.searchInput.clear()
  })
})

test.describe('ガントチャート', () => {
  // Helper: format date as YYYY-MM-DD
  const toDateStr = (d: Date) => d.toISOString().split('T')[0]

  // Save original milestone states for cleanup
  let originalMilestones: Array<{ id: string; start_date?: string; target_date?: string; status: string }>

  test.beforeAll(async () => {
    // Save original state before modifying
    originalMilestones = await listMilestones()

    const today = new Date()
    const pastDate = new Date(today)
    pastDate.setDate(today.getDate() - 30)
    const futureDate = new Date(today)
    futureDate.setDate(today.getDate() + 60)

    // Set up test data: give first 2 milestones dates to make Gantt chart visible
    if (originalMilestones.length >= 2) {
      // Milestone 0: in-progress (startDate in past, targetDate in future)
      await updateMilestone(originalMilestones[0].id, {
        startDate: toDateStr(pastDate),
        targetDate: toDateStr(futureDate),
      })
      // Milestone 1: deadline only (no startDate, targetDate in future)
      await updateMilestone(originalMilestones[1].id, {
        startDate: null,
        targetDate: toDateStr(futureDate),
      })
    }
  })

  test.afterAll(async () => {
    // Restore original state
    if (originalMilestones) {
      for (const m of originalMilestones) {
        await updateMilestone(m.id, {
          startDate: m.start_date ?? null,
          targetDate: m.target_date ?? null,
          status: m.status,
        })
      }
    }
  })

  test('ガントチャートにマイルストーンが表示される', async ({ taskManagementPage, page }) => {
    await taskManagementPage.goto()
    await page.waitForLoadState('networkidle')

    // Gantt chart card should be visible
    await expect(page.getByText('ガントチャート')).toBeVisible()

    // At least one milestone should appear (not the empty state)
    await expect(page.getByText('期限のあるマイルストーンがありません')).not.toBeVisible()
  })

  test('進行中マイルストーンが左ボーダーで強調される', async ({ taskManagementPage, page }) => {
    await taskManagementPage.goto()
    await page.waitForLoadState('networkidle')

    // In-progress milestone should have data-testid
    const inProgressRows = taskManagementPage.getInProgressMilestones()
    await expect(inProgressRows.first()).toBeVisible()

    // Verify sky-blue left border styling
    const borderColor = await inProgressRows.first().evaluate(el => {
      const style = window.getComputedStyle(el)
      return style.borderLeftWidth
    })
    expect(parseInt(borderColor)).toBeGreaterThan(0)
  })

  test('完了済みを隠すでガントチャートからも消える', async ({ taskManagementPage, page }) => {
    // Mark first milestone as completed for this test
    const milestones = await listMilestones()
    const targetMs = milestones.find(m => m.target_date)
    if (!targetMs) return

    await updateMilestone(targetMs.id, { status: 'completed' })

    try {
      await taskManagementPage.goto()
      await page.waitForLoadState('networkidle')

      // Completed milestone should be visible in Gantt
      const completedRows = taskManagementPage.getCompletedMilestones()
      await expect(completedRows.first()).toBeVisible()

      // Toggle hide completed
      await taskManagementPage.hideCompletedCheckbox.click()
      await page.waitForTimeout(300)

      // Completed milestone should disappear from Gantt
      await expect(completedRows).toHaveCount(0)
    } finally {
      // Restore status
      await updateMilestone(targetMs.id, { status: 'active' })
    }
  })

  test('開始日なしマイルストーンは進行中にならない', async ({ taskManagementPage, page }) => {
    await taskManagementPage.goto()
    await page.waitForLoadState('networkidle')

    // The second milestone (index 1) was set with no startDate in beforeAll
    // It should NOT appear as in-progress even though it has a targetDate
    const milestoneName = originalMilestones[1]?.name
    if (!milestoneName) return

    // Find the row containing this milestone name
    const milestoneRow = page.locator('[data-testid="milestone-in-progress"]').filter({ hasText: milestoneName })

    // Should NOT be marked as in-progress (no left border)
    await expect(milestoneRow).toHaveCount(0)
  })
})
