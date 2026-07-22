import { defineConfig, devices } from '@playwright/test'

const baseURL = process.env.WEKNORA_E2E_UI_HOST || 'http://127.0.0.1:5173'
const workers = Number(process.env.PW_WORKERS || '2')

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: Number.isFinite(workers) && workers > 0 ? Math.min(workers, 3) : 2,
  timeout: 90_000,
  expect: { timeout: 15_000 },
  reporter: [['list']],
  use: {
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'off',
    viewport: { width: 1440, height: 900 },
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
})
