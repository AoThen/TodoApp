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

      for (const clientChange of response.client_changes) {
        const delta = pendingDeltas.find(d => d.local_id === clientChange.local_id);
        if (delta) {
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
          created_at: new Date().toISOString(),
        });
        conflicts.push({
          local_id: conflict.local_id,
          server_id: conflict.server_id,
          reason: conflict.reason,
          options: conflict.options,
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
