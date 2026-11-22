import React, { useState, useEffect } from 'react';
import { toast } from 'react-toastify';
import { syncManager } from '../../services/syncManager';
import { indexedDBService } from '../../services/indexedDB';

const ConflictsModal: React.FC = () => {
  const [conflicts, setConflicts] = useState<any[]>([]);
  const [show, setShow] = useState(false);

  useEffect(() => {
    const handleConflicts = (newConflicts: any[]) => {
      setConflicts(prev => [...prev, ...newConflicts]);
      setShow(true);
      toast.warning(`检测到 ${newConflicts.length} 个冲突需要解决`);
    };

    syncManager.onConflicts(handleConflicts);
    return () => {
      syncManager.removeConflictListener(handleConflicts);
    };
  }, []);

  const resolveConflict = async (index: number, resolution: 'local' | 'server') => {
    const conflict = conflicts[index];
    if (!conflict) return;

    try {
      if (resolution === 'local') {
        // 保留本地：把本地完整字段作为一次正常更新入队。
        // 冲突记录里的 local_id 是 delta 队列 ID（与任务 local_id 不同命名空间），
        // 因此按 server_id 定位本地任务；CV 优先取冲突记录附带的 server_version（合并后版本）。
        const localTask = conflict.server_id
          ? await indexedDBService.getTaskByServerId(conflict.server_id)
          : undefined;
        if (!localTask || !conflict.server_id) {
          toast.error('本地版本数据缺失，已保留服务器版本');
          await indexedDBService.deleteConflict(conflict.local_id);
        } else {
          const clientVersion = conflict.server_version ?? localTask.server_version;
          const payload: Record<string, unknown> = {
            id: conflict.server_id,
            title: localTask.title,
            description: localTask.description,
            status: localTask.status,
            priority: localTask.priority,
          };
          // 仅在本地确有截止时间时携带，避免误清服务端已设置的 due_at
          if (localTask.due_at) {
            payload.due_at = localTask.due_at;
          }
          await syncManager.enqueueLocalChange('update', payload, clientVersion);
          await indexedDBService.deleteConflict(conflict.local_id);
          toast.success('已保留本地版本');
        }
      } else {
        await indexedDBService.deleteConflict(conflict.local_id);
        toast.success('已使用服务器版本');
      }

      setConflicts(prev => prev.filter((_, i) => i !== index));

      if (conflicts.length <= 1) {
        setShow(false);
      }
    } catch (error) {
      console.error('Resolve conflict failed:', error);
      toast.error('解决冲突失败');
    }
  };

  if (!show) {
    return null;
  }

  return (
    <div className="conflicts-modal">
      <div className="conflicts-content">
        <h2>冲突解决</h2>
        {conflicts.length === 0 ? (
          <p>没有待解决的冲突</p>
        ) : (
          <div className="conflicts-list">
            {conflicts.map((conflict, index) => (
              <div key={index} className="conflict-item">
                <p><strong>冲突 #{index + 1}</strong></p>
                <p>原因: {conflict.reason}</p>
                <div className="conflict-options">
                  <button onClick={() => resolveConflict(index, 'local')}>保留本地</button>
                  <button onClick={() => resolveConflict(index, 'server')}>使用服务器</button>
                </div>
              </div>
            ))}
          </div>
        )}
        <button onClick={() => { setShow(false); setConflicts([]); }}>关闭</button>
      </div>
    </div>
  );
};

export default ConflictsModal;
