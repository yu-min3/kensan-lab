import { test, expect } from '../fixtures'

test.describe('アナリティクス', () => {
  test('分析ページが表示される（サマリーカード4つ）', async ({ analyticsPage, page }) => {
    await analyticsPage.goto()
    await page.waitForLoadState('networkidle')

    await expect(analyticsPage.heading).toBeVisible()

    // 4 summary cards
    await expect(analyticsPage.totalStudyTime).toBeVisible()
    await expect(analyticsPage.completedTasks).toBeVisible()
    await expect(analyticsPage.planAchievement).toBeVisible()
    await expect(analyticsPage.dailyAverage).toBeVisible()
  })

  test('期間タブ切り替え', async ({ analyticsPage, page }) => {
    await analyticsPage.goto()

    // Wait for the summary to load (indicates page is ready)
    await expect(analyticsPage.totalStudyTime).toBeVisible()

    // Default should be "今週"
    await expect(analyticsPage.tabWeek).toHaveAttribute('aria-selected', 'true')

    // Switch to "今月"
    await analyticsPage.tabMonth.click()
    await expect(analyticsPage.tabMonth).toHaveAttribute('aria-selected', 'true')

    // Switch to "今日"
    await analyticsPage.tabToday.click()
    await expect(analyticsPage.tabToday).toHaveAttribute('aria-selected', 'true')
  })

  test('チャート表示確認', async ({ analyticsPage, page }) => {
    await analyticsPage.goto()
    await page.waitForLoadState('networkidle')

    // Chart titles should be visible
    await expect(analyticsPage.pieChartTitle).toBeVisible()
    await expect(analyticsPage.barChartTitle).toBeVisible()

    // SVG charts should be rendered
    const charts = analyticsPage.getCharts()
    await expect(charts.first()).toBeVisible()
  })
})
