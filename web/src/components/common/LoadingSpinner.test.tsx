import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import LoadingSpinner from './LoadingSpinner';

describe('LoadingSpinner', () => {
  it('渲染加载指示器（role=status）', () => {
    render(<LoadingSpinner />);
    const indicators = screen.getAllByRole('status');
    expect(indicators.length).toBeGreaterThan(0);
    expect(indicators[0]).toHaveAttribute('aria-label', 'Loading');
  });

  it('尺寸与颜色 class 正确应用', () => {
    const { container } = render(<LoadingSpinner size="large" color="danger" />);
    const spinner = container.querySelector('.animate-spin');
    expect(spinner?.className).toContain('w-12');
    expect(spinner?.className).toContain('border-red-600');
  });

  it('显示自定义文案', () => {
    render(<LoadingSpinner text="加载中..." />);
    expect(screen.getByText('加载中...')).toBeInTheDocument();
  });

  it('fullScreen 模式使用固定遮罩', () => {
    const { container } = render(<LoadingSpinner fullScreen />);
    expect(container.querySelector('.fixed.inset-0')).toBeInTheDocument();
  });
});
