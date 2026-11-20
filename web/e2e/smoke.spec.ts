import { test, expect } from '@playwright/test';

// 冒烟用例：覆盖 登录 → 任务创建 → 状态流转 → 删除 → 登出 主链路
// 依赖后端已用 INITIAL_ADMIN_EMAIL/INITIAL_ADMIN_PASSWORD 种子化管理员账户

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL || 'admin@e2e.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD || 'E2eAdmin@2026';

test('登录 → 创建任务 → 状态流转 → 删除 → 登出', async ({ page }) => {
  const dialogs: string[] = [];
  page.on('dialog', async (d) => {
    dialogs.push(d.message());
    await d.accept();
  });

  // 1. 登录
  await page.goto('/');
  const emailInput = page.locator('input[name="email"]');
  await expect(emailInput).toBeVisible();
  await page.fill('input[name="email"]', ADMIN_EMAIL);
  await page.fill('input[name="password"]', ADMIN_PASSWORD);
  await page.click('button:has-text("Login")');

  // 2. 登录成功后进入任务界面
  const taskInput = page.locator('input[placeholder="输入新任务..."]');
  await expect(taskInput).toBeVisible({ timeout: 15_000 });

  // 3. 创建任务，并在该任务项容器内做断言（避免受种子数据干扰）
  const title = `E2E任务-${Date.now()}`;
  await taskInput.fill(title);
  await page.click('button.add-task-btn');
  const item = page.locator('.task-item', { hasText: title });
  await expect(item).toBeVisible({ timeout: 15_000 });

  // 4. 状态流转 todo → done
  const statusBadge = item.locator('.task-status');
  await expect(statusBadge).toHaveText('todo');
  await statusBadge.click();
  await expect(statusBadge).toHaveText('done', { timeout: 15_000 });

  // 5. 删除任务（window.confirm 已自动接受）
  await item.locator('.delete-btn').click();
  await expect(item).not.toBeVisible({ timeout: 15_000 });
  expect(dialogs.join(' ')).toContain('确定要删除任务');

  // 6. 登出回到登录页
  await page.click('.logout-btn');
  await expect(page.locator('input[name="email"]')).toBeVisible({ timeout: 15_000 });
});
