import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { apiService } from './api';
import { indexedDBService } from './indexedDB';
import { syncManager } from './syncManager';

vi.mock('./api', () => ({
  apiService: { sync: vi.fn() },
}));

vi.mock('./indexedDB', () => ({
  indexedDBService: {
    getPendingDeltas: vi.fn(),
    getSyncMeta: vi.fn(),
    getTaskByServerId: vi.fn(),
    upsertTask: vi.fn(),
    updateTask: vi.fn(),
    clearDelta: vi.fn(),
    addConflict: vi.fn(),
    updateSyncMeta: vi.fn(),
    enqueueDelta: vi.fn(),
  },
}));

const mockedSync = vi.mocked(apiService.sync);
const mockedDb = vi.mocked(indexedDBService);

function makeDelta(id: number, localId: string) {
  return { id, local_id: localId, op: 'update' as const, payload: {}, client_version: 1, timestamp: new Date().toISOString() };
}

describe('syncManager', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedSync.mockResolvedValue({ server_changes: [], client_changes: [], conflicts: [], last_sync_at: '2026-01-01T00:00:00Z' });
    mockedDb.getPendingDeltas.mockResolvedValue([]);
    mockedDb.getSyncMeta.mockResolvedValue(undefined);
    mockedDb.getTaskByServerId.mockResolvedValue(undefined);
  });

  afterEach(() => {
    syncManager.stopAutoSync();
  });

  it('无待同步增量时不调用服务端', async () => {
    await syncManager.sync();
    expect(mockedSync).not.toHaveBeenCalled();
  });

  it('正常同步：推送增量、清除已确认队列、更新同步位点', async () => {
    const d1 = makeDelta(1, 'local-a');
    const d2 = makeDelta(2, 'local-b');
    mockedDb.getPendingDeltas.mockResolvedValue([d1, d2]);
    mockedSync.mockResolvedValue({
      server_changes: [],
      client_changes: [
        { local_id: 'local-a', server_id: 11, op: 'update' },
        { local_id: 'local-b', server_id: 12, op: 'update' },
      ],
      conflicts: [],
      last_sync_at: '2026-01-02T00:00:00Z',
    });

    await syncManager.sync();

    expect(mockedSync).toHaveBeenCalledTimes(1);
    expect(mockedDb.clearDelta).toHaveBeenCalledWith(1);
    expect(mockedDb.clearDelta).toHaveBeenCalledWith(2);
    expect(mockedDb.updateSyncMeta).toHaveBeenCalledWith('current-user', {
      last_sync_at: '2026-01-02T00:00:00Z',
      last_server_version: 0,
    });
    expect(mockedDb.addConflict).not.toHaveBeenCalled();
  });

  it('服务端变更按 server_id 回写同步元数据并更新版本号', async () => {
    const d1 = makeDelta(1, 'local-a');
    mockedDb.getPendingDeltas.mockResolvedValue([d1]);
    mockedDb.getTaskByServerId.mockResolvedValue({ local_id: 'local-a', server_id: 99, server_version: 2, title: '旧' } as never);
    mockedSync.mockResolvedValue({
      server_changes: [{ id: 99, server_version: 3, title: '新标题', updated_at: '2026-01-02T01:00:00Z', is_deleted: false }],
      client_changes: [{ local_id: 'local-a', server_id: 99, op: 'update' }],
      conflicts: [],
      last_sync_at: '2026-01-02T01:00:00Z',
    });

    await syncManager.sync();

    expect(mockedDb.getTaskByServerId).toHaveBeenCalledWith(99);
    expect(mockedDb.updateTask).toHaveBeenCalledWith('local-a', expect.objectContaining({
      server_id: 99,
      server_version: 3,
      is_deleted: false,
    }));
  });

  it('server_changes 回写不覆盖本地内容字段（保留本地待同步编辑）', async () => {
    const d1 = makeDelta(1, 'local-a');
    mockedDb.getPendingDeltas.mockResolvedValue([d1]);
    mockedDb.getTaskByServerId.mockResolvedValue({ local_id: 'local-a', server_id: 99, server_version: 2, title: '本地待同步标题' } as never);
    mockedSync.mockResolvedValue({
      server_changes: [{ id: 99, server_version: 3, title: '服务端标题', updated_at: '2026-01-02T01:00:00Z', is_deleted: false }],
      client_changes: [{ local_id: 'local-a', server_id: 99, op: 'update' }],
      conflicts: [],
      last_sync_at: '2026-01-02T01:00:00Z',
    });

    await syncManager.sync();

    const writeback = mockedDb.updateTask.mock.calls.find(call => call[0] === 'local-a');
    expect(writeback).toBeTruthy();
    expect(writeback![1]).not.toHaveProperty('title');
  });

  it('冲突写入本地冲突表并通知监听者', async () => {
    const d1 = makeDelta(1, 'local-c');
    mockedDb.getPendingDeltas.mockResolvedValue([d1]);
    mockedSync.mockResolvedValue({
      server_changes: [],
      client_changes: [{ local_id: 'local-c', server_id: 7, op: 'update' }],
      conflicts: [{ local_id: 'local-c', server_id: 7, reason: 'intelligent_merge', options: ['keep_server', 'keep_client', 'merge'] }],
      last_sync_at: '2026-01-02T02:00:00Z',
    });

    const listener = vi.fn();
    syncManager.onConflicts(listener);
    try {
      await syncManager.sync();
      expect(mockedDb.addConflict).toHaveBeenCalledWith(expect.objectContaining({
        local_id: 'local-c',
        server_id: 7,
        reason: 'intelligent_merge',
      }));
      expect(listener).toHaveBeenCalledTimes(1);
      expect(listener.mock.calls[0][0]).toHaveLength(1);
      expect(listener.mock.calls[0][0][0].local_id).toBe('local-c');
    } finally {
      syncManager.removeConflictListener(listener);
    }
  });

  it('同步进行中重复调用会被跳过（防重入）', async () => {
    const d1 = makeDelta(1, 'local-a');
    mockedDb.getPendingDeltas.mockResolvedValue([d1]);

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let resolveSync!: (v: any) => void;
    mockedSync.mockImplementation(() => new Promise(resolve => { resolveSync = resolve; }));

    const first = syncManager.sync();
    const second = syncManager.sync(); // 此时 syncInProgress=true，应直接返回

    // 等待第一个 sync 走到 apiService.sync
    for (let i = 0; i < 10 && mockedSync.mock.calls.length === 0; i++) {
      await new Promise(r => setTimeout(r, 0));
    }
    expect(mockedSync).toHaveBeenCalledTimes(1);

    resolveSync({ server_changes: [], client_changes: [{ local_id: 'local-a', server_id: 1, op: 'update' }], conflicts: [], last_sync_at: 'x' });
    await Promise.all([first, second]);
    expect(mockedSync).toHaveBeenCalledTimes(1);
  });

  it('insert 回执把服务端 ID 与初始版本回写到本地任务', async () => {
    const insertDelta = {
      id: 1,
      local_id: 'delta-1',
      op: 'insert' as const,
      payload: { local_id: 'local-new', title: '新任务' },
      client_version: 1,
      timestamp: new Date().toISOString(),
    };
    mockedDb.getPendingDeltas.mockResolvedValue([insertDelta]);
    mockedSync.mockResolvedValue({
      server_changes: [],
      client_changes: [{ local_id: 'delta-1', server_id: 55, op: 'insert' }],
      conflicts: [],
      last_sync_at: '2026-01-03T00:00:00Z',
    });

    await syncManager.sync();

    expect(mockedDb.updateTask).toHaveBeenCalledWith('local-new', { server_id: 55, server_version: 1 });
    expect(mockedDb.clearDelta).toHaveBeenCalledWith(1);
  });

  it('reconcileServerTasks 落库服务端任务，local_id 缺失时按 server_id 合成键，跳过已删除', async () => {
    await syncManager.reconcileServerTasks([
      { local_id: 's-1', server_id: 11, server_version: 2, title: 'S1' } as never,
      { local_id: '', server_id: 12, server_version: 1, title: 'S2' } as never,
      { local_id: 's-3', server_id: 13, is_deleted: true } as never,
    ]);

    expect(mockedDb.upsertTask).toHaveBeenCalledTimes(2);
    expect(mockedDb.upsertTask).toHaveBeenCalledWith(expect.objectContaining({
      local_id: 's-1',
      server_id: 11,
    }));
    expect(mockedDb.upsertTask).toHaveBeenCalledWith(expect.objectContaining({
      local_id: 'server-12',
      server_id: 12,
    }));
  });

  it('enqueueLocalChange 写入增量队列并触发 5 秒防抖同步', async () => {
    vi.useFakeTimers();
    try {
      mockedDb.getPendingDeltas.mockResolvedValue([makeDelta(1, 'insert-x')]);
      await syncManager.enqueueLocalChange('insert', { title: '新任务' }, 0);

      expect(mockedDb.enqueueDelta).toHaveBeenCalledTimes(1);
      expect(mockedDb.enqueueDelta).toHaveBeenCalledWith(expect.objectContaining({
        op: 'insert',
        client_version: 0,
      }));
      expect(mockedSync).not.toHaveBeenCalled();

      await vi.advanceTimersByTimeAsync(5000);
      expect(mockedSync).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it('startAutoSync 按周期自动同步，stopAutoSync 后停止', async () => {
    vi.useFakeTimers();
    try {
      mockedDb.getPendingDeltas.mockResolvedValue([makeDelta(1, 'local-a')]);

      syncManager.startAutoSync(1000);
      await vi.advanceTimersByTimeAsync(1000);
      expect(mockedSync).toHaveBeenCalledTimes(1);

      syncManager.stopAutoSync();
      await vi.advanceTimersByTimeAsync(5000);
      expect(mockedSync).toHaveBeenCalledTimes(1); // 不再增加
    } finally {
      vi.useRealTimers();
    }
  });
});
