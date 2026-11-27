import { defineConfig } from '@playwright/test';

// E2E 测试配置：Playwright 同时托管 Go 后端(:8080)与 vite 前端(:3000)，
// 测试结束后自动回收进程，避免外部起服的时序与残留问题。
const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL || 'admin@e2e.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD || 'E2eAdmin@2026';

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:3000',
    trace: 'retain-on-failure',
  },
  webServer: process.env.E2E_BASE_URL
    ? undefined
    : [
        {
          command: 'go run -tags sqlite_fts5 .',
          cwd: '..',
          url: 'http://localhost:8080/api/v1/health',
          reuseExistingServer: !process.env.CI,
          timeout: 120_000,
          env: {
            JWT_SECRET: 'e2e-test-secret-key-1234567890abcdef',
            DEVICE_KEY_KEK: '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
            INITIAL_ADMIN_EMAIL: ADMIN_EMAIL,
            INITIAL_ADMIN_PASSWORD: ADMIN_PASSWORD,
            SERVER_PORT: '8080',
            DATABASE_URL: '/tmp/e2e-todoapp.db',
            // 每个用例含 UI+API 两次登录，放宽限流避免 429 误伤
            MAX_LOGIN_ATTEMPTS: '1000',
            RATE_LIMIT_PER_MINUTE: '2000',
          },
        },
        {
          command: 'npm run dev -- --port 3000 --strictPort',
          url: 'http://localhost:3000',
          reuseExistingServer: !process.env.CI,
          timeout: 60_000,
        },
      ],
});
