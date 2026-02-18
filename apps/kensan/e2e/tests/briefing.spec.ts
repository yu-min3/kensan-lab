import { test, expect } from '../fixtures'

test.describe('ブリーフィング（朝）', () => {
  test('ブリーフィングページが表示される', async ({ briefingPage }) => {
    await briefingPage.gotoBriefing()

    // Greeting should be visible
    await expect(briefingPage.heading).toBeVisible()
    await expect(briefingPage.heading).toContainText('おはようございます')
  })

  test('ステータスバーが表示される', async ({ briefingPage, page }) => {
    await briefingPage.gotoBriefing()

    // Status bar steps should be visible
    await expect(page.getByText('昨日の実績を取得')).toBeVisible({ timeout: 10_000 })
  })

  test('カードが表示される', async ({ briefingPage, page }) => {
    await briefingPage.gotoBriefing()

    // Wait for cards to load (SSE streaming)
    await expect(briefingPage.getCard('昨日の実績')).toBeVisible({ timeout: 15_000 })
    await expect(briefingPage.getCard('今日のフォーカス')).toBeVisible()
    await expect(briefingPage.getCard('タイムブロック提案')).toBeVisible()
    await expect(briefingPage.getCard('持ち越しタスク')).toBeVisible()
    await expect(briefingPage.getCard('AIインサイト')).toBeVisible()
  })

  test('タイムブロック提案の承認ボタンが表示される', async ({ briefingPage }) => {
    await briefingPage.gotoBriefing()

    // Wait for action proposal
    const approveButton = briefingPage.getApproveButton()
    await expect(approveButton).toBeVisible({ timeout: 15_000 })

    const skipButton = briefingPage.getSkipButton()
    await expect(skipButton).toBeVisible()
  })

  test('フォローアップボタンが表示される', async ({ briefingPage }) => {
    await briefingPage.gotoBriefing()

    // Follow-up CTA should appear after loading
    await expect(briefingPage.followUpButton).toBeVisible({ timeout: 15_000 })
    await expect(briefingPage.followUpButton).toContainText('AIに相談する')
  })
})

test.describe('振り返り（夜）', () => {
  test('振り返りページが表示される', async ({ briefingPage }) => {
    await briefingPage.gotoReflection()

    // Greeting should be visible
    await expect(briefingPage.heading).toBeVisible()
    await expect(briefingPage.heading).toContainText('お疲れさまでした')
  })

  test('カードが表示される', async ({ briefingPage }) => {
    await briefingPage.gotoReflection()

    // Wait for cards to load
    await expect(briefingPage.getCard('実績 vs 計画')).toBeVisible({ timeout: 15_000 })
    await expect(briefingPage.getCard('今日の完了タスク')).toBeVisible()
    await expect(briefingPage.getCard('振り返り・学習日記')).toBeVisible()
    await expect(briefingPage.getCard('明日のタスク提案')).toBeVisible()
  })

  test('学習日記セクションが表示される', async ({ briefingPage }) => {
    await briefingPage.gotoReflection()

    // Learning diary section should appear in ai_insight card
    await expect(briefingPage.getCard('振り返り・学習日記')).toBeVisible({ timeout: 15_000 })

    // Save diary button
    const saveButton = briefingPage.getSaveDiaryButton()
    await expect(saveButton).toBeVisible({ timeout: 15_000 })

    // Open editor button
    const editorButton = briefingPage.getOpenEditorButton()
    await expect(editorButton).toBeVisible()
  })

  test('フォローアップボタンが表示される', async ({ briefingPage }) => {
    await briefingPage.gotoReflection()

    await expect(briefingPage.followUpButton).toBeVisible({ timeout: 15_000 })
    await expect(briefingPage.followUpButton).toContainText('振り返りを深掘りする')
  })
})
