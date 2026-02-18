import type { Page, Locator } from '@playwright/test'

export class ChatPanelPage {
  readonly page: Page
  readonly panel: Locator
  readonly headerTitle: Locator
  readonly closeButton: Locator
  readonly historyButton: Locator
  readonly newConversationButton: Locator
  readonly emptyState: Locator
  readonly messageInput: Locator

  constructor(page: Page) {
    this.page = page
    this.panel = page.locator('div').filter({ has: page.getByText('AI Assistant') }).first()
    this.headerTitle = page.getByText('AI Assistant')
    this.closeButton = page.locator('.border-l button').filter({ has: page.locator('svg') }).last()
    this.historyButton = page.locator('button[title="履歴"]')
    this.newConversationButton = page.locator('button[title="新しい会話"]')
    this.emptyState = page.getByText('/briefing からどうぞ')
    this.messageInput = page.getByPlaceholder(/メッセージ/)
  }

  async open() {
    await this.page.locator('button[title="AI Assistant"]').click()
  }

  async close() {
    // Click the X button in the chat panel header
    await this.page.locator('.border-l .border-b button').last().click()
  }
}
