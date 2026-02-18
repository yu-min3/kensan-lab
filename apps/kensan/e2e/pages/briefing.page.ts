import type { Page, Locator } from '@playwright/test'

export class BriefingPage {
  readonly page: Page

  // Header
  readonly heading: Locator
  readonly dateText: Locator

  // Status bar
  readonly statusBar: Locator

  // Cards
  readonly cards: Locator

  // Follow-up
  readonly followUpButton: Locator

  constructor(page: Page) {
    this.page = page
    this.heading = page.locator('h1').first()
    this.dateText = page.locator('h1 + p').first()
    this.statusBar = page.locator('.space-y-2').filter({ has: page.locator('.bg-brand') }).first()
    this.cards = page.locator('[class*="rounded-xl border"]')
    this.followUpButton = page.getByRole('button', { name: /AIに相談|振り返りを深掘り/ })
  }

  async gotoBriefing() {
    await this.page.goto('/briefing')
  }

  async gotoReflection() {
    await this.page.goto('/reflection')
  }

  getCard(title: string) {
    return this.page.locator('[class*="rounded-xl border"]').filter({ hasText: title })
  }

  getApproveButton() {
    return this.page.getByRole('button', { name: '承認して作成' })
  }

  getSkipButton() {
    return this.page.getByRole('button', { name: 'スキップ' })
  }

  getSaveDiaryButton() {
    return this.page.getByRole('button', { name: '学習日記として保存' })
  }

  getOpenEditorButton() {
    return this.page.getByRole('button', { name: 'ノートエディタで開く' })
  }
}
