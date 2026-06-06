import { test as base } from '@playwright/test'
import { LoginPage } from '../pages/login.page'
import { DailyPage } from '../pages/daily.page'
import { NoteListPage } from '../pages/note-list.page'
import { NoteEditPage } from '../pages/note-edit.page'
import { TaskManagementPage } from '../pages/task-management.page'
import { AnalyticsPage } from '../pages/analytics.page'
import { SettingsPage } from '../pages/settings.page'
import { ChatPanelPage } from '../pages/chat-panel.page'
import { LayoutPage } from '../pages/layout.page'
import { BriefingPage } from '../pages/briefing.page'

type Fixtures = {
  loginPage: LoginPage
  dailyPage: DailyPage
  noteListPage: NoteListPage
  noteEditPage: NoteEditPage
  taskManagementPage: TaskManagementPage
  analyticsPage: AnalyticsPage
  settingsPage: SettingsPage
  chatPanelPage: ChatPanelPage
  layoutPage: LayoutPage
  briefingPage: BriefingPage
}

export const test = base.extend<Fixtures>({
  loginPage: async ({ page }, use) => {
    await use(new LoginPage(page))
  },
  dailyPage: async ({ page }, use) => {
    await use(new DailyPage(page))
  },
  noteListPage: async ({ page }, use) => {
    await use(new NoteListPage(page))
  },
  noteEditPage: async ({ page }, use) => {
    await use(new NoteEditPage(page))
  },
  taskManagementPage: async ({ page }, use) => {
    await use(new TaskManagementPage(page))
  },
  analyticsPage: async ({ page }, use) => {
    await use(new AnalyticsPage(page))
  },
  settingsPage: async ({ page }, use) => {
    await use(new SettingsPage(page))
  },
  chatPanelPage: async ({ page }, use) => {
    await use(new ChatPanelPage(page))
  },
  layoutPage: async ({ page }, use) => {
    await use(new LayoutPage(page))
  },
  briefingPage: async ({ page }, use) => {
    await use(new BriefingPage(page))
  },
})

export { expect } from '@playwright/test'
