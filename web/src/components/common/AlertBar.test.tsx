import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import AlertBar from './AlertBar';

describe('AlertBar', () => {
  afterEach(() => vi.useRealTimers());

  it('渲染消息与类型样式', () => {
    const { container } = render(<AlertBar type="error" message="出错了" duration={null} />);
    expect(screen.getByText('出错了')).toBeInTheDocument();
    expect(container.firstElementChild?.className).toContain('bg-red-50');
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('可关闭：点击关闭按钮触发 onClose', () => {
    const onClose = vi.fn();
    render(<AlertBar type="success" message="成功" duration={null} onClose={onClose} />);

    fireEvent.click(screen.getByRole('button', { name: 'Close alert' }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('dismissible=false 时不渲染关闭按钮', () => {
    render(<AlertBar type="info" message="提示" duration={null} dismissible={false} />);
    expect(screen.queryByRole('button', { name: 'Close alert' })).not.toBeInTheDocument();
  });

  it('duration 到期自动关闭', () => {
    vi.useFakeTimers();
    const onClose = vi.fn();
    render(<AlertBar type="warning" message="警告" duration={3000} onClose={onClose} />);

    vi.advanceTimersByTime(2999);
    expect(onClose).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('duration=null 时不自动关闭', () => {
    vi.useFakeTimers();
    const onClose = vi.fn();
    render(<AlertBar type="info" message="常驻" duration={null} onClose={onClose} />);

    vi.advanceTimersByTime(60000);
    expect(onClose).not.toHaveBeenCalled();
  });

  it('showIcon=false 时不渲染图标', () => {
    const { container } = render(
      <AlertBar type="success" message="无图标" duration={null} showIcon={false} dismissible={false} />
    );
    expect(container.querySelector('svg')).toBeNull();
  });
});
