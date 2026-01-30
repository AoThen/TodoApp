import { openDB, DBSchema, IDBPDatabase } from 'idb';

interface Task {
  local_id: string;
  server_id?: number;
  user_id: string;
  server_version: number;
  title: string;
  description: string;
  status: 'todo' | 'in_progress' | 'done' | 'archived';
  priority: 'low' | 'medium' | 'high';
  due_at?: string;
  created_at: string;
  updated_at: string;
  completed_at?: string;
  is_deleted: boolean;
  last_modified: string;
}

interface DeltaChange {
  id?: number;
  local_id: string;
  op: 'insert' | 'update' | 'delete';
  payload: Record<string, unknown>;
  client_version: number;
  timestamp: string;
}

interface SyncMeta {
  user_id: string;
  last_sync_at?: string;
  last_server_version: number;
}

interface Conflict {
  id?: number;
  local_id: string;
  server_id: number;
  reason: string;
  options: string[];
  created_at: string;
}

interface TodoAppDB extends DBSchema {
  tasks: {
    key: string;
    value: Task;
    indexes: {
      'by-user-id': string;
      'by-status': string;
      'by-due-at': string;
      'by-last-modified': string;
    };
  };
  delta_queue: {
    key: number;
    value: DeltaChange;
    indexes: {
      'by-timestamp': string;
    };
  };
  sync_meta: {
    key: string;
    value: SyncMeta;
  };
  conflicts: {
    key: number;
    value: Conflict;
  };
}

const DB_NAME = 'todoapp-db';
const DB_VERSION = 1;

class IndexedDBService {
  private db: IDBPDatabase<TodoAppDB> | null = null;

  async init(): Promise<void> {
    if (this.db) return;

    this.db = await openDB<TodoAppDB>(DB_NAME, DB_VERSION, {
      upgrade(db) {
        // Tasks store
        if (!db.objectStoreNames.contains('tasks')) {
          const taskStore = db.createObjectStore('tasks', { keyPath: 'local_id' });
          taskStore.createIndex('by-user-id', 'user_id');
          taskStore.createIndex('by-status', 'status');
          taskStore.createIndex('by-due-at', 'due_at');
          taskStore.createIndex('by-last-modified', 'last_modified');
        }

        // Delta queue store
        if (!db.objectStoreNames.contains('delta_queue')) {
          const deltaStore = db.createObjectStore('delta_queue', { keyPath: 'id', autoIncrement: true });
          deltaStore.createIndex('by-timestamp', 'timestamp');
        }

        // Sync meta store
        if (!db.objectStoreNames.contains('sync_meta')) {
          db.createObjectStore('sync_meta', { keyPath: 'user_id' });
        }

        // Conflicts store
        if (!db.objectStoreNames.contains('conflicts')) {
          db.createObjectStore('conflicts', { keyPath: 'id', autoIncrement: true });
        }
      },
    });
  }

  // Task operations
  async addTask(task: Omit<Task, 'created_at' | 'updated_at'>): Promise<Task> {
    const now = new Date().toISOString();
    const newTask: Task = {
      ...task,
      created_at: now,
      updated_at: now,
    };
    await this.db!.put('tasks', newTask);
    return newTask;
  }

  async updateTask(localId: string, updates: Partial<Task>): Promise<void> {
    const task = await this.db!.get('tasks', localId);
    if (!task) throw new Error('Task not found');
    
    const updatedTask: Task = {
      ...task,
      ...updates,
      updated_at: new Date().toISOString(),
    };
    await this.db!.put('tasks', updatedTask);
  }

  async deleteTask(localId: string): Promise<void> {
    await this.updateTask(localId, { is_deleted: true });
  }

  async getTasksByUser(userId: string): Promise<Task[]> {
    return this.db!.getAllFromIndex('tasks', 'by-user-id', userId);
  }

  async getTaskByLocalId(localId: string): Promise<Task | undefined> {
    return this.db!.get('tasks', localId);
  }

  async getAllTasks(): Promise<Task[]> {
    return this.db!.getAll('tasks');
  }

  // Delta queue operations
  async enqueueDelta(change: Omit<DeltaChange, 'id' | 'timestamp'>): Promise<number> {
    const delta: DeltaChange = {
      ...change,
      timestamp: new Date().toISOString(),
    };
    return this.db!.add('delta_queue', delta);
  }

  async getPendingDeltas(): Promise<DeltaChange[]> {
    return this.db!.getAll('delta_queue');
  }

  async clearDelta(id: number): Promise<void> {
    await this.db!.delete('delta_queue', id);
  }

  async clearAllDeltas(): Promise<void> {
    await this.db!.clear('delta_queue');
  }

  // Sync meta operations
  async getSyncMeta(userId: string): Promise<SyncMeta | undefined> {
    return this.db!.get('sync_meta', userId);
  }

  async updateSyncMeta(userId: string, meta: Partial<SyncMeta>): Promise<void> {
    const existing = await this.getSyncMeta(userId);
    await this.db!.put('sync_meta', { ...existing, user_id: userId, ...meta });
  }

  // Conflict operations
  async addConflict(conflict: Omit<Conflict, 'id'>): Promise<number> {
    return this.db!.add('conflicts', conflict);
  }

  async getConflicts(): Promise<Conflict[]> {
    return this.db!.getAll('conflicts');
  }

  async clearConflict(id: number): Promise<void> {
    await this.db!.delete('conflicts', id);
  }

  async clearAllConflicts(): Promise<void> {
    await this.db!.clear('conflicts');
  }

  // Utility operations
  async getAllData(): Promise<{
    tasks: Task[];
    deltas: DeltaChange[];
    meta: SyncMeta | undefined;
    conflicts: Conflict[];
  }> {
    return {
      tasks: await this.getAllTasks(),
      deltas: await this.getPendingDeltas(),
      meta: await this.db!.get('sync_meta', 'current-user'),
      conflicts: await this.getConflicts(),
    };
  }

  async importData(data: {
    tasks: Task[];
    deltas?: DeltaChange[];
    meta?: SyncMeta;
  }): Promise<void> {
    const tx = this.db!.transaction(['tasks', 'delta_queue', 'sync_meta'], 'readwrite');
    await Promise.all([
      ...data.tasks.map(task => tx.objectStore('tasks').put(task)),
      ...(data.deltas || []).map(delta => tx.objectStore('delta_queue').add(delta)),
      data.meta ? tx.objectStore('sync_meta').put(data.meta) : Promise.resolve(),
    ]);
    await tx.done;
  }
}

export const indexedDBService = new IndexedDBService();
export type { Task, DeltaChange, SyncMeta, Conflict };
