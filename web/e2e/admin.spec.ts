import { test, expect } from '@playwright/test';

// 管理员功能 E2E：面板入口、用户列表、创建用户、锁定/解锁生效
// 依赖后端已用 INITIAL_ADMIN_EMAIL/INITIAL_ADMIN_PASSWORD 种子化管理员账户

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL || 'admin@e2e.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD || 'E2eAdmin@2026';
const API = process.env.E2E_API_URL || 'http://localhost:8080/api/v1';

test.describe('管理员功能', () => {
  test('管理面板展示用户列表，API 建用户后可见，锁定后登录被拒', async ({ page, request }) => {
    // API 登录获取管理令牌
    const loginRes = await request.post(`${API}/auth/login`, {
      data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD },
    });
    expect(loginRes.ok(), '管理员 API 登录失败').toBeTruthy();
    const { data } = await loginRes.json();
    const headers = { Authorization: `Bearer ${data.access_token}` };

    // UI 登录
    await page.goto('/');
    await page.fill('input[name="email"]', ADMIN_EMAIL);
    await page.fill('input[name="password"]', ADMIN_PASSWORD);
    await page.click('button:has-text("Login")');
    await expect(page.locator('input[placeholder="输入新任务..."]')).toBeVisible({ timeout: 15_000 });

    // API 创建新用户
    const newUserEmail = `e2e-admin-${Date.now()}@test.local`;
    const createRes = await request.post(`${API}/admin/users`, {
      headers,
      data: { email: newUserEmail, password: 'NewUser@2026x', role: 'user' },
    });
    expect(createRes.status(), await createRes.text()).toBe(201);

    // 打开管理面板：用户列表应包含管理员与新建用户
    await page.click('.admin-btn');
    await expect(page.locator('.admin-panel')).toBeVisible({ timeout: 15_000 });
    await expect(page.locator('.admin-table')).toContainText(ADMIN_EMAIL, { timeout: 15_000 });
    await expect(page.locator('.admin-table')).toContainText(newUserEmail, { timeout: 15_000 });

    // 锁定新用户后，其密码登录应被拒绝（423）
    const listRes = await request.get(`${API}/admin/users`, { headers });
    const listBody = await listRes.json();
    const target = (listBody.data.users as Array<{ id: number; email: string }>).find(u => u.email === newUserEmail);
    expect(target, '新用户应出现在用户列表').toBeTruthy();

    const lockRes = await request.post(`${API}/admin/users/${target!.id}/lock`, {
      headers,
      data: { duration_minutes: 5 },
    });
    expect(lockRes.ok(), '锁定用户失败').toBeTruthy();

    const lockedLogin = await request.post(`${API}/auth/login`, {
      data: { email: newUserEmail, password: 'NewUser@2026x' },
    });
    expect(lockedLogin.status()).toBe(423);

    // 解锁后可正常登录
    const unlockRes = await request.post(`${API}/admin/users/${target!.id}/unlock`, {
      headers,
      data: {},
    });
    expect(unlockRes.ok(), '解锁用户失败').toBeTruthy();
    const relogin = await request.post(`${API}/auth/login`, {
      data: { email: newUserEmail, password: 'NewUser@2026x' },
    });
    expect(relogin.ok(), '解锁后登录失败').toBeTruthy();
  });
});
