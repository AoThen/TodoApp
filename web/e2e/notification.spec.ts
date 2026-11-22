import { test, expect } from '@playwright/test';

// 通知实时链路 E2E：登录后 WebSocket 建立连接，API 创建通知经服务端推送，通知中心可见
const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL || 'admin@e2e.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD || 'E2eAdmin@2026';
const API = process.env.E2E_API_URL || 'http://localhost:8080/api/v1';

test.describe('通知中心', () => {
  test('API 创建的通知实时出现在通知中心', async ({ page, request }) => {
    // 收集页面错误与控制台错误，便于失败时定位渲染崩溃
    page.on('pageerror', err => console.log('[pageerror]', err.message));

    // API 登录拿令牌（与页面同一账户，通知发给本人）
    const loginRes = await request.post(`${API}/auth/login`, {
      data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD },
    });
    expect(loginRes.ok(), 'API 登录失败').toBeTruthy();
    const { data } = await loginRes.json();
    const headers = { Authorization: `Bearer ${data.access_token}` };

    // UI 登录（挂载 NotificationSystem 并建立 WS 连接）
    await page.goto('/');
    await page.fill('input[name="email"]', ADMIN_EMAIL);
    await page.fill('input[name="password"]', ADMIN_PASSWORD);
    await page.click('button:has-text("Login")');
    await expect(page.locator('input[placeholder="输入新任务..."]')).toBeVisible({ timeout: 15_000 });
    await expect(page.locator('.notification-bell')).toBeVisible({ timeout: 15_000 });

    // 等待 WS 就绪后再创建通知，避免推送早于订阅
    await page.waitForTimeout(1500);

    const title = `E2E通知-${Date.now()}`;
    const createRes = await request.post(`${API}/notifications`, {
      headers,
      data: { type: 'system', title, content: '通知中心 E2E 验证' },
    });
    expect(createRes.ok(), await createRes.text()).toBeTruthy();

    // 打开通知中心，断言标题出现（WS 实时推送或打开时拉取）
    await page.click('.notification-bell');
    await expect(page.locator('.notification-center')).toBeVisible({ timeout: 15_000 });
    await expect(page.locator('.notification-center')).toContainText(title, { timeout: 15_000 });
  });
});
