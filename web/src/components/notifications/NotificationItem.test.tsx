import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import NotificationItem from './NotificationItem';
import { Notification } from '../../services/notification';

function makeNotification(overrides: Partial<Notification> = {}): Notification {
  return {
    id: 1,
    type: 'system',
    title: '系统通知',
    content: '您有一条新消息',
    priority: 'high',
    is_read: false,
    created_at: '2026-01-01T08:30:00Z',
    ...overrides,
  } as Notification;
}

describe('NotificationItem', () => {
  it('渲染标题、内容与优先级图标', () => {
    render(<NotificationItem notification={makeNotification()} onMarkAsRead={vi.fn()} onDelete={vi.fn()} />);
    expect(screen.getByText('系统通知')).toBeInTheDocument();
    expect(screen.getByText('您有一条新消息')).toBeInTheDocument();
    expect(screen.getByText('🟠')).toBeInTheDocument(); // high 优先级
    expect(screen.getByText('未读')).toBeInTheDocument();
  });

  it('未知优先级显示默认图标', () => {
    render(
      <NotificationItem
        notification={makeNotification({ priority: 'critical' as unknown as Notification['priority'] })}
        onMarkAsRead={vi.fn()}
        onDelete={vi.fn()}
      />
    );
    expect(screen.getByText('⚪')).toBeInTheDocument();
  });

  it('未读通知点击后标记已读', () => {
    const onMarkAsRead = vi.fn();
    render(
      <NotificationItem notification={makeNotification({ is_read: false })} onMarkAsRead={onMarkAsRead} onDelete={vi.fn()} />
    );
    fireEvent.click(screen.getByText('系统通知'));
    expect(onMarkAsRead).toHaveBeenCalledWith(1);
  });

  it('已读通知点击不再标记', () => {
    const onMarkAsRead = vi.fn();
    render(
      <NotificationItem notification={makeNotification({ is_read: true })} onMarkAsRead={onMarkAsRead} onDelete={vi.fn()} />
    );
    fireEvent.click(screen.getByText('系统通知'));
    expect(onMarkAsRead).not.toHaveBeenCalled();
  });

  it('删除按钮阻止冒泡并触发 onDelete', () => {
    const onDelete = vi.fn();
    const onMarkAsRead = vi.fn();
    render(
      <NotificationItem notification={makeNotification()} onMarkAsRead={onMarkAsRead} onDelete={onDelete} />
    );
    fireEvent.click(screen.getByTitle('删除'));
    expect(onDelete).toHaveBeenCalledWith(1);
    expect(onMarkAsRead).not.toHaveBeenCalled();
  });
});
