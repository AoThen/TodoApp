import React, { useState } from 'react';
import { apiService } from '../services/api';

interface BatchDeleteDialogProps {
  isOpen: boolean;
  count: number;
  onConfirm: () => void;
  onCancel: () => void;
}

const BatchDeleteDialog: React.FC<BatchDeleteDialogProps> = ({ isOpen, count, onConfirm, onCancel }) => {
  const [loading, setLoading] = useState(false);

  const handleConfirm = async () => {
    setLoading(true);
    try {
      await onConfirm();
    } finally {
      setLoading(false);
    }
  };

  return isOpen && (
    <div className="modal-overlay">
      <div className="modal-content">
        <h3>确认批量删除</h3>
        <p>您确定要删除这 <strong>{count}</strong> 个任务吗？</p>
        <p className="warning">删除后30秒内可以撤销</p>

        {loading && <div className="loading-spinner">删除中...</div>}

        <div className="modal-actions">
          <button className="btn btn-secondary" onClick={onCancel} disabled={loading}>
            取消
          </button>
          <button className="btn btn-danger" onClick={handleConfirm} disabled={loading}>
            确定删除
          </button>
        </div>
      </div>
    </div>
  );
};

export default BatchDeleteDialog;
