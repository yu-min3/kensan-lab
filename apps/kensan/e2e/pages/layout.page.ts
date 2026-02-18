import type { Page, Locator } from '@playwright/test'

export class LayoutPage {
  readonly page: Page

  // Header
  readonly logo: Locator
  readonly themeToggle: Locator
  readonly aiButton: Locator
  readonly userMenuTrigger: Locator

  // Sidebar navigation
  readonly navDaily: Locator
  readonly navBriefing: Locator
  readonly navReflection: Locator
  readonly navNotes: Locator
  readonly navTasks: Locator
  readonly navAnalytics: Locator
  readonly navInteractions: Locator

  constructor(page: Page) {
    this.page = page

    // Header
    this.logo = page.locator('a[href="/"]').filter({ hasText: 'Kensan' })
    this.themeToggle = page.locator('button[title*="テーマ"]')
    this.aiButton = page.locator('button[title="AI Assistant"]')
    this.userMenuTrigger = page.locator('header button').filter({ has: page.locator('.rounded-full') })

    // Sidebar
    this.navDaily = page.locator('nav a[href="/"]').filter({ hasText: 'Daily' })
    this.navBriefing = page.locator('nav a[href="/briefing"]')
    this.navReflection = page.locator('nav a[href="/reflection"]')
    this.navNotes = page.locator('nav a[href="/notes"]')
    this.navTasks = page.locator('nav a[href="/tasks"]')
    this.navAnalytics = page.locator('nav a[href="/analytics"]')
    this.navInteractions = page.locator('nav a[href="/interactions"]')
  }

  async openUserMenu() {
    await this.userMenuTrigger.click()
  }

  getSettingsMenuItem() {
    return this.page.getByRole('menuitem').filter({ hasText: '設定' })
  }

  getLogoutMenuItem() {
    return this.page.getByRole('menuitem').filter({ hasText: 'ログアウト' })
  }

  async logout() {
    await this.openUserMenu()
    await this.getLogoutMenuItem().click()
  }

  async navigateTo(target: 'daily' | 'briefing' | 'reflection' | 'notes' | 'tasks' | 'analytics' | 'interactions') {
    const navMap = {
      daily: this.navDaily,
      briefing: this.navBriefing,
      reflection: this.navReflection,
      notes: this.navNotes,
      tasks: this.navTasks,
      analytics: this.navAnalytics,
      interactions: this.navInteractions,
    }
    await navMap[target].click()
  }
}
