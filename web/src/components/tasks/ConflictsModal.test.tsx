import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ConflictsModal from './ConflictsModal';
import { syncManager } from '../../services/syncManager';
import { indexedDBService } from '../../services/indexedDB';
import { toast } from 'react-toastify';

vi.mock('../../services/syncManager', () => ({
  syncManager: {
    onConflicts: vi.fn(),
    removeConflictListener: vi.fn(),
    enqueueLocalChange: vi.fn(),
  },
}));

vi.mock('../../services/indexedDB', () => ({
  indexedDBService: {
    deleteConflict: vi.fn(),
  },
}));

vi.mock('react-toastify', () => ({
  toast: {
    warning: vi.fn(),
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const mockedOnConflicts = vi.mocked(syncManager.onConflicts);
const mockedRemoveListener = vi.mocked(syncManager.removeConflictListener);
const mockedEnqueue = vi.mocked(syncManager.enqueueLocalChange);
const mockedDeleteConflict = vi.mocked(indexedDBService.deleteConflict);

const conflictA = { local_id: 'local-1', server_id: 'srv-1', reason: '本地与服务端均已修改' };
const conflictB = { local_id: 'local-2', server_id: 'srv-2', reason: '服务端已删除' };

const emitConflicts = (payload: any[]) => {
  const handler = mockedOnConflicts.mock.calls.at(-1)?.[0];
  act(() => {
    handler?.(payload);
  });
};

describe('ConflictsModal', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedEnqueue.mockResolvedValue(undefined);
    mockedDeleteConflict.mockResolvedValue(undefined);
  });

  it('初始状态不渲染模态', () => {
    const { container } = render(<ConflictsModal />);

    expect(container.firstChild).toBeNull();
    expect(mockedOnConflicts).toHaveBeenCalledTimes(1);
  });

  it('卸载时移除冲突监听器', () => {
    const { unmount } = render(<ConflictsModal />);

    unmount();

    expect(mockedRemoveListener).toHaveBeenCalledTimes(1);
  });

  it('收到冲突通知后显示模态、弹出 warning 并渲染冲突项', () => {
    render(<ConflictsModal />);

    emitConflicts([conflictA]);

    expect(screen.getByText('冲突解决')).toBeInTheDocument();
    expect(screen.getByText('冲突 #1')).toBeInTheDocument();
    expect(screen.getByText(/本地与服务端均已修改/)).toBeInTheDocument();
    expect(toast.warning).toHaveBeenCalledWith('检测到 1 个冲突需要解决');
    expect(screen.getByRole('button', { name: '保留本地' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '使用服务器' })).toBeInTheDocument();
  });

  it('多次通知时冲突按序累加', () => {
    render(<ConflictsModal />);

    emitConflicts([conflictA]);
    emitConflicts([conflictB]);

    expect(screen.getByText('冲突 #2')).toBeInTheDocument();
    expect(screen.getByText(/服务端已删除/)).toBeInTheDocument();
    expect(toast.warning).toHaveBeenLastCalledWith('检测到 1 个冲突需要解决');
  });

  it('保留本地：入队强制更新并提示成功', async () => {
    const user = userEvent.setup();
    render(<ConflictsModal />);
    emitConflicts([conflictA]);

    await user.click(screen.getByRole('button', { name: '保留本地' }));

    await waitFor(() => {
      expect(mockedEnqueue).toHaveBeenCalledWith('update', { id: 'srv-1', force: true }, 1);
    });
    expect(toast.success).toHaveBeenCalledWith('已保留本地版本');
    expect(mockedDeleteConflict).not.toHaveBeenCalled();
  });

  it('使用服务器：删除本地冲突记录并提示成功', async () => {
    const user = userEvent.setup();
    render(<ConflictsModal />);
    emitConflicts([conflictA]);

    await user.click(screen.getByRole('button', { name: '使用服务器' }));

    await waitFor(() => {
      expect(mockedDeleteConflict).toHaveBeenCalledWith('local-1');
    });
    expect(toast.success).toHaveBeenCalledWith('已使用服务器版本');
    expect(mockedEnqueue).not.toHaveBeenCalled();
  });

  it('解决失败时弹出错误提示且冲突保留', async () => {
    const user = userEvent.setup();
    mockedEnqueue.mockRejectedValueOnce(new Error('network down'));
    render(<ConflictsModal />);
    emitConflicts([conflictA]);

    await user.click(screen.getByRole('button', { name: '保留本地' }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('解决冲突失败');
    });
    expect(screen.getByText('冲突 #1')).toBeInTheDocument();
  });

  it('多个冲突时解决其一后模态保持打开', async () => {
    const user = userEvent.setup();
    render(<ConflictsModal />);
    emitConflicts([conflictA, conflictB]);

    await user.click(screen.getAllByRole('button', { name: '保留本地' })[0]);

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('已保留本地版本');
    });
    expect(screen.getByText('冲突 #1')).toBeInTheDocument();
    expect(screen.queryByText(/本地与服务端均已修改/)).not.toBeInTheDocument();
    expect(screen.getByText(/服务端已删除/)).toBeInTheDocument();
  });

  it('唯一冲突解决后模态自动关闭', async () => {
    const user = userEvent.setup();
    const { container } = render(<ConflictsModal />);
    emitConflicts([conflictA]);

    await user.click(screen.getByRole('button', { name: '保留本地' }));

    await waitFor(() => {
      expect(container.firstChild).toBeNull();
    });
  });

  it('点击关闭按钮清空并隐藏模态', () => {
    const { container } = render(<ConflictsModal />);
    emitConflicts([conflictA, conflictB]);

    fireEvent.click(screen.getByRole('button', { name: '关闭' }));

    expect(container.firstChild).toBeNull();
    expect(mockedRemoveListener).not.toHaveBeenCalled();
  });
});
