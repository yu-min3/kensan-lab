import type { Page, Locator } from '@playwright/test'

export class AnalyticsPage {
  readonly page: Page
  readonly heading: Locator
  readonly tabToday: Locator
  readonly tabWeek: Locator
  readonly tabMonth: Locator
  readonly tabCustom: Locator
  readonly totalStudyTime: Locator
  readonly completedTasks: Locator
  readonly planAchievement: Locator
  readonly dailyAverage: Locator
  readonly pieChartTitle: Locator
  readonly barChartTitle: Locator
  readonly learningRecordsSection: Locator

  constructor(page: Page) {
    this.page = page
    this.heading = page.getByRole('heading', { name: '分析・レポート' })
    this.tabToday = page.getByRole('tab', { name: '今日' })
    this.tabWeek = page.getByRole('tab', { name: '今週' })
    this.tabMonth = page.getByRole('tab', { name: '今月' })
    this.tabCustom = page.getByRole('tab', { name: 'カスタム' })
    this.totalStudyTime = page.getByText('総学習時間')
    this.completedTasks = page.getByText('完了タスク')
    this.planAchievement = page.getByText('計画達成率')
    this.dailyAverage = page.getByText('日平均')
    this.pieChartTitle = page.getByText('目標別時間配分')
    this.barChartTitle = page.getByText('日別学習時間')
    this.learningRecordsSection = page.getByRole('heading', { name: '学習記録' })
  }

  async goto() {
    await this.page.goto('/analytics')
  }

  getCharts() {
    return this.page.locator('.recharts-wrapper')
  }
}
