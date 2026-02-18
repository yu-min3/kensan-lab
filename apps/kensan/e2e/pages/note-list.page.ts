import type { Page, Locator } from '@playwright/test'

export class NoteListPage {
  readonly page: Page
  readonly heading: Locator
  readonly createButton: Locator
  readonly searchInput: Locator
  readonly tabAll: Locator
  readonly archiveToggle: Locator
  readonly emptyState: Locator
  readonly noteCards: Locator

  constructor(page: Page) {
    this.page = page
    this.heading = page.getByRole('heading', { name: 'ノート', exact: true, level: 1 })
    this.createButton = page.getByRole('link', { name: '新規作成' })
    this.searchInput = page.getByPlaceholder('検索...')
    this.tabAll = page.getByRole('tab', { name: 'すべて' })
    this.archiveToggle = page.getByRole('button', { name: /アーカイブ/ })
    this.emptyState = page.getByText('該当するノートが見つかりません')
    this.noteCards = page.locator('a[href^="/notes/"] .font-medium')
  }

  async goto() {
    await this.page.goto('/notes')
  }

  getTab(name: string) {
    return this.page.getByRole('tab', { name })
  }

  async search(query: string) {
    await this.searchInput.fill(query)
  }

  getNoteCardByTitle(title: string) {
    return this.page.locator(`a[href^="/notes/"]`).filter({ hasText: title })
  }
}
