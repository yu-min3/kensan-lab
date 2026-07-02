import type { Page, Locator } from '@playwright/test'

export class TaskManagementPage {
  readonly page: Page
  readonly heading: Locator
  readonly searchInput: Locator
  readonly hideCompletedCheckbox: Locator
  readonly goalsColumn: Locator
  readonly milestonesColumn: Locator
  readonly tasksColumn: Locator

  constructor(page: Page) {
    this.page = page
    this.heading = page.getByRole('heading', { name: 'タスク管理' })
    this.searchInput = page.getByPlaceholder('検索...')
    this.hideCompletedCheckbox = page.getByText('完了済みを隠す')
    // 3 columns are Card components with specific headers
    this.goalsColumn = page.locator('div').filter({ hasText: /^目標$/ }).first()
    this.milestonesColumn = page.locator('div').filter({ hasText: /^マイルストーン/ }).first()
    this.tasksColumn = page.locator('div').filter({ hasText: /^タスク/ }).first()
  }

  async goto() {
    await this.page.goto('/tasks')
  }

  getGoalByName(name: string) {
    return this.page.getByText(name, { exact: true })
  }

  getMilestoneByName(name: string) {
    return this.page.getByText(name, { exact: true })
  }

  getRecurringTaskWidget() {
    return this.page.getByText('定期タスク')
  }

  getGanttChart() {
    return this.page.getByText('ガントチャート').locator('..')
  }

  getGanttChartCard() {
    return this.page.locator('[class*="Card"]').filter({ hasText: 'ガントチャート' })
  }

  getInProgressMilestones() {
    return this.page.locator('[data-testid="milestone-in-progress"]')
  }

  getCompletedMilestones() {
    return this.page.locator('[data-testid="milestone-completed"]')
  }

  async search(query: string) {
    await this.searchInput.fill(query)
  }
}
