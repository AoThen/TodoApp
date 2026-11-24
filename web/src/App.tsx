import React, { useState, useEffect, useCallback } from 'react';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { indexedDBService, Task } from './services/indexedDB';
import { apiService } from './services/api';
import { syncManager } from './services/syncManager';
import { websocketService } from './services/websocket';
import AdminPanel from './components/admin/AdminPanel';
import DevicePairing from './components/DevicePairing';
import BatchDeleteDialog from './components/BatchDeleteDialog';
import UndoToast from './components/UndoToast';
import ImportDialog from './components/ImportDialog';
import ErrorBoundary from './components/common/ErrorBoundary';
import Header from './components/layout/Header';
import LoginPage from './components/auth/LoginPage';
import TaskList from './components/tasks/TaskList';
import TaskForm from './components/tasks/TaskForm';
import TaskEditModal from './components/tasks/TaskEditModal';
import TaskFilter, { FilterState } from './components/tasks/TaskFilter';
import Pagination from './components/tasks/Pagination';
import BatchActionBar from './components/tasks/BatchActionBar';
import ConflictsModal from './components/tasks/ConflictsModal';
import NotificationSystem from './components/NotificationSystem';
import { toast } from 'react-toastify';
import './App.css';

// 服务端数据统一交由 React Query 管理拉取、缓存与重试
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
      staleTime: 10_000,
    },
  },
});

// 服务端任务 DTO：主键字段为 id，本地模型使用 server_id
type ServerTaskDTO = Task & { id?: number };

const toClientTask = (t: ServerTaskDTO): Task => {
  const { id, ...rest } = t;
  return { ...rest, server_id: id ?? rest.server_id };
};

const AppContent: React.FC = () => {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [isOnline, setIsOnline] = useState(navigator.onLine);
  const [isSyncing, setIsSyncing] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [authToken, setAuthToken] = useState<string | null>(null);
  const [isAdmin, setIsAdmin] = useState(false);
  const [showAdminPanel, setShowAdminPanel] = useState(false);
  const [showPairing, setShowPairing] = useState(false);
  const [showImport, setShowImport] = useState(false);
  const [showBatchConfirm, setShowBatchConfirm] = useState(false);
  const [undoTaskIds, setUndoTaskIds] = useState<number[]>([]);
  const [selectedTasks, setSelectedTasks] = useState<Set<string>>(new Set());
  const [selectAll, setSelectAll] = useState(false);
  const [editingTask, setEditingTask] = useState<Task | null>(null);
  const [page, setPage] = useState(1);
  const [pagination, setPagination] = useState<{ page: number; pages: number; total: number } | null>(null);
  const PAGE_SIZE = 20;
  const [filters, setFilters] = useState<FilterState>({
    status: '',
    priority: '',
    search: '',
    sortBy: 'created_at',
    sortOrder: 'desc',
  });

  // Initialize IndexedDB and load data
  useEffect(() => {
    const initializeApp = async () => {
      try {
        await indexedDBService.init();

        // Check for existing auth token
        const token = localStorage.getItem('access_token');
        if (token) {
          setAuthToken(token);
          // 刷新/二次进入时也要恢复管理员标识，否则管理入口在已登录刷新后消失
          checkAdminStatus();
        }

        // Start auto-sync
        syncManager.startAutoSync(30000);
      } catch (error) {
        console.error('Failed to initialize app:', error);
      } finally {
        setIsLoading(false);
      }
    };

    initializeApp();

    // Online/offline listeners
    const handleOnline = () => setIsOnline(true);
    const handleOffline = () => setIsOnline(false);

    window.addEventListener('online', handleOnline);
    window.addEventListener('offline', handleOffline);

    return () => {
      window.removeEventListener('online', handleOnline);
      window.removeEventListener('offline', handleOffline);
    };
  }, []);

  // 服务端任务列表：随登录态、分页与筛选条件自动拉取
  const tasksQuery = useQuery({
    queryKey: ['tasks', authToken, page, filters],
    queryFn: () => apiService.getTasks({
      page,
      page_size: PAGE_SIZE,
      status: filters.status || undefined,
      priority: filters.priority || undefined,
      search: filters.search || undefined,
      sort_by: filters.sortBy,
      sort_order: filters.sortOrder,
    }),
    enabled: !!authToken,
  });

  useEffect(() => {
    if (tasksQuery.error) {
      console.error('Failed to load tasks:', tasksQuery.error);
    }
  }, [tasksQuery.error]);

  useEffect(() => {
    if (authToken && tasksQuery.data) {
      // 映射服务端主键并合并进 IndexedDB，
      // 使其他设备创建的任务在本端也能编辑/删除并参与冲突回放
      const mapped = tasksQuery.data.tasks
        .filter((t: ServerTaskDTO) => !t.is_deleted)
        .map(toClientTask);
      setTasks(mapped);
      setPagination(tasksQuery.data.pagination || null);
      syncManager.reconcileServerTasks(mapped).catch(() => {});
    }
  }, [authToken, tasksQuery.data]);

  // 服务器侧数据变更后刷新任务列表缓存（撤销/导入/手动同步等）
  const refreshTasks = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ['tasks'] });
  }, []);

  // 同步成功后刷新列表缓存：服务端生成的循环任务、其他设备变更需要重新拉取
  useEffect(() => {
    const onSynced = () => refreshTasks();
    window.addEventListener('todoapp:synced', onSynced);
    return () => window.removeEventListener('todoapp:synced', onSynced);
  }, [refreshTasks]);

  // 筛选/搜索/排序变化时重置回第 1 页
  const handleFilterChange = (next: FilterState) => {
    setFilters(next);
    setPage(1);
  };

  const handleLoginSuccess = async (token: string) => {
    setAuthToken(token);
    await checkAdminStatus();
    await syncManager.sync();
  };

  const checkAdminStatus = async () => {
    try {
      const usersResponse = await apiService.getCurrentUser();
      localStorage.setItem('user_email', usersResponse.email);
      setIsAdmin(usersResponse.email.includes('admin') || usersResponse.email === 'admin@example.com');
    } catch (error) {
      setIsAdmin(false);
    }
  };

  // 服务端任务变更统一入队入口：入队前从 IndexedDB 读取最新同步元数据
  // （insert 回执/版本回写只落 IndexedDB，React state 可能滞后一个同步周期）
  const enqueueTaskChange = useCallback(async (
    task: Task,
    op: 'update' | 'delete',
    payload: Record<string, unknown>
  ) => {
    if (!authToken) return;
    const fresh = await indexedDBService.getTaskByLocalId(task.local_id);
    const serverId = fresh?.server_id ?? task.server_id;
    if (!serverId) {
      // 服务端 ID 未知（如离线创建尚未同步）：把变更合并进待发送的 insert 增量，
      // 避免产生无法上报的孤儿更新；删除则直接取消该 insert
      const pending = await indexedDBService.getPendingDeltas();
      const insertDelta = pending.find(d => d.op === 'insert' && d.payload.local_id === task.local_id);
      if (!insertDelta?.id) return;
      if (op === 'delete') {
        await indexedDBService.clearDelta(insertDelta.id);
      } else {
        await indexedDBService.updateDelta(insertDelta.id, payload);
      }
      return;
    }
    await syncManager.enqueueLocalChange(op, { ...payload, id: serverId }, fresh?.server_version ?? task.server_version);
  }, [authToken]);

  const handleAddTaskFromForm = useCallback(async (title: string) => {
    if (!title.trim()) return;

    const now = new Date().toISOString();
    const tempId = `local-${Date.now()}`;
    
    const localTask = {
      local_id: tempId,
      user_id: 'current-user',
      server_version: 0,
      title: title,
      description: '',
      status: 'todo' as const,
      priority: 'medium' as const,
      is_deleted: false,
      last_modified: now,
    };

    await indexedDBService.addTask(localTask);
    setTasks(prev => [...prev, localTask as Task]);

    if (authToken) {
      await syncManager.enqueueLocalChange('insert', {
        local_id: tempId,
        title: title,
        status: 'todo',
        priority: 'medium',
      }, 1);
    }
  }, [authToken]);

  const handleToggleStatus = async (task: Task) => {
    const newStatus = task.status === 'done' ? 'todo' : 'done';
    const now = new Date().toISOString();
    
    await indexedDBService.updateTask(task.local_id, {
      status: newStatus,
      completed_at: newStatus === 'done' ? now : undefined,
    });

    setTasks(prev => prev.map(t => 
      t.local_id === task.local_id 
        ? { ...t, status: newStatus, completed_at: newStatus === 'done' ? now : t.completed_at }
        : t
    ));

    if (authToken) {
      // client_version 语义：客户端所见版本号（服务端据此做乐观并发校验）
      await enqueueTaskChange(task, 'update', { status: newStatus });
    }
  };

  const handleSaveTask = async (updates: Partial<Task>) => {
    if (!editingTask) return;
    const now = new Date().toISOString();

    const patch: Partial<Task> = { ...updates, last_modified: now };
    if (updates.status === 'done' && editingTask.status !== 'done') {
      patch.completed_at = now;
    } else if (updates.status && updates.status !== 'done') {
      patch.completed_at = undefined;
    }

    // 本地立即生效（offline-first），再入 delta 队列同步
    await indexedDBService.updateTask(editingTask.local_id, patch);
    setTasks(prev => prev.map(t =>
      t.local_id === editingTask.local_id ? { ...t, ...patch } : t
    ));
    setEditingTask(null);

    if (authToken) {
      await enqueueTaskChange(editingTask, 'update', { ...updates });
    }
  };

  // 手动排序：拖拽落点以中点插入法计算新 sort_order，仅产生一条 update delta；
  // 服务端分页/筛选下取可视相邻项中点仍等价于全局正确位置
  const handleReorderTasks = useCallback(async (draggedLocalId: string, targetLocalId: string) => {
    if (draggedLocalId === targetLocalId) return;
    const fromIndex = tasks.findIndex(t => t.local_id === draggedLocalId);
    const toIndex = tasks.findIndex(t => t.local_id === targetLocalId);
    if (fromIndex < 0 || toIndex < 0) return;

    const next = [...tasks];
    const [moved] = next.splice(fromIndex, 1);
    next.splice(toIndex, 0, moved);

    const prev = next[toIndex - 1];
    const follow = next[toIndex + 1];
    let newSortOrder: number;
    if (prev && follow) {
      newSortOrder = ((prev.sort_order ?? 0) + (follow.sort_order ?? 0)) / 2;
    } else if (prev) {
      newSortOrder = (prev.sort_order ?? 0) + 1000;
    } else if (follow) {
      newSortOrder = (follow.sort_order ?? 0) - 1000;
    } else {
      newSortOrder = 0;
    }

    const patched: Task = { ...moved, sort_order: newSortOrder };
    setTasks(next.map(t => (t.local_id === moved.local_id ? patched : t)));

    // 本地立即生效（offline-first），再入 delta 队列同步
    await indexedDBService.updateTask(moved.local_id, { sort_order: newSortOrder });
    if (authToken) {
      await enqueueTaskChange(patched, 'update', { sort_order: newSortOrder });
    }
  }, [tasks, authToken, enqueueTaskChange]);

  const handleDeleteTask = async (task: Task) => {
    const confirmed = window.confirm(`确定要删除任务"${task.title}"吗？`);
    if (!confirmed) {
      return;
    }
    
    await indexedDBService.deleteTask(task.local_id);
    setTasks(prev => prev.filter(t => t.local_id !== task.local_id));

    if (authToken) {
      await enqueueTaskChange(task, 'delete', {});
    }
  };

  const toggleTaskSelection = (localId: string, checked: boolean) => {
    const newSelected = new Set(selectedTasks);
    if (checked) {
      newSelected.add(localId);
    } else {
      newSelected.delete(localId);
      setSelectAll(false);
    }
    setSelectedTasks(newSelected);
  };

  const handleSelectAll = (checked: boolean) => {
    setSelectAll(checked);
    if (checked) {
      const allIds = tasks.map(t => t.local_id);
      setSelectedTasks(new Set(allIds));
    } else {
      setSelectedTasks(new Set());
    }
  };

  const handleBatchDelete = async () => {
    if (!authToken) {
      alert('Please login first');
      setShowBatchConfirm(false);
      return;
    }

    const selectedTasksArr = Array.from(selectedTasks);
    if (selectedTasksArr.length === 0) {
      alert('请先选择要删除的任务');
      return;
    }

    const tasksToDelete = tasks.filter(t => selectedTasks.has(t.local_id));
    const taskIds = tasksToDelete.map(t => t.server_id).filter((id): id is number => typeof id === 'number') as number[];

    try {
      setIsLoading(true);
      const response = await apiService.batchDeleteTasks(taskIds);
      setShowBatchConfirm(false);

      // 从IndexedDB和state中移除任务
      await Promise.all(tasksToDelete.map(t => indexedDBService.deleteTask(t.local_id)));
      setTasks(prev => prev.filter(t => !selectedTasksArr.includes(t.local_id)));

      // 设置可撤销的taskIds
      setUndoTaskIds(taskIds);
      setSelectedTasks(new Set());
      setSelectAll(false);

    } catch (error: unknown) {
      console.error('Batch delete failed:', error);
      alert('批量删除失败');
      setShowBatchConfirm(false);
    } finally {
      setIsLoading(false);
    }
  };

  const handleUndo = async () => {
    try {
      setIsLoading(true);
      await Promise.all(undoTaskIds.map(id => apiService.restoreTask(id)));

      // 撤销后刷新任务列表
      refreshTasks();

      setUndoTaskIds([]);
    } catch (error) {
      console.error('Undo failed:', error);
      alert('撤销失败');
    } finally {
      setIsLoading(false);
    }
  };

  const handleManualSync = async () => {
    if (!authToken) {
      alert('Please login first');
      return;
    }

    setIsSyncing(true);
    try {
      await syncManager.sync();
      refreshTasks();
    } catch (error) {
      console.error('Sync failed:', error);
      alert('Sync failed. Please try again.');
    } finally {
      setIsSyncing(false);
    }
  };

  const handleImport = async () => {
    if (!authToken) {
      alert('Please login first');
      return;
    }
    setShowImport(true);
  };

  const handleLogout = async () => {
    websocketService.disconnect();
    await apiService.logout();
    setAuthToken(null);
    setTasks([]);
    setIsAdmin(false);
    setShowAdminPanel(false);
  };

  if (isLoading) {
    return <div className="loading">Loading...</div>;
  }

  if (showAdminPanel) {
    return <AdminPanel onBack={() => setShowAdminPanel(false)} />;
  }

  if (!authToken) {
    return <LoginPage onSuccess={handleLoginSuccess} />;
  }

  return (
    <div className="app">
      <Header
        isOnline={isOnline}
        isSyncing={isSyncing}
        onLogout={handleLogout}
        onOpenAdmin={() => setShowAdminPanel(true)}
        onOpenPairing={() => setShowPairing(true)}
        onOpenImport={() => setShowImport(true)}
        isAdmin={isAdmin}
        bell={authToken ? <NotificationSystem token={authToken} /> : undefined}
      />

      <main className="app-main">
        <TaskFilter filters={filters} onFilterChange={handleFilterChange} />
        <TaskForm onAddTask={handleAddTaskFromForm} />

        {pagination && (
          <Pagination
            page={page}
            pages={pagination.pages}
            total={pagination.total}
            onChange={setPage}
          />
        )}

        <BatchActionBar
          selectAll={selectAll}
          selectedCount={selectedTasks.size}
          isSyncing={isSyncing}
          isOnline={isOnline}
          onSelectAll={handleSelectAll}
          onBatchDelete={() => setShowBatchConfirm(true)}
          onManualSync={handleManualSync}
        />

        <TaskList
          tasks={tasks}
          onDeleteTask={handleDeleteTask}
          onToggleSelect={toggleTaskSelection}
          onEditTask={setEditingTask}
          onToggleStatus={handleToggleStatus}
          selectedTasks={selectedTasks}
          onReorder={filters.sortBy === 'manual' ? handleReorderTasks : undefined}
        />
      </main>

      {editingTask && (
        <TaskEditModal
          task={editingTask}
          onSave={handleSaveTask}
          onClose={() => setEditingTask(null)}
        />
      )}

      {showPairing && (
        <DevicePairing onClose={() => setShowPairing(false)} />
      )}

      {showBatchConfirm && (
        <BatchDeleteDialog
          isOpen={showBatchConfirm}
          count={selectedTasks.size}
          onConfirm={handleBatchDelete}
          onCancel={() => setShowBatchConfirm(false)}
        />
      )}

      {undoTaskIds.length > 0 && (
        <UndoToast
          taskIds={undoTaskIds}
          onClose={() => setUndoTaskIds([])}
          onUndoComplete={async () => {
            refreshTasks();
            toast.success('已撤销删除');
          }}
        />
      )}

      {showImport && (
        <ImportDialog
          isOpen={showImport}
          onClose={() => setShowImport(false)}
          onImportSuccess={refreshTasks}
        />
      )}
    </div>
  );
};

const App: React.FC = () => {
  return (
    <QueryClientProvider client={queryClient}>
      <ErrorBoundary>
        <AppContent />
        {/* 常驻挂载：保证 syncManager 冲突监听在任意视图（含登录页/加载中）下都生效 */}
        <ConflictsModal />
      </ErrorBoundary>
    </QueryClientProvider>
  );
};

export default App;
