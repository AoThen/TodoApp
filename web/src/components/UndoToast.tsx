import React, { useEffect, useState } from 'react';
import { apiService } from '../services/api';

interface UndoToastProps {
  taskIds: number[];
  onClose: () => void;
  onUndoComplete: () => void;
}

const UndoToast: React.FC<UndoToastProps> = ({ taskIds, onClose, onUndoComplete }) => {
  const [countdown, setCountdown] = useState(30);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (taskIds.length === 0) return;

    const timer = setInterval(() => {
      setCountdown(c => {
        if (c <= 1) {
          clearInterval(timer);
          onClose();
          return 0;
        }
        return c - 1;
      });
    }, 1000);

    return () => clearInterval(timer);
  }, [taskIds, onClose]);

  const handleUndo = async () => {
    setLoading(true);
    try {
      await Promise.all(taskIds.map(id => apiService.restoreTask(id)));
      onUndoComplete();
      onClose();
    } catch (error) {
      console.error("撤销失败", error);
    } finally {
      setLoading(false);
    }
  };

  if (countdown <= 0) return null;

  return (
    <div className="undo-toast">
      <span className="undo-message">
        已删除 <strong>{taskIds.length}</strong> 个任务
      </span>
      <span className="countdown">{countdown}秒后可撤销</span>
      {!loading ? (
        <button className="btn btn-small btn-secondary" onClick={handleUndo}>
          撤销
        </button>
      ) : (
        <span className="undo-loading">撤销中...</span>
      )}
      <button className="btn btn-small" onClick={onClose}>关闭</button>
    </div>
  );
};

export default UndoToast;
