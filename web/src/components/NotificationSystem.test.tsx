import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act, waitFor } from '@testing-library/react';
import NotificationSystem from './NotificationSystem';

const { mockService, MockNotificationService, mockWs, mockApiService, mockToast } = vi.hoisted(() => {
  const mockService = {
    getNotifications: vi.fn(),
    getUnreadCount: vi.fn(),
    markAsRead: vi.fn(),
    markAllAsRead: vi.fn(),
    deleteNotification: vi.fn(),
    clearNotifications: vi.fn(),
  };
  return {
    mockService,
    MockNotificationService: vi.fn(() => mockService),
    mockWs: {
      connect: vi.fn(),
      onMessage: vi.fn(),
      offMessage: vi.fn(),
      disconnect: vi.fn(),
    },
    mockApiService: {
      getClient: vi.fn(() => ({})),
    },
    mockToast: {
      info: vi.fn(),
      success: vi.fn(),
      error: vi.fn(),
      warning: vi.fn(),
    },
  };
});

vi.mock('../services/notification', () => ({
  NotificationService: MockNotificationService,
}));
vi.mock('../services/websocket', () => ({
  websocketService: mockWs,
}));
vi.mock('../services/api', () => ({
  apiService: mockApiService,
}));
vi.mock('react-toastify', () => ({ toast: mockToast }));

// NotificationCenter 被 mock，避免其内部再次实例化服务干扰断言
vi.mock('./notifications/NotificationCenter', () => ({
  default: ({ onClose }: { onClose: () => void }) => (
    <div data-testid="notification-center">
      <button onClick={onClose}>center-close</button>
    </div>
  ),
}));

describe('NotificationSystem', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockService.getUnreadCount.mockResolvedValue({ unread_count: 3 });
  });
  afterEach(() => vi.useRealTimers());

  it('挂载后建立 WebSocket 连接、注册消息处理器并拉取未读数', async () => {
    render(<NotificationSystem token="token-1" />);
    await waitFor(() => expect(mockWs.connect).toHaveBeenCalledWith('token-1', true));
    expect(mockWs.onMessage).toHaveBeenCalledWith('notification', expect.any(Function));
    await waitFor(() => expect(mockService.getUnreadCount).toHaveBeenCalled());
    expect(mockApiService.getClient).toHaveBeenCalled();
  });

  it('token 为空时不建立连接', () => {
    render(<NotificationSystem token="" />);
    expect(mockWs.connect).not.toHaveBeenCalled();
    expect(mockWs.onMessage).not.toHaveBeenCalled();
  });

  it('未读数渲染到铃铛徽标', async () => {
    render(<NotificationSystem token="token-1" />);
    await waitFor(() => expect(screen.getByText('3')).toBeInTheDocument());
  });

  it('卸载时注销消息处理器并断开连接', async () => {
    const { unmount } = render(<NotificationSystem token="token-1" />);
    await waitFor(() => expect(mockWs.onMessage).toHaveBeenCalled());
    unmount();
    expect(mockWs.offMessage).toHaveBeenCalledWith('notification');
    expect(mockWs.disconnect).toHaveBeenCalledTimes(1);
  });

  it('每 60 秒定时刷新未读数，卸载后停止', async () => {
    vi.useFakeTimers();
    const { unmount } = render(<NotificationSystem token="token-1" />);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(mockService.getUnreadCount).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(60000);
    });
    expect(mockService.getUnreadCount).toHaveBeenCalledTimes(2);

    unmount();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(120000);
    });
    expect(mockService.getUnreadCount).toHaveBeenCalledTimes(2);
  });

  it('收到 WebSocket 通知消息时刷新未读数并弹出提示', async () => {
    render(<NotificationSystem token="token-1" />);
    await waitFor(() => expect(mockWs.onMessage).toHaveBeenCalled());
    const handler = mockWs.onMessage.mock.calls[0][1] as (m: { data?: Record<string, string> }) => void;
    const callsBefore = mockService.getUnreadCount.mock.calls.length;

    act(() => {
      handler({ data: { title: '新任务', content: '请查收' } });
    });
    await waitFor(() => {
      expect(mockService.getUnreadCount.mock.calls.length).toBe(callsBefore + 1);
    });
    expect(mockToast.info).toHaveBeenCalledWith('新任务: 请查收');
  });

  it('无内容的通知消息只显示标题', async () => {
    render(<NotificationSystem token="token-1" />);
    await waitFor(() => expect(mockWs.onMessage).toHaveBeenCalled());
    const handler = mockWs.onMessage.mock.calls[0][1] as (m: { data?: Record<string, string> }) => void;

    act(() => {
      handler({ data: { title: '仅标题' } });
    });
    await waitFor(() => expect(mockToast.info).toHaveBeenCalledWith('仅标题'));
  });

  it('消息缺少 data 时忽略', async () => {
    render(<NotificationSystem token="token-1" />);
    await waitFor(() => expect(mockWs.onMessage).toHaveBeenCalled());
    const handler = mockWs.onMessage.mock.calls[0][1] as (m: { data?: unknown }) => void;
    const callsBefore = mockService.getUnreadCount.mock.calls.length;

    act(() => {
      handler({});
    });
    expect(mockService.getUnreadCount.mock.calls.length).toBe(callsBefore);
    expect(mockToast.info).not.toHaveBeenCalled();
  });

  it('WebSocket 连接失败不阻断渲染', async () => {
    mockWs.connect.mockRejectedValue(new Error('ws down'));
    render(<NotificationSystem token="token-1" />);
    await waitFor(() => expect(mockWs.connect).toHaveBeenCalled());
    expect(screen.getByTitle('通知中心')).toBeInTheDocument();
  });

  it('点击铃铛打开通知中心并刷新未读数', async () => {
    render(<NotificationSystem token="token-1" />);
    await waitFor(() => expect(screen.getByTitle('通知中心')).toBeInTheDocument());
    const callsBefore = mockService.getUnreadCount.mock.calls.length;

    fireEvent.click(screen.getByTitle('通知中心'));

    expect(screen.getByTestId('notification-center')).toBeInTheDocument();
    await waitFor(() => {
      expect(mockService.getUnreadCount.mock.calls.length).toBe(callsBefore + 1);
    });
  });

  it('点击遮罩或中心关闭按钮关闭通知中心', async () => {
    render(<NotificationSystem token="token-1" />);
    await waitFor(() => expect(screen.getByTitle('通知中心')).toBeInTheDocument());
    fireEvent.click(screen.getByTitle('通知中心'));
    expect(screen.getByTestId('notification-center')).toBeInTheDocument();

    fireEvent.click(screen.getByText('center-close'));
    expect(screen.queryByTestId('notification-center')).not.toBeInTheDocument();
  });
});
