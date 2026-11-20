import { describe, it, expect, vi, beforeAll, beforeEach } from 'vitest';

// axios.create 返回可调用（重试场景 this.client(config)）的 mock 实例
const { mockClient } = vi.hoisted(() => {
  const client: any = vi.fn((config: any) => Promise.resolve({ data: { retried: true, url: config?.url } }));
  client.interceptors = {
    request: { use: vi.fn() },
    response: { use: vi.fn() },
  };
  client.get = vi.fn();
  client.post = vi.fn();
  client.patch = vi.fn();
  client.delete = vi.fn();
  return { mockClient: client };
});

vi.mock('axios', () => ({
  default: { create: vi.fn(() => mockClient) },
}));

import { apiService } from './api';

let requestInterceptor: (config: any) => any;
let responseErrorHandler: (error: any) => Promise<any>;

beforeAll(() => {
  requestInterceptor = mockClient.interceptors.request.use.mock.calls[0][0];
  responseErrorHandler = mockClient.interceptors.response.use.mock.calls[0][1];
});

describe('apiService', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    mockClient.post.mockReset();
    mockClient.get.mockReset();
    // 保留拦截器捕获记录，仅重置调用
    mockClient.interceptors.request.use.mockClear();
    mockClient.interceptors.response.use.mockClear();
    Object.defineProperty(window, 'location', { value: { href: '' }, writable: true });
  });

  it('请求拦截器注入 Authorization 与 X-Request-ID', () => {
    localStorage.setItem('access_token', 'tok-123');
    const config = requestInterceptor({ headers: {} as any });
    expect(config.headers.Authorization).toBe('Bearer tok-123');
    expect(config.headers['X-Request-ID']).toBeTruthy();
  });

  it('无 token 时不注入 Authorization，但仍带 X-Request-ID', () => {
    const config = requestInterceptor({ headers: {} as any });
    expect(config.headers.Authorization).toBeUndefined();
    expect(config.headers['X-Request-ID']).toBeTruthy();
  });

  it('login 成功存储三个 token 字段', async () => {
    mockClient.post.mockResolvedValue({
      data: { access_token: 'at', refresh_token: 'rt', expires_in: 900 },
    });
    const result = await apiService.login('a@b.c', 'pass');
    expect(result.access_token).toBe('at');
    expect(localStorage.getItem('access_token')).toBe('at');
    expect(localStorage.getItem('refresh_token')).toBe('rt');
    expect(localStorage.getItem('token_expires_at')).toBeTruthy();
    expect(apiService.isTokenExpired()).toBe(false);
  });

  it('login 缺少 token 时抛错且不落盘', async () => {
    mockClient.post.mockResolvedValue({ data: { access_token: 'at' } });
    await expect(apiService.login('a@b.c', 'pass')).rejects.toThrow('No tokens received');
    expect(localStorage.getItem('access_token')).toBeNull();
  });

  it('isTokenExpired：无过期时间/已过期返回 true', () => {
    expect(apiService.isTokenExpired()).toBe(true);
    localStorage.setItem('token_expires_at', String(Date.now() - 1000));
    expect(apiService.isTokenExpired()).toBe(true);
    localStorage.setItem('token_expires_at', String(Date.now() + 60000));
    expect(apiService.isTokenExpired()).toBe(false);
  });

  it('401 时自动刷新 token 并重试原请求', async () => {
    localStorage.setItem('refresh_token', 'rt-old');
    mockClient.post.mockResolvedValue({
      data: { access_token: 'at-new', refresh_token: 'rt-new', expires_in: 900 },
    });

    const error = { response: { status: 401 }, config: { headers: {} as any } };
    const result = await responseErrorHandler(error);

    expect(mockClient.post).toHaveBeenCalledWith('/auth/refresh', { refresh_token: 'rt-old' });
    expect(localStorage.getItem('access_token')).toBe('at-new');
    expect(result.data.retried).toBe(true);
  });

  it('refresh 失败时清空 token 并跳转登录页', async () => {
    localStorage.setItem('access_token', 'at');
    localStorage.setItem('refresh_token', 'rt');
    mockClient.post.mockRejectedValue(new Error('refresh failed'));

    const error = { response: { status: 401 }, config: { headers: {} as any } };
    await expect(responseErrorHandler(error)).rejects.toThrow('refresh failed');
    expect(localStorage.getItem('access_token')).toBeNull();
    expect(localStorage.getItem('refresh_token')).toBeNull();
    expect(window.location.href).toBe('/login');
  });

  it('重试超过 3 次后放弃并跳转登录页', async () => {
    localStorage.setItem('refresh_token', 'rt');
    mockClient.post.mockResolvedValue({ data: { access_token: 'at', expires_in: 900 } });

    const config: any = { headers: {} };
    // 模拟已重试 4 次（_retry 尚为 false 时再次遇到 401，计数 +1 后超过 3）
    const error = { response: { status: 401 }, config: { ...config, _retry: false, _retryCount: 4 } };
    await expect(responseErrorHandler(error)).rejects.toThrow('Token refresh failed after multiple attempts');
    expect(localStorage.getItem('access_token')).toBeNull();
    expect(window.location.href).toBe('/login');
  });

  it('sync 按 delta 契约提交', async () => {
    mockClient.post.mockResolvedValue({ data: { server_changes: [], client_changes: [], conflicts: [], last_sync_at: 't' } });
    const changes: any[] = [{ id: 1, local_id: 'l1', op: 'insert', payload: {}, client_version: 0, timestamp: 't' }];
    const result = await apiService.sync('2026-01-01', changes);
    expect(mockClient.post).toHaveBeenCalledWith('/sync', { last_sync_at: '2026-01-01', changes });
    expect(result.last_sync_at).toBe('t');
  });

  it('batchDeleteTasks 以 task_ids body 发送 DELETE', async () => {
    mockClient.delete.mockResolvedValue({ data: {} });
    await apiService.batchDeleteTasks([1, 2, 3]);
    expect(mockClient.delete).toHaveBeenCalledWith('/tasks/batch', { data: { task_ids: [1, 2, 3] } });
  });

  it('logout 无论接口成败都清理本地 token', async () => {
    localStorage.setItem('access_token', 'at');
    localStorage.setItem('refresh_token', 'rt');
    mockClient.post.mockRejectedValue(new Error('network down'));
    await apiService.logout();
    expect(localStorage.getItem('access_token')).toBeNull();
    expect(localStorage.getItem('refresh_token')).toBeNull();
  });
});
