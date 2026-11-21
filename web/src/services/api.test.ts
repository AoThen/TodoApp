import { describe, it, expect, vi, beforeEach } from 'vitest';
import axios from 'axios';
import { apiService } from './api';

vi.mock('axios', () => {
  const client: any = vi.fn();
  client.get = vi.fn();
  client.post = vi.fn();
  client.patch = vi.fn();
  client.delete = vi.fn();
  client.interceptors = {
    request: { use: vi.fn() },
    response: { use: vi.fn() },
  };
  return {
    default: {
      create: vi.fn(() => client),
      __client: client,
    },
  };
});

const getClient = (): any => (axios as any).__client;

const requestFulfilled = () => getClient().interceptors.request.use.mock.calls[0][0];
const requestRejected = () => getClient().interceptors.request.use.mock.calls[0][1];
const responseFulfilled = () => getClient().interceptors.response.use.mock.calls[0][0];
const responseRejected = () => getClient().interceptors.response.use.mock.calls[0][1];

describe('apiService', () => {
  beforeEach(() => {
    const client = getClient();
    client.mockReset();
    client.get.mockReset();
    client.post.mockReset();
    client.patch.mockReset();
    client.delete.mockReset();
    Object.defineProperty(window, 'location', {
      value: { href: '' },
      writable: true,
    });
    localStorage.clear();
    sessionStorage.clear();
  });

  it('getClient 返回内部 axios 实例', () => {
    expect(apiService.getClient()).toBe(getClient());
  });

  it('login 成功：存储 access_token 与过期时间并返回', async () => {
    getClient().post.mockResolvedValueOnce({
      data: { access_token: 'at-1', expires_in: 900 },
    });

    const result = await apiService.login('a@b.com', 'pw');

    expect(getClient().post).toHaveBeenCalledWith('/auth/login', { email: 'a@b.com', password: 'pw' });
    expect(result).toEqual({ access_token: 'at-1', expires_in: 900 });
    expect(localStorage.getItem('access_token')).toBe('at-1');
    expect(localStorage.getItem('refresh_token')).toBeNull();
    expect(Number(localStorage.getItem('token_expires_at'))).toBeGreaterThan(Date.now());
  });

  it('login 响应缺少 access_token 时抛错', async () => {
    getClient().post.mockResolvedValueOnce({ data: {} });

    await expect(apiService.login('a@b.com', 'pw')).rejects.toThrow('No tokens received');
  });

  it('login 网络失败时向上抛出错误', async () => {
    getClient().post.mockRejectedValueOnce(new Error('network'));

    await expect(apiService.login('a@b.com', 'pw')).rejects.toThrow('network');
  });

  it('refreshToken 成功时写入新 access_token', async () => {
    getClient().post.mockResolvedValueOnce({
      data: { access_token: 'at-2', expires_in: 900 },
    });

    await apiService.refreshToken();

    expect(getClient().post).toHaveBeenCalledWith('/auth/refresh');
    expect(localStorage.getItem('access_token')).toBe('at-2');
  });

  it('refreshToken 失败时清理令牌并抛错', async () => {
    localStorage.setItem('access_token', 'stale');
    getClient().post.mockRejectedValueOnce(new Error('refresh failed'));

    await expect(apiService.refreshToken()).rejects.toThrow('refresh failed');
    expect(localStorage.getItem('access_token')).toBeNull();
  });

  it('logout：调用登出接口并清理本地令牌', async () => {
    localStorage.setItem('access_token', 'at-3');
    getClient().post.mockResolvedValueOnce({ data: {} });

    await apiService.logout();

    expect(getClient().post).toHaveBeenCalledWith('/auth/logout');
    expect(localStorage.getItem('access_token')).toBeNull();
    expect(sessionStorage.length).toBe(0);
  });

  it('logout：接口失败仍清理本地令牌', async () => {
    localStorage.setItem('access_token', 'at-4');
    getClient().post.mockRejectedValueOnce(new Error('boom'));

    await apiService.logout();

    expect(localStorage.getItem('access_token')).toBeNull();
  });

  it('isTokenExpired：无记录/已过期/未过期三分支', () => {
    expect(apiService.isTokenExpired()).toBe(true);

    localStorage.setItem('token_expires_at', String(Date.now() - 1000));
    expect(apiService.isTokenExpired()).toBe(true);

    localStorage.setItem('token_expires_at', String(Date.now() + 60000));
    expect(apiService.isTokenExpired()).toBe(false);
  });

  it('getTasks：拼接查询参数并跳过空值', async () => {
    getClient().get.mockResolvedValueOnce({ data: { tasks: [], pagination: {} } });

    await apiService.getTasks({ page: 2, status: 'todo', search: '', priority: undefined });

    expect(getClient().get).toHaveBeenCalledWith('/tasks?page=2&status=todo');
  });

  it('getTasks：无参数时使用空查询串', async () => {
    getClient().get.mockResolvedValueOnce({ data: { tasks: [], pagination: {} } });

    await apiService.getTasks();

    expect(getClient().get).toHaveBeenCalledWith('/tasks?');
  });

  it('任务 CRUD：create/delete 请求转发', async () => {
    getClient().post.mockResolvedValueOnce({ data: { id: 1, title: 't' } });
    getClient().delete.mockResolvedValueOnce({ data: {} });

    await apiService.createTask({ title: 't' } as any);
    await apiService.deleteTask(1);

    expect(getClient().post).toHaveBeenCalledWith('/tasks', { title: 't' });
    expect(getClient().delete).toHaveBeenCalledWith('/tasks/1');
  });

  it('sync：提交增量变更并返回同步结果', async () => {
    getClient().post.mockResolvedValueOnce({ data: { server_changes: [], client_changes: [], last_sync_at: 'x' } });

    await apiService.sync('2026-01-01', []);

    expect(getClient().post).toHaveBeenCalledWith('/sync', { last_sync_at: '2026-01-01', changes: [] });
  });

  it('exportTasks：以 blob 响应类型请求导出', async () => {
    const blob = new Blob(['x']);
    getClient().get.mockResolvedValueOnce({ data: blob });

    const result = await apiService.exportTasks('csv');

    expect(getClient().get).toHaveBeenCalledWith('/export', {
      params: { type: 'tasks', format: 'csv' },
      responseType: 'blob',
    });
    expect(result).toBe(blob);
  });

  it('importTasks / batchDeleteTasks / restoreTask 请求转发', async () => {
    getClient().post.mockResolvedValue({ data: { imported: 1 } });
    getClient().delete.mockResolvedValue({ data: {} });

    await apiService.importTasks({ items: [] });
    await apiService.batchDeleteTasks([1, 2]);
    await apiService.restoreTask(7);

    expect(getClient().post).toHaveBeenCalledWith('/import', { items: [] });
    expect(getClient().delete).toHaveBeenCalledWith('/tasks/batch', { data: { task_ids: [1, 2] } });
    expect(getClient().post).toHaveBeenCalledWith('/tasks/7/restore');
  });

  it('用户与设备接口：getCurrentUser/getDevices/initPairing/pairDevice/regenerateKey/revokeDevice', async () => {
    getClient().get.mockResolvedValue({ data: {} });
    getClient().post.mockResolvedValue({ data: {} });
    getClient().delete.mockResolvedValue({ data: {} });

    await apiService.getCurrentUser();
    await apiService.getDevices();
    await apiService.initPairing();
    await apiService.initPairing('tablet', 'dev-9');
    await apiService.pairDevice({ key: 'k', device_type: 'web', device_id: 'd1' });
    await apiService.regenerateKey('d1');
    await apiService.revokeDevice('d1');

    expect(getClient().get).toHaveBeenCalledWith('/users/me');
    expect(getClient().get).toHaveBeenCalledWith('/devices');
    expect(getClient().post).toHaveBeenCalledWith('/devices/pair/init', { device_type: 'web', device_id: undefined });
    expect(getClient().post).toHaveBeenCalledWith('/devices/pair/init', { device_type: 'tablet', device_id: 'dev-9' });
    expect(getClient().post).toHaveBeenCalledWith('/devices/pair', { key: 'k', device_type: 'web', device_id: 'd1' });
    expect(getClient().post).toHaveBeenCalledWith('/devices/d1/regenerate');
    expect(getClient().delete).toHaveBeenCalledWith('/devices/d1');
  });

  it('请求拦截器：注入 Authorization 与 X-Request-ID', () => {
    localStorage.setItem('access_token', 'tok-1');
    const config: any = { headers: {} };

    const result = requestFulfilled()(config);

    expect(result.headers.Authorization).toBe('Bearer tok-1');
    expect(result.headers['X-Request-ID']).toBeTruthy();
  });

  it('请求拦截器：无令牌时仅注入请求 ID', () => {
    const config: any = { headers: {} };

    const result = requestFulfilled()(config);

    expect(result.headers.Authorization).toBeUndefined();
    expect(result.headers['X-Request-ID']).toBeTruthy();
  });

  it('请求拦截器：错误直接 reject', async () => {
    const err = new Error('req fail');

    await expect(requestRejected()(err)).rejects.toBe(err);
  });

  it('响应拦截器：解包 {success, data} 包裹结构', () => {
    const response = { data: { success: true, data: { id: 1 } } };

    const result = responseFulfilled()(response);

    expect(result.data).toEqual({ id: 1 });
  });

  it('响应拦截器：数组与普通对象不解包', () => {
    const arrayResponse = { data: [1, 2, 3] };
    const plainResponse = { data: { id: 1 } };

    expect(responseFulfilled()(arrayResponse).data).toEqual([1, 2, 3]);
    expect(responseFulfilled()(plainResponse).data).toEqual({ id: 1 });
  });

  it('401 刷新请求自身失败：清理令牌、跳转登录并抛错', async () => {
    localStorage.setItem('access_token', 'stale');
    const error: any = {
      config: { url: '/auth/refresh' },
      response: { status: 401 },
      message: 'unauthorized',
    };

    await expect(responseRejected()(error)).rejects.toThrow('Refresh token invalid');
    expect(localStorage.getItem('access_token')).toBeNull();
    expect(window.location.href).toBe('/login');
  });

  it('401 业务请求：刷新成功后携带新令牌重放请求', async () => {
    localStorage.setItem('access_token', 'old');
    getClient().post.mockResolvedValueOnce({ data: { access_token: 'new-at', expires_in: 900 } });
    getClient().mockResolvedValueOnce({ data: { ok: true } });
    const error: any = {
      config: { url: '/tasks', headers: {} },
      response: { status: 401 },
      message: 'unauthorized',
    };

    const result = await responseRejected()(error);

    expect(getClient().post).toHaveBeenCalledWith('/auth/refresh');
    expect(error.config._retry).toBe(true);
    expect(error.config.headers.Authorization).toBe('Bearer new-at');
    expect(result.data).toEqual({ ok: true });
  });

  it('401 业务请求：重试超过 3 次终止并跳转登录', async () => {
    const error: any = {
      config: { url: '/tasks', _retryCount: 3 },
      response: { status: 401 },
      message: 'unauthorized',
    };

    await expect(responseRejected()(error)).rejects.toThrow('Token refresh failed after multiple attempts');
    expect(localStorage.getItem('access_token')).toBeNull();
    expect(window.location.href).toBe('/login');
  });

  it('401 业务请求：刷新失败时清理令牌并抛出原错误', async () => {
    getClient().post.mockRejectedValueOnce(new Error('refresh exploded'));
    const error: any = {
      config: { url: '/tasks', headers: {} },
      response: { status: 401 },
      message: 'unauthorized',
    };

    await expect(responseRejected()(error)).rejects.toThrow('refresh exploded');
    expect(localStorage.getItem('access_token')).toBeNull();
    expect(window.location.href).toBe('/login');
  });

  it('非 401 错误：开发模式下记录日志并原样抛出', async () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const error: any = { config: { url: '/tasks' }, response: { status: 500 }, message: 'server error' };

    await expect(responseRejected()(error)).rejects.toBe(error);
    consoleSpy.mockRestore();
  });
});
