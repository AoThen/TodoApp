import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import NotificationCenter from './NotificationCenter';
import type { Notification, NotificationListResponse } from '../../services/notification';

const { mockService, MockNotificationService } = vi.hoisted(() => {
  const mockService = {
    getNotifications: vi.fn(),
    markAsRead: vi.fn(),
    markAllAsRead: vi.fn(),
    deleteNotification: vi.fn(),
    clearNotifications: vi.fn(),
    getUnreadCount: vi.fn(),
  };
  return {
    mockService,
    MockNotificationService: vi.fn(() => mockService),
  };
});

vi.mock('../../services/notification', () => ({
  NotificationService: MockNotificationService,
}));

const makeNotification = (overrides: Partial<Notification> = {}): Notification => ({
  id: 1,
  user_id: 1,
  type: 'system',
  title: '通知标题',
  content: '通知内容',
  priority: 'normal',
  is_read: false,
  read_at: null,
  created_at: '2026-01-01T10:00:00Z',
  expires_at: null,
  ...overrides,
});

const makeResponse = (
  notifications: Notification[],
  pagination: Partial<NotificationListResponse['pagination']> = {}
): NotificationListResponse => ({
  notifications,
  pagination: { page: 1, page_size: 20, total: notifications.length, pages: 1, ...pagination },
});

const renderCenter = () =>
  render(<NotificationCenter onClose={vi.fn()} onUpdateCount={vi.fn()} />);

describe('NotificationCenter', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // 组件构造 NotificationService 时直接读取 window.apiService.client
    (window as any).apiService = { client: {} };
  });

  it('加载中显示 loading', async () => {
    mockService.getNotifications.mockReturnValue(new Promise(() => {}));
    renderCenter();
    expect(screen.getByText('加载中...')).toBeInTheDocument();
    await waitFor(() => expect(mockService.getNotifications).toHaveBeenCalled());
  });

  it('加载成功后渲染通知列表并回调 onUpdateCount', async () => {
    mockService.getNotifications.mockResolvedValue(
      makeResponse([makeNotification({ id: 1, title: '第一条' }), makeNotification({ id: 2, title: '第二条' })])
    );
    const onUpdateCount = vi.fn();
    render(<NotificationCenter onClose={vi.fn()} onUpdateCount={onUpdateCount} />);

    await waitFor(() => {
      expect(screen.getByText('第一条')).toBeInTheDocument();
      expect(screen.getByText('第二条')).toBeInTheDocument();
    });
    expect(mockService.getNotifications).toHaveBeenCalledWith(1, 20, {});
    expect(onUpdateCount).toHaveBeenCalled();
  });

  it('加载失败显示错误提示', async () => {
    mockService.getNotifications.mockRejectedValue(new Error('network'));
    renderCenter();
    await waitFor(() => expect(screen.getByText('加载通知失败')).toBeInTheDocument());
  });

  it('空列表显示空状态', async () => {
    mockService.getNotifications.mockResolvedValue(makeResponse([]));
    renderCenter();
    await waitFor(() => expect(screen.getByText('暂无通知')).toBeInTheDocument());
  });

  it('点击未读通知触发标记已读并更新状态', async () => {
    mockService.getNotifications.mockResolvedValue(makeResponse([makeNotification({ id: 7 })]));
    mockService.markAsRead.mockResolvedValue(undefined);
    renderCenter();

    await waitFor(() => expect(screen.getByText('通知标题')).toBeInTheDocument());
    expect(document.querySelector('.unread-indicator')).toBeInTheDocument();

    fireEvent.click(screen.getByText('通知标题').closest('.notification-item')!);

    await waitFor(() => expect(mockService.markAsRead).toHaveBeenCalledWith(7));
    await waitFor(() =>
      expect(document.querySelector('.unread-indicator')).not.toBeInTheDocument()
    );
  });

  it('点击已读通知不再触发标记已读', async () => {
    mockService.getNotifications.mockResolvedValue(
      makeResponse([makeNotification({ id: 7, is_read: true })])
    );
    renderCenter();

    await waitFor(() => expect(screen.getByText('通知标题')).toBeInTheDocument());
    fireEvent.click(screen.getByText('通知标题').closest('.notification-item')!);
    expect(mockService.markAsRead).not.toHaveBeenCalled();
  });

  it('确认后删除通知并从列表移除', async () => {
    mockService.getNotifications.mockResolvedValue(makeResponse([makeNotification({ id: 7 })]));
    mockService.deleteNotification.mockResolvedValue(undefined);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderCenter();

    await waitFor(() => expect(screen.getByText('通知标题')).toBeInTheDocument());
    fireEvent.click(screen.getByTitle('删除'));

    await waitFor(() => expect(mockService.deleteNotification).toHaveBeenCalledWith(7));
    await waitFor(() => expect(screen.queryByText('通知标题')).not.toBeInTheDocument());
  });

  it('取消确认时不删除通知', async () => {
    mockService.getNotifications.mockResolvedValue(makeResponse([makeNotification({ id: 7 })]));
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    renderCenter();

    await waitFor(() => expect(screen.getByText('通知标题')).toBeInTheDocument());
    fireEvent.click(screen.getByTitle('删除'));
    expect(mockService.deleteNotification).not.toHaveBeenCalled();
  });

  it('有未读总数时显示全部已读与清空按钮', async () => {
    mockService.getNotifications.mockResolvedValue(makeResponse([makeNotification()]));
    renderCenter();

    await waitFor(() => expect(screen.getByText('全部已读')).toBeInTheDocument());
    expect(screen.getByText('清空')).toBeInTheDocument();
  });

  it('全部已读：调用 markAllAsRead 并将所有通知置为已读', async () => {
    mockService.getNotifications.mockResolvedValue(
      makeResponse([makeNotification({ id: 1 }), makeNotification({ id: 2 })])
    );
    mockService.markAllAsRead.mockResolvedValue({ status: 'ok', marked_count: 2 });
    renderCenter();

    await waitFor(() => expect(screen.getByText('全部已读')).toBeInTheDocument());
    fireEvent.click(screen.getByText('全部已读'));

    await waitFor(() => expect(mockService.markAllAsRead).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(document.querySelector('.unread-indicator')).not.toBeInTheDocument()
    );
  });

  it('确认后清空所有通知', async () => {
    mockService.getNotifications.mockResolvedValue(makeResponse([makeNotification()]));
    mockService.clearNotifications.mockResolvedValue({ status: 'ok', cleared_count: 1 });
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderCenter();

    await waitFor(() => expect(screen.getByText('清空')).toBeInTheDocument());
    fireEvent.click(screen.getByText('清空'));

    await waitFor(() => expect(mockService.clearNotifications).toHaveBeenCalledWith(0));
    await waitFor(() => expect(screen.getByText('暂无通知')).toBeInTheDocument());
  });

  it('筛选条件变化时以新筛选重新加载并重置到第一页', async () => {
    mockService.getNotifications.mockResolvedValue(makeResponse([makeNotification()]));
    renderCenter();

    await waitFor(() => expect(mockService.getNotifications).toHaveBeenCalledTimes(1));
    fireEvent.change(screen.getAllByRole('combobox')[0], { target: { value: 'false' } });

    await waitFor(() => {
      expect(mockService.getNotifications).toHaveBeenLastCalledWith(1, 20, { read: 'false' });
    });
  });

  it('多页时渲染分页并支持翻页', async () => {
    mockService.getNotifications.mockResolvedValue(
      makeResponse([makeNotification()], { page: 1, pages: 2, total: 25 })
    );
    renderCenter();

    await waitFor(() => expect(screen.getByText(/第 1 \/ 2 页/)).toBeInTheDocument());
    expect(screen.getByText('上一页')).toBeDisabled();
    expect(screen.getByText('下一页')).toBeEnabled();

    fireEvent.click(screen.getByText('下一页'));
    await waitFor(() => {
      expect(mockService.getNotifications).toHaveBeenLastCalledWith(2, 20, {});
    });
  });

  it('单页时不渲染分页控件', async () => {
    mockService.getNotifications.mockResolvedValue(makeResponse([makeNotification()]));
    renderCenter();
    await waitFor(() => expect(screen.getByText('通知标题')).toBeInTheDocument());
    expect(screen.queryByText('上一页')).not.toBeInTheDocument();
  });

  it('点击关闭按钮触发 onClose', async () => {
    const onClose = vi.fn();
    mockService.getNotifications.mockResolvedValue(makeResponse([]));
    render(<NotificationCenter onClose={onClose} onUpdateCount={vi.fn()} />);

    await waitFor(() => expect(screen.getByTitle('关闭')).toBeInTheDocument());
    fireEvent.click(screen.getByTitle('关闭'));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
