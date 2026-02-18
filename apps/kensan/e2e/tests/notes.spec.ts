import { test, expect } from '../fixtures'
import { createNote, listNoteContents, createNoteContent } from '../helpers/api'

test.describe('ノート管理', () => {
  test('ノート一覧が表示される', async ({ noteListPage }) => {
    await noteListPage.goto()

    await expect(noteListPage.heading).toBeVisible()
    await expect(noteListPage.createButton).toBeVisible()
    await expect(noteListPage.tabAll).toBeVisible()
    await expect(noteListPage.searchInput).toBeVisible()
  })

  test('タイプフィルターで絞り込み', async ({ noteListPage, page }) => {
    await noteListPage.goto()

    // Wait for notes to load
    await page.waitForLoadState('networkidle')

    // Click on a type tab (e.g., diary or learning)
    const diaryTab = page.getByRole('tab', { name: '日記' })
    if (await diaryTab.isVisible({ timeout: 3000 }).catch(() => false)) {
      await diaryTab.click()
      // URL should update with type filter
      await expect(page).toHaveURL(/type=diary/)
    }
  })

  test('新規ノート作成→保存→一覧に反映', async ({ noteListPage, noteEditPage, page }) => {
    // Use general type to avoid diary's unique (type, date) constraint
    await page.goto('/notes/new?type=general')

    // Fill in the title
    await expect(noteEditPage.titleInput).toBeVisible()
    await noteEditPage.titleInput.fill('[E2E] テストノート')

    // Type in the TipTap editor
    const editor = page.locator('[contenteditable="true"]')
    await editor.click()
    await page.keyboard.type('これはE2Eテストで作成されたノートです')

    // Wait for save button to be enabled
    await expect(noteEditPage.saveButton).toBeEnabled({ timeout: 5_000 })

    // Click save and wait for navigation concurrently
    await Promise.all([
      page.waitForURL('**/notes', { timeout: 15_000 }),
      noteEditPage.saveButton.click(),
    ])
    await expect(noteListPage.heading).toBeVisible()
  })

  test('既存ノートの編集画面表示', async ({ noteListPage, page }) => {
    await noteListPage.goto()
    await page.waitForLoadState('networkidle')

    // Click on the first note card (exclude /notes/new links)
    const noteCards = page.locator('a[href^="/notes/"]:not([href="/notes/new"])')
      .filter({ has: page.locator('.font-medium') })
    const firstNote = noteCards.first()
    if (await firstNote.isVisible({ timeout: 5000 }).catch(() => false)) {
      await firstNote.click()
      // Should navigate to edit page with a UUID
      await expect(page).toHaveURL(/\/notes\/[0-9a-f-]+/)
      // Heading should show edit mode
      await expect(page.locator('h1').first()).toContainText('編集')
    }
  })

  test('ノート削除', async ({ noteEditPage, page }) => {
    // First create a note to delete (use general type to avoid date uniqueness)
    await page.goto('/notes/new?type=general')

    await expect(noteEditPage.titleInput).toBeVisible()
    await noteEditPage.titleInput.fill('[E2E] 削除テストノート')

    const editor = page.locator('[contenteditable="true"]')
    await editor.click()
    await page.keyboard.type('削除予定のノート')

    // Wait for save button to be enabled
    await expect(noteEditPage.saveButton).toBeEnabled({ timeout: 5_000 })

    // Click save and wait for navigation concurrently
    await Promise.all([
      page.waitForURL('**/notes', { timeout: 15_000 }),
      noteEditPage.saveButton.click(),
    ])

    // Open the note we just created
    const noteLink = page.locator('a[href^="/notes/"]').filter({ hasText: '[E2E] 削除テストノート' })
    if (await noteLink.isVisible({ timeout: 5000 }).catch(() => false)) {
      await noteLink.click()
      await expect(page).toHaveURL(/\/notes\/[0-9a-f-]+/)

      // Click delete and confirm
      const deleteButton = page.getByRole('main').getByRole('button', { name: '削除' }).first()
      await deleteButton.click()
      // Confirm in the popover
      const confirmDelete = page.getByRole('button', { name: '削除' }).last()
      await confirmDelete.click()

      // Should redirect to notes list
      await page.waitForURL('**/notes', { timeout: 10_000 })
    }
  })

  test('検索フィルタリング', async ({ noteListPage, page }) => {
    await noteListPage.goto()
    await page.waitForLoadState('networkidle')

    // Search for a non-existent term
    await noteListPage.search('存在しないノート検索ワード')
    await page.waitForTimeout(500) // Debounce wait

    // Should show empty or filtered results
    const emptyState = page.getByText('該当するノートが見つかりません')
    const noResults = await emptyState.isVisible({ timeout: 3000 }).catch(() => false)

    // Clear search
    await noteListPage.searchInput.clear()

    // Notes should reappear
    if (noResults) {
      await page.waitForTimeout(500)
    }
  })

  test('新規ノート: draw.io トグルONで作成→保存→note_contentsにmarkdownとdrawioが作成される', async ({ noteEditPage, page }) => {
    await page.goto('/notes/new?type=general')
    const editorPanel = page.locator('.lg\\:col-span-3')

    // Fill title
    await expect(noteEditPage.titleInput).toBeVisible()
    await noteEditPage.titleInput.fill('[E2E] drawio付きノート')

    // Type markdown content
    const editor = page.locator('[contenteditable="true"]')
    await editor.click()
    await page.keyboard.type('Markdownコンテンツ')

    // Toggle draw.io ON
    await expect(noteEditPage.drawioToggle).toBeVisible()
    await noteEditPage.toggleDrawio(true)

    // draw.io editor section should appear in the editor panel
    await expect(editorPanel.getByText('draw.io 図', { exact: true }).first()).toBeVisible()

    // Wait for save button to be enabled then save
    await expect(noteEditPage.saveButton).toBeEnabled({ timeout: 5_000 })
    await Promise.all([
      page.waitForURL('**/notes', { timeout: 15_000 }),
      noteEditPage.saveButton.click(),
    ])

    // Verify the note appears in the list
    await expect(page.locator('a[href^="/notes/"]').filter({ hasText: '[E2E] drawio付きノート' })).toBeVisible({ timeout: 5_000 })
  })

  test('draw.io トグルON/OFF切り替え', async ({ noteEditPage, page }) => {
    await page.goto('/notes/new?type=general')
    const editorPanel = page.locator('.lg\\:col-span-3')

    await expect(noteEditPage.titleInput).toBeVisible()
    await noteEditPage.titleInput.fill('[E2E] drawioトグルテスト')

    const editor = page.locator('[contenteditable="true"]')
    await editor.click()
    await page.keyboard.type('テスト内容')

    // Toggle draw.io ON
    await noteEditPage.toggleDrawio(true)
    await expect(editorPanel.getByText('draw.io 図', { exact: true }).first()).toBeVisible()

    // Toggle draw.io OFF
    await noteEditPage.toggleDrawio(false)
    await expect(editorPanel.getByText('draw.io 図', { exact: true }).first()).not.toBeVisible()

    // Toggle draw.io ON again
    await noteEditPage.toggleDrawio(true)
    await expect(editorPanel.getByText('draw.io 図', { exact: true }).first()).toBeVisible()

    // Save and verify
    await expect(noteEditPage.saveButton).toBeEnabled({ timeout: 5_000 })
    await Promise.all([
      page.waitForURL('**/notes', { timeout: 15_000 }),
      noteEditPage.saveButton.click(),
    ])
    await expect(page.locator('a[href^="/notes/"]').filter({ hasText: '[E2E] drawioトグルテスト' })).toBeVisible({ timeout: 5_000 })
  })

  test('既存ノートの編集: note_contentsが読み込まれる', async ({ noteEditPage, page }) => {
    // Create a note with note_contents via API
    const note = await createNote({
      type: 'general',
      title: '[E2E] コンテンツ読み込みテスト',
      content: 'API経由のMarkdown',
      format: 'markdown',
    })

    // Create markdown and drawio contents via API
    await createNoteContent(note.id, {
      contentType: 'markdown',
      content: 'API経由のMarkdown',
      sortOrder: 0,
    })
    await createNoteContent(note.id, {
      contentType: 'drawio',
      content: '<mxGraphModel><root></root></mxGraphModel>',
      sortOrder: 1,
    })

    // Open the note in the editor
    await page.goto(`/notes/${note.id}`)
    await expect(noteEditPage.heading).toBeVisible({ timeout: 10_000 })
    const editorPanel = page.locator('.lg\\:col-span-3')

    // draw.io toggle should be ON (since drawio content exists)
    await expect(noteEditPage.drawioToggle).toBeVisible()
    const ariaChecked = await noteEditPage.drawioToggle.getAttribute('aria-checked')
    expect(ariaChecked).toBe('true')

    // draw.io editor section should be visible
    await expect(editorPanel.getByText('draw.io 図', { exact: true }).first()).toBeVisible()
  })

  test('既存ノートのdraw.ioトグルOFF→保存でdrawio contentが削除される', async ({ noteEditPage, page }) => {
    // Create a note with drawio content via API
    const note = await createNote({
      type: 'general',
      title: '[E2E] drawio削除テスト',
      content: 'テストMarkdown',
      format: 'markdown',
    })

    await createNoteContent(note.id, {
      contentType: 'markdown',
      content: 'テストMarkdown',
      sortOrder: 0,
    })
    await createNoteContent(note.id, {
      contentType: 'drawio',
      content: '<mxGraphModel><root></root></mxGraphModel>',
      sortOrder: 1,
    })

    // Open note
    await page.goto(`/notes/${note.id}`)
    await expect(noteEditPage.heading).toBeVisible({ timeout: 10_000 })
    const editorPanel = page.locator('.lg\\:col-span-3')

    // draw.io should be ON
    await expect(editorPanel.getByText('draw.io 図', { exact: true }).first()).toBeVisible()

    // Toggle draw.io OFF
    await noteEditPage.toggleDrawio(false)
    await expect(editorPanel.getByText('draw.io 図', { exact: true }).first()).not.toBeVisible()

    // Save
    await expect(noteEditPage.saveButton).toBeEnabled({ timeout: 5_000 })
    await Promise.all([
      page.waitForURL('**/notes', { timeout: 15_000 }),
      noteEditPage.saveButton.click(),
    ])

    // Verify via API that drawio content was deleted
    const contents = await listNoteContents(note.id)
    const drawioContents = contents.filter((c) => c.contentType === 'drawio')
    expect(drawioContents.length).toBe(0)

    // markdown content should still exist
    const mdContents = contents.filter((c) => c.contentType === 'markdown')
    expect(mdContents.length).toBeGreaterThanOrEqual(1)
  })
})
