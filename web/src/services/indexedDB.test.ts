import 'fake-indexeddb/auto';
import { describe, it, expect, beforeAll, beforeEach } from 'vitest';
import { indexedDBService } from './indexedDB';

let seq = 0;
function uniqueUser() {
  seq += 1;
  return `user-${seq}-${Math.random().toString(36).slice(2, 8)}`;
}

function makeTask(userId: string, title: string) {
  return {
    local_id: `${userId}-${title}-${Math.random().toString(36).slice(2, 8)}`,
    user_id: userId,
    server_version: 0,
    title,
    description: '',
    status: 'todo' as const,
    priority: 'medium' as const,
    is_deleted: false,
    last_modified: new Date().toISOString(),
  };
}

beforeAll(async () => {
  await indexedDBService.init();
});

describe('indexedDBService', () => {
  beforeEach(async () => {
    await indexedDBService.clearAllDeltas();
    await indexedDBService.clearAllConflicts();
  });

  it('addTask 自动补时间戳并可按 local_id/用户查询', async () => {
    const uid = uniqueUser();
    const task = await indexedDBService.addTask(makeTask(uid, '任务A'));

    expect(task.created_at).toBeTruthy();
    expect(task.updated_at).toBeTruthy();

    const byId = await indexedDBService.getTaskByLocalId(task.local_id);
    expect(byId?.title).toBe('任务A');

    const byUser = await indexedDBService.getTasksByUser(uid);
    expect(byUser).toHaveLength(1);

    const all = await indexedDBService.getAllTasks();
    expect(all.some(t => t.local_id === task.local_id)).toBe(true);
  });

  it('updateTask 合并字段并刷新 updated_at', async () => {
    const uid = uniqueUser();
    const task = await indexedDBService.addTask(makeTask(uid, '原标题'));
    await new Promise(r => setTimeout(r, 5));

    await indexedDBService.updateTask(task.local_id, { title: '新标题', status: 'done' });
    const updated = await indexedDBService.getTaskByLocalId(task.local_id);
    if (!updated) throw new Error('更新后的任务应存在');
    expect(updated.title).toBe('新标题');
    expect(updated.status).toBe('done');
    expect(updated.updated_at >= task.updated_at).toBe(true);
  });

  it('updateTask 不存在时抛错', async () => {
    await expect(indexedDBService.updateTask('ghost-id', { title: 'x' })).rejects.toThrow('Task not found');
  });

  it('deleteTask 软删除：is_deleted 置真但数据保留', async () => {
    const uid = uniqueUser();
    const task = await indexedDBService.addTask(makeTask(uid, '待删除'));
    await indexedDBService.deleteTask(task.local_id);

    const all = await indexedDBService.getAllTasks();
    const found = all.find(t => t.local_id === task.local_id);
    expect(found?.is_deleted).toBe(true);
  });

  it('delta 队列：入队、读取、单条清除、全清', async () => {
    const id1 = await indexedDBService.enqueueDelta({ local_id: 'l1', op: 'insert', payload: { title: 'x' }, client_version: 0 });
    await indexedDBService.enqueueDelta({ local_id: 'l2', op: 'update', payload: {}, client_version: 1 });

    let pending = await indexedDBService.getPendingDeltas();
    expect(pending).toHaveLength(2);
    expect(pending[0].timestamp).toBeTruthy();

    await indexedDBService.clearDelta(id1);
    pending = await indexedDBService.getPendingDeltas();
    expect(pending).toHaveLength(1);
    expect(pending[0].local_id).toBe('l2');

    await indexedDBService.clearAllDeltas();
    expect(await indexedDBService.getPendingDeltas()).toHaveLength(0);
  });

  it('syncMeta 合并更新且保留未覆盖字段', async () => {
    await indexedDBService.updateSyncMeta('current-user', { last_sync_at: 't1', last_server_version: 5 });
    await indexedDBService.updateSyncMeta('current-user', { last_sync_at: 't2' });

    const meta = await indexedDBService.getSyncMeta('current-user');
    expect(meta?.last_sync_at).toBe('t2');
    expect(meta?.last_server_version).toBe(5); // 未覆盖字段保留
  });

  it('冲突：增查删清', async () => {
    await indexedDBService.addConflict({ local_id: 'c1', server_id: 1, reason: 'merge', options: ['keep_server'], created_at: 't' });
    await indexedDBService.addConflict({ local_id: 'c2', server_id: 2, reason: 'keep_server', options: [], created_at: 't' });

    let conflicts = await indexedDBService.getConflicts();
    expect(conflicts).toHaveLength(2);

    await indexedDBService.deleteConflict('c1');
    conflicts = await indexedDBService.getConflicts();
    expect(conflicts.map(c => c.local_id)).toEqual(['c2']);

    await indexedDBService.clearAllConflicts();
    expect(await indexedDBService.getConflicts()).toHaveLength(0);
  });

  it('getAllData 与 importData 往返', async () => {
    const uid = uniqueUser();
    const task = await indexedDBService.addTask(makeTask(uid, '备份任务'));
    await indexedDBService.updateSyncMeta('current-user', { last_sync_at: 'backup-t', last_server_version: 9 });

    const backup = await indexedDBService.getAllData();
    expect(backup.tasks.some(t => t.local_id === task.local_id)).toBe(true);

    await indexedDBService.importData({ tasks: backup.tasks, deltas: backup.deltas, meta: backup.meta });
    const restored = await indexedDBService.getSyncMeta('current-user');
    expect(restored?.last_server_version).toBe(9);
  });
});
