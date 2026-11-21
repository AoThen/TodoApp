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
        await syncManager.enqueueLocalChange('update', {
          id: conflict.server_id,
          force: true,
        }, 1);
        toast.success('已保留本地版本');
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
