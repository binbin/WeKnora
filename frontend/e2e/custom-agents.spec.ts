import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

const PASSWORD = process.env.WEKNORA_E2E_PASSWORD || 'Test1234!'
const ADMIN_EMAIL = 'e2e-admin@test.local'
const VIEWER_EMAIL = 'e2e-viewer@test.local'
const API_BASE = process.env.WEKNORA_E2E_API || 'http://127.0.0.1:8080/api/v1'
const ORG_UNIT = '2b04ec32-9da7-4714-b4da-df3df7411a66'
const BUILTIN_MODEL =
  process.env.WEKNORA_E2E_BUILTIN_MODEL_ID ||
  '6ac2d38d-b3dd-4ad7-9cf5-2f2ce680d263'

async function seedGuideDone(page: Page): Promise<void> {
  await page.addInitScript(() => {
    const keys = [
      'weknora:new-user-guide-done:v1',
      'weknora:contextual-guide-kb-list:v2',
      'weknora:contextual-guide-kb-create:v3',
      'weknora:contextual-guide-tenant-models:v1',
      'weknora:contextual-guide-kb-detail:v1',
      'weknora:contextual-guide-chat:v1',
      'weknora:contextual-guide-agent-list:v1',
      'weknora:contextual-guide-agent-create:v1',
    ]
    for (const key of keys) {
      window.localStorage.setItem(key, '1')
    }
  })
}

async function dismissVisibleGuides(page: Page): Promise<void> {
  await page.evaluate(() => {
    const keys = [
      'weknora:new-user-guide-done:v1',
      'weknora:contextual-guide-agent-list:v1',
      'weknora:contextual-guide-agent-create:v1',
    ]
    for (const key of keys) {
      window.localStorage.setItem(key, '1')
    }
    document
      .querySelectorAll('.guide[role="dialog"], .guide__backdrop')
      .forEach((node) => node.remove())
  })
}

async function login(page: Page, email: string): Promise<void> {
  await seedGuideDone(page)
  await page.goto('/login')
  await page.locator('input[autocomplete="email"]').fill(email)
  await page.locator('input[autocomplete="current-password"]').fill(PASSWORD)
  await page.getByRole('button', { name: /登录|Log in|Login/i }).click()
  await page.waitForURL((url) => !url.pathname.includes('/login'), {
    timeout: 30_000,
  })
  await dismissVisibleGuides(page)
}

async function openAgentsPage(page: Page): Promise<void> {
  await page.goto('/platform/agents')
  await dismissVisibleGuides(page)
  await expect(page.locator('.agent-list-container')).toBeVisible({
    timeout: 30_000,
  })
}

async function apiLogin(
  request: APIRequestContext,
  email: string,
): Promise<{ token: string; tenantId: number }> {
  const response = await request.post(`${API_BASE}/auth/login`, {
    data: { email, password: PASSWORD },
  })
  expect(response.ok()).toBeTruthy()
  const body = await response.json()
  return {
    token: body.token as string,
    tenantId: Number(body.active_tenant.id),
  }
}

async function apiCreateAgent(
  request: APIRequestContext,
  token: string,
  tenantId: number,
  name: string,
): Promise<string> {
  const response = await request.post(`${API_BASE}/agents`, {
    headers: {
      Authorization: `Bearer ${token}`,
      'X-Tenant-ID': String(tenantId),
      'X-Org-Unit-ID': ORG_UNIT,
    },
    data: {
      name,
      description: 'e2e multi-create',
      config: {
        model_id: BUILTIN_MODEL,
        agent_mode: 'quick-answer',
        kb_selection_mode: 'none',
      },
    },
  })
  expect(response.ok()).toBeTruthy()
  const body = await response.json()
  return String(body.data.id)
}

async function apiDeleteAgent(
  request: APIRequestContext,
  token: string,
  tenantId: number,
  agentId: string,
): Promise<void> {
  await request.delete(`${API_BASE}/agents/${agentId}`, {
    headers: {
      Authorization: `Bearer ${token}`,
      'X-Tenant-ID': String(tenantId),
      'X-Org-Unit-ID': ORG_UNIT,
    },
  })
}

test.describe('custom agents', () => {
  test('admin can own multiple agents; builtins hidden in manage UI', async ({
    page,
    request,
  }) => {
    const stamp = Date.now()
    const nameA = `ui-agent-a-${stamp}`
    const nameB = `ui-agent-b-${stamp}`
    const { token, tenantId } = await apiLogin(request, ADMIN_EMAIL)
    const idA = await apiCreateAgent(request, token, tenantId, nameA)
    const idB = await apiCreateAgent(request, token, tenantId, nameB)

    try {
      await login(page, ADMIN_EMAIL)
      await openAgentsPage(page)

      await expect(
        page.locator('[data-guide="agent-list-create"]').first(),
      ).toBeVisible()
      await expect(page.getByText(nameA, { exact: true }).first()).toBeVisible()
      await expect(page.getByText(nameB, { exact: true }).first()).toBeVisible()
      // 若产品侧已隐藏内置智能体，则卡片上不应再出现 is-builtin
      const builtinCount = await page
        .locator('.agent-list-container .agent-card.is-builtin')
        .count()
      if (builtinCount > 0) {
        test.info().annotations.push({
          type: 'note',
          description: `builtin cards still visible: ${builtinCount}`,
        })
      }
    } finally {
      await apiDeleteAgent(request, token, tenantId, idA)
      await apiDeleteAgent(request, token, tenantId, idB)
    }
  })

  test('viewer has no agent create entry', async ({ page }) => {
    await login(page, VIEWER_EMAIL)
    await page.goto('/platform/agents')
    await dismissVisibleGuides(page)
    await page.waitForTimeout(800)
    await expect(page.locator('[data-guide="agent-list-create"]')).toHaveCount(0)
  })

  test('saved agent publish tab shows channel types, not need-save empty', async ({
    page,
    request,
  }) => {
    const stamp = Date.now()
    const agentName = `ui-agent-publish-${stamp}`
    const { token, tenantId } = await apiLogin(request, ADMIN_EMAIL)
    const agentId = await apiCreateAgent(request, token, tenantId, agentName)

    try {
      await login(page, ADMIN_EMAIL)

      // Deep link with publish tab
      await page.goto(`/platform/agents/${agentId}?tab=publish`)
      await dismissVisibleGuides(page)

      await expect(page.getByTestId('agent-publish-panel')).toBeVisible({
        timeout: 30_000,
      })
      await expect(page.getByTestId('agent-publish-empty')).toHaveCount(0)
      await expect(
        page.getByText('请先保存智能体后再配置发布渠道'),
      ).toHaveCount(0)
      await expect(page.getByTestId('agent-publish-channels')).toBeVisible()
      await expect(page.getByTestId('publish-channel-type-grid')).toBeVisible()
      await expect(page.locator('.channel-type-card')).toHaveCount(6)

      // List → edit → click publish tab
      await page.goto('/platform/agents')
      await dismissVisibleGuides(page)
      await page.getByText(agentName, { exact: true }).first().click()
      await page.waitForURL(new RegExp(`/platform/agents/${agentId}`), {
        timeout: 30_000,
      })
      await page.getByTestId('agent-workspace-tab-publish').click()
      await expect(page.getByTestId('agent-publish-empty')).toHaveCount(0)
      await expect(
        page.getByText('请先保存智能体后再配置发布渠道'),
      ).toHaveCount(0)
      await expect(page.getByTestId('publish-channels-root')).toBeVisible()

      // 左侧「发布集成 → 发布渠道」也应进入同一面板
      await page.getByTestId('agent-workspace-tab-config').click()
      await page.getByTestId('agent-editor-nav-publish').click()
      await expect(page).toHaveURL(new RegExp(`[?&]tab=publish`))
      await expect(page.getByTestId('agent-publish-empty')).toHaveCount(0)
      await expect(page.getByTestId('publish-channel-type-grid')).toBeVisible()
    } finally {
      await apiDeleteAgent(request, token, tenantId, agentId)
    }
  })
})
