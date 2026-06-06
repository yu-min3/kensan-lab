import type { Page, Locator } from '@playwright/test'

export class DailyPage {
  readonly page: Page
  readonly greeting: Locator
  readonly todayMemo: Locator
  readonly createLearningRecord: Locator
  readonly createDiary: Locator
  readonly recordSection: Locator

  constructor(page: Page) {
    this.page = page
    this.greeting = page.locator('h1').first()
    this.todayMemo = page.getByText('今日のメモ')
    this.createLearningRecord = page.getByRole('link', { name: '学習記録を作成' })
    this.createDiary = page.getByRole('link', { name: '日記を書く' })
    this.recordSection = page.getByText('記録')
  }

  async goto() {
    await this.page.goto('/')
  }

  /** Navigate to the previous day */
  async goToPreviousDay() {
    await this.page.getByRole('button', { name: '前日' }).click()
  }

  /** Navigate to the next day */
  async goToNextDay() {
    await this.page.getByRole('button', { name: '翌日' }).click()
  }

  /** Get the time block section */
  getTimeBlockSection() {
    return this.page.locator('[data-timeline-container]')
  }

  /** Get the add time block button */
  getAddTimeBlockButton() {
    return this.page.getByRole('button', { name: /タイムブロック/ })
  }
}
