import { defineConfig } from '@playwright/test';

// E2E 冒烟测试配置
// 后端（Go, :8080）由外部启动（本地 `go run .` / CI job 后台进程）；
// 前端 vite dev server 默认由 Playwright 拉起（端口 3000，命中后端 CORS 白名单）。
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
    : {
        command: 'npm run dev -- --port 3000 --strictPort',
        url: 'http://localhost:3000',
        reuseExistingServer: !process.env.CI,
        timeout: 60_000,
      },
});
