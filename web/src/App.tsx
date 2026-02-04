import React, { useState, useEffect, useCallback } from 'react';
import { indexedDBService, Task } from './services/indexedDB';
import { apiService } from './services/api';
import { syncManager } from './services/syncManager';
import AdminPanel from './components/admin/AdminPanel';
import DevicePairing from './components/DevicePairing';
import BatchDeleteDialog from './components/BatchDeleteDialog';
import UndoToast from './components/UndoToast';
import ImportDialog from './components/ImportDialog';
import { toast } from 'react-toastify';
import './App.css';

const App: React.FC = () => {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [isOnline, setIsOnline] = useState(navigator.onLine);
  const [isSyncing, setIsSyncing] = useState(false);
  const [showConflicts, setShowConflicts] = useState(false);
  const [newTaskTitle, setNewTaskTitle] = useState('');
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

  const loadTasks = async () => {
    try {
      const allTasks = await indexedDBService.getAllTasks();
      setTasks(allTasks.filter(t => !t.is_deleted));
    } catch (error) {
      console.error('Failed to load tasks:', error);
    }
  };

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    const email = (e.target as HTMLFormElement).email.value;
    const password = (e.target as HTMLFormElement).password.value;

    try {
      const { access_token } = await apiService.login(email, password);
      setAuthToken(access_token);
      localStorage.setItem('user_email', email);
      await checkAdminStatus();
      await loadTasks();
      await syncManager.sync();
    } catch (error) {
      console.error('Login failed:', error);
      alert('Login failed. Please check your credentials.');
    }
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

  const handleAddTask = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newTaskTitle.trim()) return;

    const now = new Date().toISOString();
    const tempId = `local-${Date.now()}`;
    
    // Create local task
    const localTask = {
      local_id: tempId,
      user_id: 'current-user',
      server_version: 0,
      title: newTaskTitle,
      description: '',
      status: 'todo' as const,
      priority: 'medium' as const,
      is_deleted: false,
      last_modified: now,
    };

    // Save to IndexedDB
    await indexedDBService.addTask(localTask);
    setTasks(prev => [...prev, localTask as Task]);
    setNewTaskTitle('');

    // Enqueue sync
    if (authToken) {
      await syncManager.enqueueLocalChange('insert', {
        title: newTaskTitle,
        description: '',
        status: 'todo',
        priority: 'medium',
      }, 1);
    }
  };

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

  const handleDeleteTask = async (task: Task) => {
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
      setLoading(true);
      await Promise.all(undoTaskIds.map(id => apiService.restoreTask(id)));

      // 撤销后刷新任务列表
      await loadTasks();

      setUndoTaskIds([]);
    } catch (error) {
      console.error('Undo failed:', error);
      alert('撤销失败');
    } finally {
      setLoading(false);
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

  const handleExport = async (format: 'json' | 'csv') => {
    try {
      const blob = await apiService.exportTasks(format);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `tasks.${format}`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (error) {
      console.error('Export failed:', error);
      alert('Export failed');
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
    return (
      <div className="login-container">
        <h1>TodoApp Login</h1>
        <form onSubmit={handleLogin}>
          <input type="email" name="email" placeholder="Email" defaultValue="test@example.com" required />
          <input type="password" name="password" placeholder="Password" defaultValue="password" required />
          <button type="submit">Login</button>
        </form>
      </div>
    );
  }

  return (
    <div className="app">
      <header className="app-header">
        <h1>TodoApp</h1>
        <div className="status-bar">
          <span className={`status ${isOnline ? 'online' : 'offline'}`}>
            {isOnline ? 'Online' : 'Offline'}
          </span>
          {isSyncing && <span className="syncing">Syncing...</span>}
          <button onClick={handleManualSync} disabled={isSyncing || !isOnline}>
            Sync Now
          </button>
          <button onClick={() => handleExport('json')}>Export JSON</button>
          <button onClick={() => handleExport('csv')}>Export CSV</button>
          <button onClick={() => setShowImport(true)}>Import</button>
          {isAdmin && (
            <button onClick={() => setShowAdminPanel(true)}>Admin Panel</button>
          )}
          <button onClick={() => setShowPairing(true)}>Device Pairing</button>
          <button onClick={handleLogout}>Logout</button>
        </div>
      </header>

      <main className="app-main">
         <form onSubmit={handleAddTask} className="add-task-form">
           <input
             type="text"
             value={newTaskTitle}
             onChange={(e) => setNewTaskTitle(e.target.value)}
             placeholder="Add a new task..."
           />
           <button type="submit">Add Task</button>
         </form>

         {/* 批量操作工具栏 */}
         <div className="batch-actions-bar">
           <input
             type="checkbox"
             checked={selectAll}
             onChange={(e) => handleSelectAll(e.target.checked)}
           />
           <span>已选择 <strong>{selectedTasks.size}</strong> 个任务</span>
           {selectedTasks.size > 0 && (
             <>
               <button
                 className="btn btn-danger"
                 onClick={() => setShowBatchConfirm(true)}
               >
                 批量删除 ({selectedTasks.size})
               </button>
               <button
                 className="btn btn-secondary"
                 onClick={handleManualSync}
                 disabled={isSyncing || !isOnline}
               >
                 {isSyncing ? "同步中..." : "Sync Now"}
               </button>
             </>
           )}
         </div>

         <ul className="task-list">
           {tasks.map(task => (
             <li key={task.local_id} className={`task-item ${task.status}`}>
               <input
                 type="checkbox"
                 className="task-select-checkbox"
                 checked={selectedTasks.has(task.local_id)}
                 onChange={(e) => toggleTaskSelection(task.local_id, e.target.checked)}
               />
               <input
                 type="checkbox"
                 checked={task.status === 'done'}
                 onChange={() => handleToggleStatus(task)}
               />
               <span className="task-title">{task.title}</span>
               <div className="task-actions">
                 <span className={`priority ${task.priority}`}>{task.priority}</span>
                 <button onClick={() => handleDeleteTask(task)}>Delete</button>
               </div>
             </li>
           ))}
           {tasks.length === 0 && (
             <li className="no-tasks">No tasks yet. Add one above!</li>
           )}
         </ul>
       </main>

      {showConflicts && (
        <div className="conflicts-modal">
          <h2>Conflicts</h2>
          <p>Conflict resolution UI would go here</p>
          <button onClick={() => setShowConflicts(false)}>Close</button>
        </div>
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
        <ImportDialog onClose={() => setShowImport(false)} />
      )}
     </div>
  );
};

export default App;
