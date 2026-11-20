import { describe, it, expect, vi } from 'vitest';
import { NotificationService } from './notification';

const fakeClient = {
  get: vi.fn(),
  post: vi.fn(),
  patch: vi.fn(),
  delete: vi.fn(),
};

const makeService = () => new NotificationService(fakeClient as any);

describe('NotificationService', () => {
  it('getNotifications 拼接分页与筛选参数', async () => {
    fakeClient.get.mockResolvedValueOnce({
      data: { notifications: [], pagination: { page: 2, page_size: 20, total: 0, pages: 0 } },
    });
    await makeService().getNotifications(2, 20, { read: 'false', type: 'sync_success', priority: 'high' });
    expect(fakeClient.get).toHaveBeenCalledWith(
      '/api/v1/notifications?page=2&page_size=20&read=false&type=sync_success&priority=high'
    );
  });

  it('getNotifications 无筛选时仅携带分页参数', async () => {
    fakeClient.get.mockResolvedValueOnce({
      data: { notifications: [], pagination: { page: 1, page_size: 20, total: 0, pages: 0 } },
    });
    await makeService().getNotifications();
    expect(fakeClient.get).toHaveBeenCalledWith('/api/v1/notifications?page=1&page_size=20');
  });

  it('createNotification 以 POST 提交请求体', async () => {
    const payload = { type: 'system', title: '标题', content: '内容', priority: 'high' as const };
    fakeClient.post.mockResolvedValueOnce({ data: { id: 1, ...payload } });
    const created = await makeService().createNotification(payload);
    expect(fakeClient.post).toHaveBeenCalledWith('/api/v1/notifications', payload);
    expect(created.id).toBe(1);
  });

  it('markAsRead 调用对应 PATCH 端点', async () => {
    fakeClient.patch.mockResolvedValueOnce({});
    await makeService().markAsRead(42);
    expect(fakeClient.patch).toHaveBeenCalledWith('/api/v1/notifications/42/read');
  });

  it('markAllAsRead 返回解包后的统计', async () => {
    fakeClient.patch.mockResolvedValueOnce({ data: { status: 'ok', marked_count: 5 } });
    const result = await makeService().markAllAsRead();
    expect(fakeClient.patch).toHaveBeenCalledWith('/api/v1/notifications/read-all');
    expect(result.marked_count).toBe(5);
  });

  it('deleteNotification 调用对应 DELETE 端点', async () => {
    fakeClient.delete.mockResolvedValueOnce({});
    await makeService().deleteNotification(7);
    expect(fakeClient.delete).toHaveBeenCalledWith('/api/v1/notifications/7');
  });

  it('clearNotifications 默认保留 30 天', async () => {
    fakeClient.delete.mockResolvedValueOnce({ data: { status: 'ok', cleared_count: 3 } });
    const result = await makeService().clearNotifications();
    expect(fakeClient.delete).toHaveBeenCalledWith('/api/v1/notifications/clear?older_than_days=30');
    expect(result.cleared_count).toBe(3);
  });

  it('getUnreadCount 携带类型过滤', async () => {
    fakeClient.get.mockResolvedValueOnce({ data: { unread_count: 2 } });
    const result = await makeService().getUnreadCount(['sync_success', 'sync_conflict']);
    expect(fakeClient.get).toHaveBeenCalledWith('/api/v1/notifications/unread-count?type=sync_success,sync_conflict');
    expect(result.unread_count).toBe(2);
  });

  it('getUnreadCount 无类型时不带查询参数', async () => {
    fakeClient.get.mockResolvedValueOnce({ data: { unread_count: 0 } });
    await makeService().getUnreadCount();
    expect(fakeClient.get).toHaveBeenCalledWith('/api/v1/notifications/unread-count');
  });
});
