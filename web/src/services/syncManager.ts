import { indexedDBService, Task, DeltaChange } from './indexedDB';
import { apiService } from './api';

class SyncManager {
  private syncInProgress = false;
  private syncIntervalId: number | null = null;

  async sync(): Promise<void> {
    if (this.syncInProgress) {
      console.log('同步已在进行中，跳过');
      return;
    }

    this.syncInProgress = true;
    try {
      // 获取待处理的 delta
      const pendingDeltas = await indexedDBService.getPendingDeltas();
      if (pendingDeltas.length === 0) {
        console.log('没有待同步的更改');
        return;
      }

      // 获取同步元数据
      const syncMeta = await indexedDBService.getSyncMeta('current-user');
      const lastSyncAt = syncMeta?.last_sync_at || new Date(0).toISOString();

      // 执行同步
      const response = await apiService.sync(lastSyncAt, pendingDeltas);

      // 应用服务器更改
      for (const serverChange of response.server_changes) {
        const localTask = await indexedDBService.getTaskByLocalId(serverChange.id.toString());
        if (localTask) {
          await indexedDBService.updateTask(localTask.local_id, {
            server_id: serverChange.id,
            server_version: serverChange.server_version,
            title: serverChange.title,
            updated_at: serverChange.updated_at,
            is_deleted: serverChange.is_deleted,
          });
        }
      }

      // 清除已确认的 delta
      for (const clientChange of response.client_changes) {
        const delta = pendingDeltas.find(d => d.local_id === clientChange.local_id);
        if (delta) {
          await indexedDBService.clearDelta(delta.id!);
        }
      }

      // 处理冲突
      for (const conflict of response.conflicts) {
        await indexedDBService.addConflict({
          local_id: conflict.local_id,
          server_id: conflict.server_id,
          reason: conflict.reason,
          options: conflict.options,
          created_at: new Date().toISOString(),
        });
      }

      // 更新同步元数据
      await indexedDBService.updateSyncMeta('current-user', {
        last_sync_at: response.last_sync_at,
        last_server_version: 0, // 可以跟踪实际的服务器版本
      });

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

    // 触发同步
    await this.sync();
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
  }
}

export const syncManager = new SyncManager();
