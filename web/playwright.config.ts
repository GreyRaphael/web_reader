import { defineConfig, devices } from '@playwright/test'

const passwordHash = '$2a$10$icKXtl3i1EesvUogb6Qaru1gHMLMdtpYUEK3qRRuMfx..F/gfBpd2'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  retries: process.env.CI ? 2 : 0,
  reporter: [['list'], ['html', { open: 'never', outputFolder: '../playwright-report' }]],
  use: {
    baseURL: 'http://127.0.0.1:18848',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  webServer: {
    command: `../build/web-reader --addr 127.0.0.1:18848 --workspace ../testdata/workspace --password-hash '${passwordHash}'`,
    url: 'http://127.0.0.1:18848/healthz',
    reuseExistingServer: false,
    timeout: 30_000,
  },
  projects: [
    {
      name: 'desktop-chromium',
      use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 900 } },
    },
    {
      name: 'desktop-firefox',
      use: { ...devices['Desktop Firefox'], viewport: { width: 1440, height: 900 } },
    },
    {
      name: 'mobile-chromium',
      use: {
        ...devices['Pixel 7'],
        viewport: { width: 390, height: 844 },
      },
    },
    {
      name: 'mobile-webkit',
      use: {
        ...devices['iPhone 13'],
        viewport: { width: 360, height: 800 },
      },
    },
  ],
})
