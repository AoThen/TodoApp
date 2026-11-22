import { test, expect } from '@playwright/test';

// 设备配对 E2E：登录后打开配对弹窗，应展示配对说明与操作入口
const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL || 'admin@e2e.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD || 'E2eAdmin@2026';

test.describe('设备配对', () => {
  test('打开配对弹窗并展示配对引导', async ({ page }) => {
    await page.goto('/');
    await page.fill('input[name="email"]', ADMIN_EMAIL);
    await page.fill('input[name="password"]', ADMIN_PASSWORD);
    await page.click('button:has-text("Login")');
    await expect(page.locator('input[placeholder="输入新任务..."]')).toBeVisible({ timeout: 15_000 });

    await page.click('button:has-text("设备配对")');
    await expect(page.locator('.pairing-description')).toBeVisible({ timeout: 15_000 });
    await expect(page.locator('.pairing-actions')).toBeVisible({ timeout: 15_000 });
  });
});
