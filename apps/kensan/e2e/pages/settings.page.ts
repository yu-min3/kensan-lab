import type { Page, Locator } from '@playwright/test'

export class SettingsPage {
  readonly page: Page
  readonly heading: Locator
  readonly timezoneLabel: Locator
  readonly themeLabel: Locator
  readonly saveButton: Locator
  readonly cancelButton: Locator

  constructor(page: Page) {
    this.page = page
    // CardTitle renders as a div, not a heading role
    this.heading = page.getByText('設定', { exact: true }).first()
    this.timezoneLabel = page.getByText('タイムゾーン')
    this.themeLabel = page.getByText('テーマ')
    this.saveButton = page.getByRole('button', { name: /設定を保存/ })
    this.cancelButton = page.getByRole('button', { name: 'キャンセル' })
  }

  async goto() {
    await this.page.goto('/settings')
  }

  /** Select a theme by clicking the theme select and choosing an option */
  async selectTheme(theme: 'システム設定に従う' | 'ライト' | 'ダーク') {
    // Click the theme select trigger button
    const themeButton = this.page.locator('button').filter({ hasText: /システム設定に従う|ライト|ダーク/ }).last()
    await themeButton.click()
    // SelectItem is a plain div with data-selected attribute (custom Select, not Radix)
    await this.page.locator('[data-selected]').filter({ hasText: theme }).click()
  }
}
