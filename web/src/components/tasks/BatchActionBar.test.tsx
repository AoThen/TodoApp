import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import BatchActionBar from './BatchActionBar';

describe('BatchActionBar', () => {
  const defaultProps = {
    selectAll: false,
    selectedCount: 0,
    isSyncing: false,
    isOnline: true,
    onSelectAll: vi.fn(),
    onBatchDelete: vi.fn(),
    onManualSync: vi.fn(),
  };

  const renderBar = (props: Partial<typeof defaultProps> = {}) =>
    render(<BatchActionBar {...defaultProps} {...props} />);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('渲染全选框与已选数量文本', () => {
    renderBar({ selectedCount: 0 });

    expect(screen.getByRole('checkbox')).not.toBeChecked();
    expect(screen.getByText(/已选择/)).toBeInTheDocument();
    expect(screen.getByText('0')).toBeInTheDocument();
  });

  it('未选中任何任务时不渲染批量操作按钮', () => {
    renderBar({ selectedCount: 0 });

    expect(screen.queryByRole('button', { name: /批量删除/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Sync Now|同步中/ })).not.toBeInTheDocument();
  });

  it('选中任务后渲染批量删除与手动同步按钮', () => {
    renderBar({ selectedCount: 3 });

    expect(screen.getByRole('button', { name: '批量删除 (3)' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sync Now' })).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
  });

  it('勾选/取消全选框回调 onSelectAll', async () => {
    const user = userEvent.setup();
    const { rerender } = renderBar({ selectAll: false, selectedCount: 2 });

    await user.click(screen.getByRole('checkbox'));
    expect(defaultProps.onSelectAll).toHaveBeenCalledWith(true);

    rerender(<BatchActionBar {...defaultProps} selectAll selectedCount={2} />);
    expect(screen.getByRole('checkbox')).toBeChecked();

    await user.click(screen.getByRole('checkbox'));
    expect(defaultProps.onSelectAll).toHaveBeenLastCalledWith(false);
  });

  it('点击批量删除与手动同步按钮触发对应回调', async () => {
    const user = userEvent.setup();
    renderBar({ selectedCount: 2 });

    await user.click(screen.getByRole('button', { name: '批量删除 (2)' }));
    await user.click(screen.getByRole('button', { name: 'Sync Now' }));

    expect(defaultProps.onBatchDelete).toHaveBeenCalledTimes(1);
    expect(defaultProps.onManualSync).toHaveBeenCalledTimes(1);
  });

  it('同步进行中：按钮显示同步中且禁用', () => {
    renderBar({ selectedCount: 2, isSyncing: true });

    const syncButton = screen.getByRole('button', { name: '同步中...' });
    expect(syncButton).toBeDisabled();
  });

  it('离线状态下手动同步按钮禁用', () => {
    renderBar({ selectedCount: 2, isOnline: false });

    expect(screen.getByRole('button', { name: 'Sync Now' })).toBeDisabled();
  });

  it('在线且非同步中时手动同步按钮可用', () => {
    renderBar({ selectedCount: 2, isSyncing: false, isOnline: true });

    expect(screen.getByRole('button', { name: 'Sync Now' })).toBeEnabled();
  });

  it('checkbox 变化事件在受控模式下透传目标状态', () => {
    renderBar({ selectAll: false, selectedCount: 1 });
    const checkbox = screen.getByRole('checkbox') as HTMLInputElement;

    fireEvent.click(checkbox);

    expect(defaultProps.onSelectAll).toHaveBeenCalledWith(true);
  });
});
