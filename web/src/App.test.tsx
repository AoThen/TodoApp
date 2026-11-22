import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import App from './App';

vi.mock('./services/indexedDB', () => ({
  indexedDBService: {
    init: vi.fn(async () => undefined),
    getTaskByLocalId: vi.fn(async () => undefined),
    getPendingDeltas: vi.fn(async () => []),
    clearDelta: vi.fn(async () => undefined),
    updateDelta: vi.fn(async () => undefined),
  },
}));

vi.mock('./services/api', () => ({
  apiService: {
    getTasks: vi.fn(async () => ({ tasks: [], pagination: { page: 1, pages: 1, total: 0 } })),
    getCurrentUser: vi.fn(async () => ({ email: 'user@test.local' })),
    logout: vi.fn(async () => undefined),
    batchDeleteTasks: vi.fn(async () => undefined),
    restoreTask: vi.fn(async () => undefined),
    getClient: vi.fn(async () => ({ key: '', server_url: '' })),
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
});
