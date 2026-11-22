import { test, expect, APIRequestContext, Page, Locator } from '@playwright/test';

// 跨平台同步 E2E：Web(UI) ↔ 服务端 ↔ B 设备(纯 API，模拟 Android 端走同一 sync 协议)
// 依赖后端已用 INITIAL_ADMIN_EMAIL/INITIAL_ADMIN_PASSWORD 种子化管理员账户

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL || 'admin@e2e.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD || 'E2eAdmin@2026';
const API = process.env.E2E_API_URL || 'http://localhost:8080/api/v1';

type ServerTask = {
  id: number;
  local_id: string;
  server_version: number;
  title: string;
  status: string;
  is_deleted: boolean;
};

type SyncChange = {
  local_id: string;
  op: 'insert' | 'update' | 'delete';
  payload: Record<string, unknown>;
  client_version: number;
};

class DeviceB {
  private constructor(private request: APIRequestContext, private token: string) {}

  static async create(request: APIRequestContext): Promise<DeviceB> {
    const res = await request.post(`${API}/auth/login`, {
      data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD },
    });
    expect(res.ok(), `B 设备登录失败: ${res.status()}`).toBeTruthy();
    const body = await res.json();
    expect(body?.data?.access_token, '登录响应缺少 access_token').toBeTruthy();
    return new DeviceB(request, body.data.access_token as string);
  }

  private headers() {
    return { Authorization: `Bearer ${this.token}` };
  }

  async sync(changes: SyncChange[] = [], lastSyncAt = '1970-01-01T00:00:00Z') {
    const res = await this.request.post(`${API}/sync`, {
      data: { last_sync_at: lastSyncAt, changes },
      headers: this.headers(),
    });
    expect(res.ok(), `B 设备同步失败: ${res.status()} ${await res.text()}`).toBeTruthy();
    return (await res.json()) as {
      server_changes: Array<Record<string, unknown>>;
      client_changes: Array<Record<string, unknown>>;
      conflicts: Array<Record<string, unknown>>;
      last_sync_at: string;
    };
  }

  async getTasks(): Promise<ServerTask[]> {
    const res = await this.request.get(`${API}/tasks?page=1&page_size=100`, { headers: this.headers() });
    expect(res.ok(), `B 设备拉取任务失败: ${res.status()}`).toBeTruthy();
    const body = await res.json();
    return body.data.tasks as ServerTask[];
  }

  async findTask(title: string): Promise<ServerTask | undefined> {
    return (await this.getTasks()).find(t => t.title === title && !t.is_deleted);
  }
}

async function login(page: Page) {
  await page.goto('/');
  await expect(page.locator('input[name="email"]')).toBeVisible();
  await page.fill('input[name="email"]', ADMIN_EMAIL);
  await page.fill('input[name="password"]', ADMIN_PASSWORD);
  await page.click('button:has-text("Login")');
  await expect(page.locator('input[placeholder="输入新任务..."]')).toBeVisible({ timeout: 15_000 });
}

async function createTaskViaUI(page: Page, title: string): Promise<Locator> {
  await page.fill('input[placeholder="输入新任务..."]', title);
  await page.click('button.add-task-btn');
  const item = page.locator('.task-item', { hasText: title });
  await expect(item).toBeVisible({ timeout: 15_000 });
  return item;
}

async function manualSync(page: Page) {
  await page.click('button:has-text("立即同步")');
  await expect(page.locator('button:has-text("同步中...")')).toHaveCount(0, { timeout: 15_000 });
}

async function editTaskTitle(page: Page, item: Locator, newTitle: string) {
  await item.locator('.task-title').click();
  const modal = page.locator('.task-edit-modal');
  await expect(modal).toBeVisible();
  await modal.locator('input[placeholder="任务标题"]').fill(newTitle);
  await modal.locator('button.task-edit-btn.primary').click();
  await expect(modal).not.toBeVisible({ timeout: 15_000 });
}

test.describe('跨平台同步', () => {
  test.setTimeout(60_000);

  test('Web 创建任务与状态切换同步到 B 设备', async ({ page, request }) => {
    const b = await DeviceB.create(request);
    await login(page);

    const title = `同步任务-${Date.now()}`;
    const item = await createTaskViaUI(page, title);

    // 不等首次同步、在 insert 防抖窗口内立即切换状态：验证变更合并进待发 insert 而非丢失
    await item.locator('.task-status').click();
    await expect(item.locator('.task-status')).toHaveText('done', { timeout: 15_000 });
    await manualSync(page);

    const created = await b.findTask(title);
    expect(created, 'B 设备应能拉取到 Web 创建的任务').toBeTruthy();
    expect(created!.status).toBe('done');
    // 变更被合并进待发 insert，服务端只看到一条携带最终状态的插入（v1）
    expect(created!.server_version).toBe(1);
  });

  test('B 设备创建的任务同步后出现在 Web 界面', async ({ page, request }) => {
    const b = await DeviceB.create(request);
    await login(page);

    const title = `安卓任务-${Date.now()}`;
    const result = await b.sync([
      { local_id: `b-insert-${Date.now()}`, op: 'insert', payload: { title, status: 'todo', priority: 'high' }, client_version: 0 },
    ]);
    expect(result.conflicts).toHaveLength(0);

    await manualSync(page);
    await expect(page.locator('.task-item', { hasText: title })).toBeVisible({ timeout: 15_000 });
  });

  test('Web 删除任务同步到 B 设备', async ({ page, request }) => {
    const b = await DeviceB.create(request);
    await login(page);
    page.on('dialog', d => d.accept());

    const title = `删除任务-${Date.now()}`;
    const item = await createTaskViaUI(page, title);
    await manualSync(page);
    expect(await b.findTask(title)).toBeTruthy();

    await item.locator('.delete-btn').click();
    await expect(item).not.toBeVisible({ timeout: 15_000 });
    await manualSync(page);

    expect(await b.findTask(title)).toBeUndefined();
  });

  test('冲突解决：保留本地（本地编辑最终覆盖服务端）', async ({ page, request }) => {
    const b = await DeviceB.create(request);
    await login(page);

    const title = `冲突任务-${Date.now()}`;
    const item = await createTaskViaUI(page, title);
    await manualSync(page);
    const onServer = await b.findTask(title);
    expect(onServer).toBeTruthy();
    expect(onServer!.server_version).toBe(1);

    // B 设备先改标题 → v2；Web 不知情，仍基于 v1 编辑 → 版本不匹配触发冲突
    await b.sync([
      { local_id: 'b-update-1', op: 'update', payload: { id: onServer!.id, title: `B改-${title}` }, client_version: 1 },
    ]);

    const webTitle = `Web改-${title}`;
    await editTaskTitle(page, item, webTitle);

    const modal = page.locator('.conflicts-modal');
    await expect(modal).toBeVisible({ timeout: 20_000 });
    await expect(page.locator('.conflict-item')).toContainText('intelligent_merge');

    await page.click('button:has-text("保留本地")');
    await expect(modal).not.toBeVisible({ timeout: 15_000 });

    // 重放完整本地字段（CV=冲突记录的版本号）→ 服务端干净应用
    await manualSync(page);
    const final = await b.findTask(webTitle);
    expect(final, '保留本地后服务端应为 Web 标题').toBeTruthy();
    expect(final!.server_version).toBe(4); // v1 → B改v2 → 合并v3 → 本地重放v4
    expect(await b.findTask(`B改-${title}`)).toBeUndefined();
  });

  test('冲突解决：使用服务器（服务端标题保持不变）', async ({ page, request }) => {
    const b = await DeviceB.create(request);
    await login(page);

    const title = `服冲任务-${Date.now()}`;
    const item = await createTaskViaUI(page, title);
    await manualSync(page);
    const onServer = await b.findTask(title);
    expect(onServer).toBeTruthy();

    await b.sync([
      { local_id: 'b-update-2', op: 'update', payload: { id: onServer!.id, title: `B保留-${title}` }, client_version: 1 },
    ]);

    await editTaskTitle(page, item, `Web弃-${title}`);
    const modal = page.locator('.conflicts-modal');
    await expect(modal).toBeVisible({ timeout: 20_000 });

    await page.click('button:has-text("使用服务器")');
    await expect(modal).not.toBeVisible({ timeout: 15_000 });
    await manualSync(page);

    const final = await b.findTask(`B保留-${title}`);
    expect(final, '使用服务器后服务端应保持 B 标题').toBeTruthy();
    expect(final!.server_version).toBe(3); // v1 → B改v2 → 合并v3（标题服务器优先）
    expect(await b.findTask(`Web弃-${title}`)).toBeUndefined();
  });

  test('离线创建任务，恢复联网后补同步到服务端', async ({ page, request }) => {
    const b = await DeviceB.create(request);
    await login(page);

    const title = `离线任务-${Date.now()}`;
    await page.context().setOffline(true);
    await createTaskViaUI(page, title);
    await page.context().setOffline(false);

    await manualSync(page);
    expect(await b.findTask(title)).toBeTruthy();
  });
});
