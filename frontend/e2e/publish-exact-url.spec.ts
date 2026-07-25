import { expect, test } from '@playwright/test'

const PASSWORD = process.env.WEKNORA_E2E_PASSWORD || 'Test1234!'
const EMAIL = process.env.WEKNORA_E2E_EMAIL || '421099982@qq.com'
const AGENT =
  process.env.WEKNORA_E2E_AGENT_ID ||
  'bea03e6d-9dc5-4d39-ad3a-4147e97cba12'

test('exact localhost publish url shows seven channel cards', async ({
  page,
}) => {
  await page.addInitScript(() => {
    for (const key of [
      'weknora:new-user-guide-done:v1',
      'weknora:contextual-guide-agent-list:v1',
      'weknora:contextual-guide-agent-create:v1',
      'weknora:contextual-guide-kb-list:v2',
      'weknora:contextual-guide-kb-create:v3',
      'weknora:contextual-guide-chat:v1',
    ]) {
      window.localStorage.setItem(key, '1')
    }
    window.localStorage.removeItem('weknora_lite_mode')
  })

  await page.goto('/login')
  await page.locator('input[autocomplete="email"]').fill(EMAIL)
  await page
    .locator('input[autocomplete="current-password"]')
    .fill(PASSWORD)
  await page.getByRole('button', { name: /登录|Log in|Login/i }).click()
  await page.waitForURL((url) => !url.pathname.includes('/login'), {
    timeout: 30_000,
  })

  await page.goto(`/platform/agents/${AGENT}?tab=publish`)
  await page.evaluate(() => {
    document
      .querySelectorAll('.guide__backdrop, .guide[role="dialog"]')
      .forEach((node) => node.remove())
  })

  await expect(page).toHaveURL(new RegExp(`${AGENT}.*tab=publish`))
  await expect(page.getByTestId('agent-publish-panel')).toBeVisible({
    timeout: 30_000,
  })
  await expect(page.getByTestId('agent-publish-empty')).toHaveCount(0)
  await expect(
    page.getByText('请先保存智能体后再配置发布渠道'),
  ).toHaveCount(0)
  await expect(page.getByTestId('agent-publish-channels')).toBeVisible()
  await expect(page.getByTestId('publish-channel-type-grid')).toBeVisible()
  // guest + embed + api + feishu + dingtalk + wechat + portal.
  await expect(page.locator('.channel-type-card')).toHaveCount(7)
  await expect(
    page.locator('.channel-type-card__title', { hasText: '免登录窗口' }),
  ).toBeVisible()
  await expect(
    page.locator('.channel-type-card__title', { hasText: '网页嵌入' }),
  ).toBeVisible()
  // Default selection is the guest-link short link; its own create action
  // reads "创建短链" (distinct from the embed panel's "创建新链接").
  await expect(page.getByText('创建短链').first()).toBeVisible()

  const panelBox = await page.getByTestId('agent-publish-panel').boundingBox()
  expect(panelBox?.width ?? 0).toBeGreaterThan(400)
  expect(panelBox?.height ?? 0).toBeGreaterThan(300)

  await page.screenshot({
    path: 'test-results/exact-publish-localhost.png',
    fullPage: true,
  })
})

test('legacy weknora_lite_mode localStorage no longer blanks publish', async ({
  page,
}) => {
  await page.addInitScript(() => {
    for (const key of [
      'weknora:new-user-guide-done:v1',
      'weknora:contextual-guide-agent-list:v1',
      'weknora:contextual-guide-agent-create:v1',
    ]) {
      window.localStorage.setItem(key, '1')
    }
    window.localStorage.setItem('weknora_lite_mode', 'true')
  })

  await page.goto('/login')
  await page.locator('input[autocomplete="email"]').fill(EMAIL)
  await page
    .locator('input[autocomplete="current-password"]')
    .fill(PASSWORD)
  await page.getByRole('button', { name: /登录|Log in|Login/i }).click()
  await page.waitForURL((url) => !url.pathname.includes('/login'), {
    timeout: 30_000,
  })

  await page.goto(`/platform/agents/${AGENT}?tab=publish`)
  await page.evaluate(() => {
    document
      .querySelectorAll('.guide__backdrop, .guide[role="dialog"]')
      .forEach((node) => node.remove())
  })

  await expect(page.getByTestId('publish-channel-type-grid')).toBeVisible({
    timeout: 30_000,
  })
  await expect(page.locator('.channel-type-card')).toHaveCount(7)
  await expect(page.getByTestId('agent-publish-empty')).toHaveCount(0)

  const liteFlag = await page.evaluate(
    () => window.localStorage.getItem('weknora_lite_mode'),
  )
  expect(liteFlag).toBeNull()
})

test('guest-link and web-embed channel type panels show distinct, non-overlapping copy', async ({
  page,
}) => {
  await page.addInitScript(() => {
    for (const key of [
      'weknora:new-user-guide-done:v1',
      'weknora:contextual-guide-agent-list:v1',
      'weknora:contextual-guide-agent-create:v1',
      'weknora:contextual-guide-kb-list:v2',
      'weknora:contextual-guide-kb-create:v3',
      'weknora:contextual-guide-chat:v1',
    ]) {
      window.localStorage.setItem(key, '1')
    }
  })

  await page.goto('/login')
  await page.locator('input[autocomplete="email"]').fill(EMAIL)
  await page
    .locator('input[autocomplete="current-password"]')
    .fill(PASSWORD)
  await page.getByRole('button', { name: /登录|Log in|Login/i }).click()
  await page.waitForURL((url) => !url.pathname.includes('/login'), {
    timeout: 30_000,
  })

  await page.goto(`/platform/agents/${AGENT}?tab=publish`)
  await page.evaluate(() => {
    document
      .querySelectorAll('.guide__backdrop, .guide[role="dialog"]')
      .forEach((node) => node.remove())
  })
  await expect(page.getByTestId('publish-channel-type-grid')).toBeVisible({
    timeout: 30_000,
  })

  const detail = page.locator('.channel-detail')

  // Default selection is the guest-link (免登录窗口) short link: its own
  // empty-state copy shows, and web-embed-only copy (iframe/widget create
  // button, embed table empty state) must not leak in.
  await expect(detail.getByRole('heading', { name: '免登录窗口' })).toBeVisible()
  await expect(detail.getByText('暂无网页嵌入渠道')).toHaveCount(0)
  await expect(detail.getByRole('button', { name: '创建新链接' })).toHaveCount(0)

  // Switching to 网页嵌入 flips it: embed-only copy shows, guest-link-only
  // copy (its empty state / short-link create button) disappears.
  await page.evaluate(() => {
    document
      .querySelectorAll('.guide__backdrop, .guide[role="dialog"]')
      .forEach((node) => node.remove())
  })
  await page.locator('.channel-type-card', { hasText: '网页嵌入' }).click({ force: true })

  await expect(detail.getByRole('heading', { name: '网页嵌入' })).toBeVisible()
  await expect(detail.getByRole('button', { name: '创建新链接' })).toBeVisible()
  await expect(detail.getByText('暂无网页嵌入渠道')).toBeVisible()
  await expect(detail.getByText('暂无免登录窗口链接')).toHaveCount(0)
  await expect(detail.getByRole('button', { name: '创建短链' })).toHaveCount(0)
})
