import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import BatchActionBar from './BatchActionBar';

const baseProps = {
  selectAll: false,
  selectedCount: 0,
  isSyncing: false,
  isOnline: true,
  onSelectAll: vi.fn(),
  onBatchDelete: vi.fn(),
  onManualSync: vi.fn(),
};

describe('BatchActionBar', () => {
  it('未选择任务时只显示全选框与计数', () => {
    render(<BatchActionBar {...baseProps} />);
    expect(screen.getByText(/已选择/)).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('全选勾选触发 onSelectAll(true)', () => {
    const onSelectAll = vi.fn();
    render(<BatchActionBar {...baseProps} onSelectAll={onSelectAll} />);
    fireEvent.click(screen.getByRole('checkbox'));
    expect(onSelectAll).toHaveBeenCalledWith(true);
  });

  it('取消全选触发 onSelectAll(false)', () => {
    const onSelectAll = vi.fn();
    render(<BatchActionBar {...baseProps} selectAll onSelectAll={onSelectAll} />);
    fireEvent.click(screen.getByRole('checkbox'));
    expect(onSelectAll).toHaveBeenCalledWith(false);
  });

  it('选中任务后显示批量删除与立即同步按钮', () => {
    render(<BatchActionBar {...baseProps} selectedCount={3} />);
    expect(screen.getByText('批量删除 (3)')).toBeInTheDocument();
    expect(screen.getByText('立即同步')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
  });

  it('点击批量删除触发 onBatchDelete', () => {
    const onBatchDelete = vi.fn();
    render(<BatchActionBar {...baseProps} selectedCount={2} onBatchDelete={onBatchDelete} />);
    fireEvent.click(screen.getByText('批量删除 (2)'));
    expect(onBatchDelete).toHaveBeenCalledTimes(1);
  });

  it('点击立即同步触发 onManualSync', () => {
    const onManualSync = vi.fn();
    render(<BatchActionBar {...baseProps} selectedCount={1} onManualSync={onManualSync} />);
    fireEvent.click(screen.getByText('立即同步'));
    expect(onManualSync).toHaveBeenCalledTimes(1);
  });

  it('同步中或离线时立即同步按钮禁用', () => {
    const { rerender } = render(<BatchActionBar {...baseProps} selectedCount={1} isSyncing />);
    expect(screen.getByText('同步中...')).toBeDisabled();

    rerender(<BatchActionBar {...baseProps} selectedCount={1} isOnline={false} />);
    expect(screen.getByText('立即同步')).toBeDisabled();
  });
});
