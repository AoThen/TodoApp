import React, { useState, useEffect, useCallback } from 'react';
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
          await loadTasks();
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

  useEffect(() => {
    if (authToken) {
      loadTasks();
    }
  }, [page, filters, authToken]);

  const loadTasks = async () => {
  try {
    if (authToken) {
      const response = await apiService.getTasks({
        page,
        page_size: PAGE_SIZE,
        status: filters.status || undefined,
        priority: filters.priority || undefined,
        search: filters.search || undefined,
        sort_by: filters.sortBy,
        sort_order: filters.sortOrder,
      });
      setTasks(response.tasks.filter((t: any) => !t.is_deleted));
      setPagination(response.pagination || null);
    } else {
      const allTasks = await indexedDBService.getAllTasks();
      setTasks(allTasks.filter(t => !t.is_deleted));
      setPagination(null);
    }
  } catch (error) {
    console.error('Failed to load tasks:', error);
  }
};

  // 筛选/搜索/排序变化时重置回第 1 页
  const handleFilterChange = (next: FilterState) => {
    setFilters(next);
    setPage(1);
  };

  const handleLoginSuccess = async (token: string) => {
    setAuthToken(token);
    await checkAdminStatus();
    await loadTasks();
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
      await syncManager.enqueueLocalChange('update', {
        id: task.server_id,
        status: newStatus,
      }, task.server_version + 1);
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

    if (authToken && editingTask.server_id) {
      await syncManager.enqueueLocalChange('update', {
        id: editingTask.server_id,
        ...updates,
      }, editingTask.server_version + 1);
    }
  };

  const handleDeleteTask = async (task: Task) => {
    const confirmed = window.confirm(`确定要删除任务"${task.title}"吗？`);
    if (!confirmed) {
      return;
    }
    
    await indexedDBService.deleteTask(task.local_id);
    setTasks(prev => prev.filter(t => t.local_id !== task.local_id));

    if (authToken && task.server_id) {
      await syncManager.enqueueLocalChange('delete', {
        id: task.server_id,
      }, task.server_version + 1);
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
      await loadTasks();

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
      await loadTasks();
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
             await loadTasks();
             toast.success('已撤销删除');
           }}
         />
       )}

{showImport && (
           <ImportDialog
             isOpen={showImport}
             onClose={() => setShowImport(false)}
             onImportSuccess={loadTasks}
           />
         )}
      </div>
   );
};

const App: React.FC = () => {
  return (
    <ErrorBoundary>
      <AppContent />
      {/* 常驻挂载：保证 syncManager 冲突监听在任意视图（含登录页/加载中）下都生效 */}
      <ConflictsModal />
    </ErrorBoundary>
  );
};

export default App;
