import type { Page, Locator } from '@playwright/test'

export class LoginPage {
  readonly page: Page
  readonly emailInput: Locator
  readonly passwordInput: Locator
  readonly loginButton: Locator
  readonly registerToggle: Locator
  readonly nameInput: Locator
  readonly errorMessage: Locator
  readonly heading: Locator

  constructor(page: Page) {
    this.page = page
    this.emailInput = page.locator('#email')
    this.passwordInput = page.locator('#password')
    this.loginButton = page.getByRole('button', { name: 'ログイン' })
    this.registerToggle = page.getByRole('button', { name: '新規登録' })
    this.nameInput = page.locator('#name')
    this.errorMessage = page.locator('.text-red-600')
    this.heading = page.getByRole('heading', { name: 'Kensan' })
  }

  async goto() {
    await this.page.goto('/login')
  }

  async login(email: string, password: string) {
    await this.emailInput.fill(email)
    await this.passwordInput.fill(password)
    await this.loginButton.click()
  }
}
