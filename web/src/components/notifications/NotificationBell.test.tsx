import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import NotificationBell from './NotificationBell';

describe('NotificationBell', () => {
  it('count 为 0 时不显示徽标', () => {
    render(<NotificationBell count={0} onClick={vi.fn()} />);
    expect(screen.queryByText('0')).not.toBeInTheDocument();
    expect(screen.getByTitle('通知中心')).toBeInTheDocument();
  });

  it('显示未读数量徽标', () => {
    render(<NotificationBell count={5} onClick={vi.fn()} />);
    expect(screen.getByText('5')).toBeInTheDocument();
  });

  it('count 超过 99 时显示 99+', () => {
    render(<NotificationBell count={120} onClick={vi.fn()} />);
    expect(screen.getByText('99+')).toBeInTheDocument();
    expect(screen.queryByText('120')).not.toBeInTheDocument();
  });

  it('count 为 99 时显示原始数字', () => {
    render(<NotificationBell count={99} onClick={vi.fn()} />);
    expect(screen.getByText('99')).toBeInTheDocument();
  });

  it('点击铃铛触发 onClick', () => {
    const onClick = vi.fn();
    render(<NotificationBell count={3} onClick={onClick} />);
    fireEvent.click(screen.getByTitle('通知中心'));
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});
