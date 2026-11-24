import { indexedDBService, Task, DeltaChange, Conflict } from './indexedDB';
import { apiService } from './api';

type ConflictCallback = (conflicts: Conflict[]) => void;

class SyncManager {
  private syncInProgress = false;
  private syncIntervalId: number | null = null;
  private pendingSyncTimer: number | null = null;
  private conflictListeners: ConflictCallback[] = [];

  onConflicts(callback: ConflictCallback): void {
    this.conflictListeners.push(callback);
  }

  removeConflictListener(callback: ConflictCallback): void {
    this.conflictListeners = this.conflictListeners.filter(cb => cb !== callback);
  }

  private notifyConflicts(conflicts: Conflict[]): void {
    this.conflictListeners.forEach(callback => callback(conflicts));
  }

  private scheduleDebouncedSync(): void {
    if (this.pendingSyncTimer !== null) {
      clearTimeout(this.pendingSyncTimer);
    }
    this.pendingSyncTimer = window.setTimeout(() => {
      this.sync().catch(console.error);
      this.pendingSyncTimer = null;
    }, 5000);
  }

  async sync(): Promise<void> {
    if (this.syncInProgress) {
      console.log('同步已在进行中，跳过');
      return;
    }

    this.syncInProgress = true;
    try {
      const pendingDeltas = await indexedDBService.getPendingDeltas();
      if (pendingDeltas.length === 0) {
        console.log('没有待同步的更改');
        return;
      }

      const syncMeta = await indexedDBService.getSyncMeta('current-user');
      const lastSyncAt = syncMeta?.last_sync_at || new Date(0).toISOString();

      const response = await apiService.sync(lastSyncAt, pendingDeltas);

      for (const serverChange of response.server_changes) {
        // 按服务器 ID 匹配本地任务并回写同步元数据（版本号等）。
        // 注意：不回写 title 等内容字段——本地任务可能带有待同步的未确认编辑，
        // "保留本地"重放依赖这份本地快照，被服务端内容覆盖会导致重放错误数据。
        const localTask = await indexedDBService.getTaskByServerId(serverChange.id);
        if (localTask) {
          await indexedDBService.updateTask(localTask.local_id, {
            server_id: serverChange.id,
            server_version: serverChange.server_version,
            updated_at: serverChange.updated_at,
            is_deleted: serverChange.is_deleted,
          });
        }
      }

      for (const clientChange of response.client_changes) {
        const delta = pendingDeltas.find(d => d.local_id === clientChange.local_id);
        if (delta) {
          // insert 回执：把服务端 ID/初始版本回写本地任务，
          // 后续对该任务的修改/删除才能携带正确的 id 与 client_version
          if (clientChange.op === 'insert' && clientChange.server_id > 0) {
            const taskLocalId = typeof delta.payload.local_id === 'string' ? delta.payload.local_id : '';
            if (taskLocalId) {
              try {
                await indexedDBService.updateTask(taskLocalId, {
                  server_id: clientChange.server_id,
                  server_version: 1,
                });
              } catch {
                // 本地任务已不存在（如撤销），跳过回写
              }
            }
          }
          await indexedDBService.clearDelta(delta.id!);
        }
      }

      const conflicts: Conflict[] = [];
      for (const conflict of response.conflicts) {
        await indexedDBService.addConflict({
          local_id: conflict.local_id,
          server_id: conflict.server_id,
          reason: conflict.reason,
          options: conflict.options,
          conflicts: conflict.conflicts,
          server_version: conflict.server_version,
          created_at: new Date().toISOString(),
        });
        conflicts.push({
          local_id: conflict.local_id,
          server_id: conflict.server_id,
          reason: conflict.reason,
          options: conflict.options,
          conflicts: conflict.conflicts,
          server_version: conflict.server_version,
          created_at: new Date().toISOString(),
        } as Conflict);
      }

      if (conflicts.length > 0) {
        this.notifyConflicts(conflicts);
      }

      await indexedDBService.updateSyncMeta('current-user', {
        last_sync_at: response.last_sync_at,
        last_server_version: 0,
      });

      // 通知 UI 层刷新列表（如服务端生成的循环任务、其他设备变更需要重新拉取）
      window.dispatchEvent(new CustomEvent('todoapp:synced'));
      console.log('同步成功完成');
    } catch (error) {
      console.error('同步失败:', error);
      throw error;
    } finally {
      this.syncInProgress = false;
    }
  }

  async enqueueLocalChange(
    op: 'insert' | 'update' | 'delete',
    payload: Record<string, unknown>,
    clientVersion: number
  ): Promise<void> {
    const localId = `${op}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;

    await indexedDBService.enqueueDelta({
      local_id: localId,
      op,
      payload,
      client_version: clientVersion,
    });

    this.scheduleDebouncedSync();
  }

  /**
   * 把服务端任务列表合并进本地 IndexedDB（其他设备产生的任务也因此在本端可编辑）。
   * 调用方需已完成 id → server_id 的映射。
   */
  async reconcileServerTasks(serverTasks: Task[]): Promise<void> {
    for (const task of serverTasks) {
      if (task.is_deleted) continue;
      const normalized: Task = {
        ...task,
        local_id: task.local_id || `server-${task.server_id}`,
      };
      try {
        await indexedDBService.upsertTask(normalized);
      } catch (error) {
        console.error('合并服务端任务失败:', error);
      }
    }
  }

  startAutoSync(intervalMs: number = 30000): void {
    this.stopAutoSync();
    this.syncIntervalId = window.setInterval(() => {
      this.sync().catch(console.error);
    }, intervalMs);
  }

  stopAutoSync(): void {
    if (this.syncIntervalId !== null) {
      clearInterval(this.syncIntervalId);
      this.syncIntervalId = null;
    }
    if (this.pendingSyncTimer !== null) {
      clearTimeout(this.pendingSyncTimer);
      this.pendingSyncTimer = null;
    }
  }
}

export const syncManager = new SyncManager();
