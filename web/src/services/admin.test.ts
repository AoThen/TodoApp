import { describe, it, expect, vi, beforeAll } from 'vitest';

// axios.create 返回的 mock 实例，用于捕获拦截器与方法调用
const { mockClient } = vi.hoisted(() => {
  const client: any = vi.fn();
  client.interceptors = {
    request: { use: vi.fn() },
    response: { use: vi.fn() },
  };
  client.get = vi.fn();
  client.post = vi.fn();
  client.patch = vi.fn();
  client.put = vi.fn();
  client.delete = vi.fn();
  return { mockClient: client };
});

vi.mock('axios', () => ({
  default: { create: vi.fn(() => mockClient) },
}));

import { adminService } from './admin';

let responseInterceptor: (response: any) => any;

beforeAll(() => {
  responseInterceptor = mockClient.interceptors.response.use.mock.calls[0][0];
});

describe('adminService 响应拦截器', () => {
  it('解包 {success, data} 信封，使方法直接拿到业务数据', () => {
    const response = { data: { success: true, data: { users: [{ id: 1 }] } } };
    expect(responseInterceptor(response).data).toEqual({ users: [{ id: 1 }] });
  });

  it('非信封结构保持原样', () => {
    const response = { data: { foo: 'bar' } };
    expect(responseInterceptor(response).data).toEqual({ foo: 'bar' });
  });

  it('data 为数组的信封同样解包', () => {
    const response = { data: { success: true, data: [1, 2, 3] } };
    // 信封判断针对外层对象是否为数组；data 本身是数组时正常解包（与 api.ts 行为一致）
    expect(responseInterceptor(response).data).toEqual([1, 2, 3]);
  });
});

describe('adminService 方法', () => {
  it('getUsers 拼接分页与筛选参数并返回业务数据', async () => {
    mockClient.get.mockResolvedValueOnce({
      data: { users: [{ id: 1, email: 'a@b.c' }], pagination: { page: 2, page_size: 10 } },
    });
    const result = await adminService.getUsers({ page: 2, page_size: 10, email: 'a@b.c', role: 'user' });
    expect(mockClient.get).toHaveBeenCalledWith('/admin/users?page=2&page_size=10&email=a%40b.c&role=user');
    expect(result.users).toHaveLength(1);
  });

  it('getUsers 无参数时不拼接查询串', async () => {
    mockClient.get.mockResolvedValueOnce({ data: { users: [], pagination: { page: 1 } } });
    await adminService.getUsers();
    expect(mockClient.get).toHaveBeenCalledWith('/admin/users?');
  });

  it('createUser 返回解包后的用户数据', async () => {
    mockClient.post.mockResolvedValueOnce({ data: { id: 7, email: 'new@x.com', role: 'user' } });
    const created = await adminService.createUser({ email: 'new@x.com', password: 'Str0ng!Pass', role: 'user' });
    expect(mockClient.post).toHaveBeenCalledWith('/admin/users', {
      email: 'new@x.com', password: 'Str0ng!Pass', role: 'user',
    });
    expect(created.id).toBe(7);
  });

  it('resetPassword 提交 new_password 字段', async () => {
    mockClient.post.mockResolvedValueOnce({});
    await adminService.resetPassword(3, 'NewPass!2026');
    expect(mockClient.post).toHaveBeenCalledWith('/admin/users/3/password', { new_password: 'NewPass!2026' });
  });

  it('lockUser 默认锁定 30 分钟', async () => {
    mockClient.post.mockResolvedValueOnce({});
    await adminService.lockUser(5);
    expect(mockClient.post).toHaveBeenCalledWith('/admin/users/5/lock', { duration_minutes: 30 });
  });

  it('getLoginLogs 传递筛选参数并返回解包后的日志', async () => {
    mockClient.get.mockResolvedValueOnce({ data: [{ id: 1, email: 'a@b.c', success: false }] });
    const logs = await adminService.getLoginLogs({ email: 'a@b.c', success: false, page: 1 });
    expect(mockClient.get).toHaveBeenCalledWith('/admin/logs/login?email=a%40b.c&success=false&page=1');
    expect(logs).toHaveLength(1);
  });

  it('getConfig 返回解包后的配置表', async () => {
    mockClient.get.mockResolvedValueOnce({ data: { maintenance: 'false' } });
    const config = await adminService.getConfig();
    expect(config).toEqual({ maintenance: 'false' });
  });

  it('deleteUser 调用正确的 DELETE 端点', async () => {
    mockClient.delete.mockResolvedValueOnce({});
    await adminService.deleteUser(9);
    expect(mockClient.delete).toHaveBeenCalledWith('/admin/users/9');
  });

  it('updateUser 提交 PATCH 与变更字段', async () => {
    mockClient.patch.mockResolvedValueOnce({});
    await adminService.updateUser(3, { role: 'admin' });
    expect(mockClient.patch).toHaveBeenCalledWith('/admin/users/3', { role: 'admin' });
  });

  it('unlockUser 调用解锁端点', async () => {
    mockClient.post.mockResolvedValueOnce({});
    await adminService.unlockUser(5);
    expect(mockClient.post).toHaveBeenCalledWith('/admin/users/5/unlock');
  });

  it('getActionLogs 传递筛选参数并返回解包后的日志', async () => {
    mockClient.get.mockResolvedValueOnce({ data: [{ id: 1, action: 'delete_user' }] });
    const logs = await adminService.getActionLogs({
      email: 'a@b.c',
      target_email: 't@b.c',
      action: 'delete_user',
      start_time: '2026-01-01T00:00:00Z',
      end_time: '2026-01-31T00:00:00Z',
      page: 1,
      page_size: 50,
    });
    expect(mockClient.get).toHaveBeenCalledWith(
      '/admin/logs/actions?email=a%40b.c&target_email=t%40b.c&action=delete_user&start_time=2026-01-01T00%3A00%3A00Z&end_time=2026-01-31T00%3A00%3A00Z&page=1&page_size=50'
    );
    expect(logs).toHaveLength(1);
  });

  it('getActionLogs 无参数时不拼接筛选串', async () => {
    mockClient.get.mockResolvedValueOnce({ data: [] });
    await adminService.getActionLogs();
    expect(mockClient.get).toHaveBeenCalledWith('/admin/logs/actions?');
  });

  it('setConfig 提交 key/value 与可选描述', async () => {
    mockClient.put.mockResolvedValueOnce({});
    await adminService.setConfig('maintenance', 'true', '维护开关');
    expect(mockClient.put).toHaveBeenCalledWith('/admin/config', {
      key: 'maintenance',
      value: 'true',
      description: '维护开关',
    });
  });
});
