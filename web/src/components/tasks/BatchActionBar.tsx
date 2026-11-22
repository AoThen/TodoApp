import React from 'react';

interface BatchActionBarProps {
  selectAll: boolean;
  selectedCount: number;
  isSyncing: boolean;
  isOnline: boolean;
  onSelectAll: (checked: boolean) => void;
  onBatchDelete: () => void;
  onManualSync: () => void;
}

const BatchActionBar: React.FC<BatchActionBarProps> = ({
  selectAll,
  selectedCount,
  isSyncing,
  isOnline,
  onSelectAll,
  onBatchDelete,
  onManualSync,
}) => {
  return (
    <div className="batch-actions-bar">
      <input
        type="checkbox"
        checked={selectAll}
        onChange={(e) => onSelectAll(e.target.checked)}
      />
      <span>已选择 <strong>{selectedCount}</strong> 个任务</span>
      {selectedCount > 0 && (
        <button
          className="btn btn-danger"
          onClick={onBatchDelete}
        >
          批量删除 ({selectedCount})
        </button>
      )}
      <button
        className="btn btn-secondary"
        onClick={onManualSync}
        disabled={isSyncing || !isOnline}
      >
        {isSyncing ? '同步中...' : '立即同步'}
      </button>
    </div>
  );
};

export default BatchActionBar;
