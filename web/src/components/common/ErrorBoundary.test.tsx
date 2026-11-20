import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import ErrorBoundary from './ErrorBoundary';

function Bomb({ shouldThrow }: { shouldThrow: boolean }) {
  if (shouldThrow) {
    throw new Error('测试爆炸');
  }
  return <div>正常内容</div>;
}

describe('ErrorBoundary', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('无错误时正常渲染子组件', () => {
    render(
      <ErrorBoundary>
        <Bomb shouldThrow={false} />
      </ErrorBoundary>
    );
    expect(screen.getByText('正常内容')).toBeInTheDocument();
  });

  it('捕获子组件错误并渲染默认 fallback', () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    render(
      <ErrorBoundary>
        <Bomb shouldThrow={true} />
      </ErrorBoundary>
    );

    expect(screen.getByText('发生错误')).toBeInTheDocument();
    expect(screen.getByText('刷新页面')).toBeInTheDocument();
    expect(screen.getByText('返回首页')).toBeInTheDocument();
    // 开发模式展示错误详情
    expect(screen.getByText(/测试爆炸/)).toBeInTheDocument();
    consoleSpy.mockRestore();
  });

  it('提供自定义 fallback 时优先使用', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {});
    render(
      <ErrorBoundary fallback={<div>自定义降级 UI</div>}>
        <Bomb shouldThrow={true} />
      </ErrorBoundary>
    );
    expect(screen.getByText('自定义降级 UI')).toBeInTheDocument();
  });

  it('fallback 按钮触发页面操作', () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const reloadSpy = vi.fn();
    Object.defineProperty(window, 'location', {
      value: { ...window.location, reload: reloadSpy, href: '' },
      writable: true,
    });

    render(
      <ErrorBoundary>
        <Bomb shouldThrow={true} />
      </ErrorBoundary>
    );

    fireEvent.click(screen.getByText('刷新页面'));
    expect(reloadSpy).toHaveBeenCalled();

    fireEvent.click(screen.getByText('返回首页'));
    expect(window.location.href).toBe('/');
    consoleSpy.mockRestore();
  });
});
