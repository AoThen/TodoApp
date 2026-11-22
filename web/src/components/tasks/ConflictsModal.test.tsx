import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import ConflictsModal from './ConflictsModal';
import * as syncModule from '../../services/syncManager';
import { syncManager } from '../../services/syncManager';
import { indexedDBService } from '../../services/indexedDB';
import { toast } from 'react-toastify';

vi.mock('react-toastify', () => ({
  toast: { warning: vi.fn(), success: vi.fn(), error: vi.fn() },
}));

vi.mock('../../services/syncManager', () => {
  const listeners: Array<(c: unknown[]) => void> = [];
  return {
    syncManager: {
      onConflicts: (cb: (c: unknown[]) => void) => { listeners.push(cb); },
      removeConflictListener: (cb: (c: unknown[]) => void) => {
        const idx = listeners.indexOf(cb);
        if (idx >= 0) listeners.splice(idx, 1);
      },
      enqueueLocalChange: vi.fn().mockResolvedValue(undefined),
    },
    __listeners: listeners,
  };
});

vi.mock('../../services/indexedDB', () => ({
  indexedDBService: {
    getTaskByServerId: vi.fn(),
    deleteConflict: vi.fn().mockResolvedValue(undefined),
  },
}));

const listeners = (syncModule as unknown as { __listeners: Array<(c: unknown[]) => void> }).__listeners;
const enqueueLocalChange = syncManager.enqueueLocalChange as ReturnType<typeof vi.fn>;
const getTaskByServerId = indexedDBService.getTaskByServerId as ReturnType<typeof vi.fn>;
const deleteConflict = indexedDBService.deleteConflict as ReturnType<typeof vi.fn>;
const toastSuccess = toast.success as ReturnType<typeof vi.fn>;
const toastError = toast.error as ReturnType<typeof vi.fn>;

// 模拟真实 ID 空间：conflict.local_id 是 delta 队列 ID，与任务 local_id 不同
const sampleConflict = {
  local_id: 'update-1763712-x9k2',
  server_id: 42,
  reason: 'intelligent_merge',
  options: ['keep_server', 'keep_client', 'merge'],
  server_version: 8,
  created_at: '2026-11-21T00:00:00Z',
};

const localTask = {
  local_id: 'local-1763711-ab',
  server_id: 42,
  user_id: 'u1',
  server_version: 8,
  title: '本地标题',
  description: '本地描述',
  status: 'in_progress' as const,
  priority: 'high' as const,
  due_at: '2026-12-01T00:00:00Z',
  created_at: '2026-11-20T00:00:00Z',
  updated_at: '2026-11-20T00:00:00Z',
  is_deleted: false,
  last_modified: '2026-11-20T00:00:00Z',
};

beforeEach(() => {
  vi.clearAllMocks();
  listeners.length = 0;
});

function renderWithConflict(conflict: unknown = sampleConflict) {
  render(<ConflictsModal />);
  act(() => {
    listeners.forEach(cb => cb([conflict]));
  });
}

describe('ConflictsModal', () => {
  it('收到冲突通知后展示弹窗与冲突信息', () => {
    renderWithConflict();
    expect(screen.getByText('冲突解决')).toBeInTheDocument();
    expect(screen.getByText(/intelligent_merge/)).toBeInTheDocument();
    expect(screen.getByText('保留本地')).toBeInTheDocument();
    expect(screen.getByText('使用服务器')).toBeInTheDocument();
  });

  it('展示字段级冲突明细（服务器值 vs 本地值），未登记字段名回退原名', () => {
    renderWithConflict({
      ...sampleConflict,
      conflicts: [
        { field_name: 'title', server_value: '服务器标题', client_value: '客户端标题' },
        { field_name: 'unknown_field', server_value: 1, client_value: 2 },
      ],
    });
    expect(screen.getByText(/标题：服务器「服务器标题」 vs 本地「客户端标题」/)).toBeInTheDocument();
    expect(screen.getByText(/unknown_field：服务器「1」 vs 本地「2」/)).toBeInTheDocument();
  });

  it('无字段级明细时不渲染明细区块', () => {
    renderWithConflict();
    expect(screen.queryByText(/服务器「/)).not.toBeInTheDocument();
  });

  it('无冲突时不渲染弹窗', () => {
    render(<ConflictsModal />);
    expect(screen.queryByText('冲突解决')).not.toBeInTheDocument();
  });

  it('保留本地时按 server_id 取本地任务，并以冲突记录的版本号完整入队', async () => {
    getTaskByServerId.mockResolvedValue(localTask);
    renderWithConflict();

    await act(async () => {
      fireEvent.click(screen.getByText('保留本地'));
    });

    expect(getTaskByServerId).toHaveBeenCalledWith(42);
    expect(enqueueLocalChange).toHaveBeenCalledTimes(1);
    const [op, payload, clientVersion] = enqueueLocalChange.mock.calls[0];
    expect(op).toBe('update');
    expect(payload).toMatchObject({
      id: 42,
      title: '本地标题',
      description: '本地描述',
      status: 'in_progress',
      priority: 'high',
      due_at: '2026-12-01T00:00:00Z',
    });
    expect(clientVersion).toBe(8);
    expect(deleteConflict).toHaveBeenCalledWith(sampleConflict.local_id);
    expect(toastSuccess).toHaveBeenCalledWith('已保留本地版本');
  });

  it('冲突记录缺少版本号时回退到本地任务的版本号', async () => {
    getTaskByServerId.mockResolvedValue({ ...localTask, server_version: 5 });
    renderWithConflict({ ...sampleConflict, server_version: undefined });

    await act(async () => {
      fireEvent.click(screen.getByText('保留本地'));
    });

    const [, , clientVersion] = enqueueLocalChange.mock.calls[0];
    expect(clientVersion).toBe(5);
  });

  it('本地无截止日期时重放不携带 due_at（避免误清服务端已设置的截止时间）', async () => {
    getTaskByServerId.mockResolvedValue({ ...localTask, due_at: undefined });
    renderWithConflict();

    await act(async () => {
      fireEvent.click(screen.getByText('保留本地'));
    });

    const [, payload] = enqueueLocalChange.mock.calls[0];
    expect(payload).not.toHaveProperty('due_at');
  });

  it('本地数据缺失时提示错误并回退为服务器版本', async () => {
    getTaskByServerId.mockResolvedValue(undefined);
    renderWithConflict();

    await act(async () => {
      fireEvent.click(screen.getByText('保留本地'));
    });

    expect(enqueueLocalChange).not.toHaveBeenCalled();
    expect(deleteConflict).toHaveBeenCalledWith(sampleConflict.local_id);
    expect(toastError).toHaveBeenCalledWith('本地版本数据缺失，已保留服务器版本');
  });

  it('使用服务器版本时仅清理冲突记录', async () => {
    renderWithConflict();

    await act(async () => {
      fireEvent.click(screen.getByText('使用服务器'));
    });

    expect(enqueueLocalChange).not.toHaveBeenCalled();
    expect(deleteConflict).toHaveBeenCalledWith(sampleConflict.local_id);
    expect(toastSuccess).toHaveBeenCalledWith('已使用服务器版本');
  });

  it('解决最后一个冲突后关闭弹窗', async () => {
    getTaskByServerId.mockResolvedValue(localTask);
    renderWithConflict();

    await act(async () => {
      fireEvent.click(screen.getByText('使用服务器'));
    });

    expect(screen.queryByText('冲突解决')).not.toBeInTheDocument();
  });
});
