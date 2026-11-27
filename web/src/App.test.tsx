import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';
import App from './App';

vi.mock('./services/indexedDB', () => ({
  indexedDBService: {
    init: vi.fn(async () => undefined),
    getTaskByLocalId: vi.fn(async () => undefined),
    getPendingDeltas: vi.fn(async () => []),
    clearDelta: vi.fn(async () => undefined),
    updateDelta: vi.fn(async () => undefined),
    addTask: vi.fn(async () => undefined),
    updateTask: vi.fn(async () => undefined),
    deleteTask: vi.fn(async () => undefined),
  },
}));

vi.mock('./services/api', () => ({
  apiService: {
    getTasks: vi.fn(async () => ({ tasks: [], pagination: { page: 1, pages: 1, total: 0 } })),
    getCurrentUser: vi.fn(async () => ({ email: 'user@test.local' })),
    logout: vi.fn(async () => undefined),
    batchDeleteTasks: vi.fn(async () => undefined),
    restoreTask: vi.fn(async () => undefined),
    getClient: vi.fn(() => ({
      get: vi.fn(async () => ({ data: { users: [], pagination: { page: 1, page_size: 10, total: 0, pages: 0, has_prev: false, has_next: false } } })),
      post: vi.fn(async () => ({ data: {} })),
      patch: vi.fn(async () => ({ data: {} })),
      put: vi.fn(async () => ({ data: {} })),
      delete: vi.fn(async () => ({ data: {} })),
    })),
  },
}));

vi.mock('./services/syncManager', () => ({
  syncManager: {
    startAutoSync: vi.fn(),
    stopAutoSync: vi.fn(),
    sync: vi.fn(async () => undefined),
    enqueueLocalChange: vi.fn(async () => undefined),
    reconcileServerTasks: vi.fn(async () => undefined),
    onConflicts: vi.fn(),
    removeConflictListener: vi.fn(),
  },
}));

vi.mock('./services/websocket', () => ({
  websocketService: {
    connect: vi.fn(async () => undefined),
    disconnect: vi.fn(),
    onMessage: vi.fn(),
    offMessage: vi.fn(),
  },
}));

vi.mock('react-toastify', () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warn: vi.fn() },
  ToastContainer: () => null,
}));

describe('App 登录守卫', () => {
  beforeEach(() => {
    cleanup();
    localStorage.clear();
    vi.clearAllMocks();
  });

  it('未登录时渲染登录页而非任务列表', async () => {
    render(<App />);
    expect(await screen.findByRole('button', { name: /login/i })).toBeInTheDocument();
    expect(screen.queryByPlaceholderText('输入新任务...')).not.toBeInTheDocument();
  });

  it('持有访问令牌时直接进入任务主界面', async () => {
    localStorage.setItem('access_token', 'token-abc');
    render(<App />);
    expect(await screen.findByPlaceholderText('输入新任务...')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /login/i })).not.toBeInTheDocument();
  });

  it('管理员邮箱持有令牌时主界面正常渲染并恢复管理态', async () => {
    const { apiService } = await import('./services/api');
    vi.mocked(apiService.getCurrentUser).mockResolvedValue({ email: 'admin@e2e.local' } as never);
    localStorage.setItem('access_token', 'admin-token');
    render(<App />);
    expect(await screen.findByPlaceholderText('输入新任务...')).toBeInTheDocument();
    // 刷新/二次进入时也应恢复管理员标识（getCurrentUser 被调用）
    expect(vi.mocked(apiService.getCurrentUser)).toHaveBeenCalled();
  });

  it('已登录用户可添加任务', async () => {
    localStorage.setItem('access_token', 'token-add');
    render(<App />);
    const input = await screen.findByPlaceholderText('输入新任务...');
    fireEvent.change(input, { target: { value: '新任务甲' } });
    fireEvent.click(screen.getByRole('button', { name: '添加' }));

    const { indexedDBService } = await import('./services/indexedDB');
    const { syncManager } = await import('./services/syncManager');
    await waitFor(() => {
      expect(vi.mocked(indexedDBService.addTask)).toHaveBeenCalled();
    });
    expect(vi.mocked(syncManager.enqueueLocalChange)).toHaveBeenCalled();
  });

  it('已登录用户可完成任务与删除任务', async () => {
    const { apiService } = await import('./services/api');
    vi.mocked(apiService.getTasks).mockResolvedValue({
      tasks: [
        {
          id: 1,
          local_id: 'l1',
          title: '任务甲',
          description: '',
          status: 'todo',
          priority: 'medium',
          is_deleted: false,
          server_version: 1,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
        },
      ],
      pagination: { page: 1, pages: 1, total: 1 },
    } as never);
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    localStorage.setItem('access_token', 'token-ops');
    render(<App />);
    expect(await screen.findByText('任务甲')).toBeInTheDocument();

    // 点击状态文本完成任务
    fireEvent.click(screen.getByText('todo'));
    const { indexedDBService } = await import('./services/indexedDB');
    await waitFor(() => {
      expect(vi.mocked(indexedDBService.updateTask)).toHaveBeenCalled();
    });

    // 确认后删除任务
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    await waitFor(() => {
      expect(vi.mocked(indexedDBService.deleteTask)).toHaveBeenCalled();
    });
    expect(confirmSpy).toHaveBeenCalled();
    confirmSpy.mockRestore();
  });

  it('已登录用户登出后回到登录页', async () => {
    const { apiService } = await import('./services/api');
    // 真实 logout 会在 finally 中清除本地令牌，mock 同步该副作用
    vi.mocked(apiService.logout).mockImplementation(async () => {
      localStorage.removeItem('access_token');
    });
    localStorage.setItem('access_token', 'token-out');
    render(<App />);
    expect(await screen.findByPlaceholderText('输入新任务...')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '登出' }));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /login/i })).toBeInTheDocument();
    });
    expect(vi.mocked(apiService.logout)).toHaveBeenCalled();
    expect(localStorage.getItem('access_token')).toBeNull();
  });
});
