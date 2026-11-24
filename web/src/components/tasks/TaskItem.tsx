import React, { memo } from 'react';
import { Task } from '../../services/indexedDB';

interface TaskItemProps {
  task: Task;
  onDelete: (task: Task) => void;
  onToggleSelect: (localId: string, checked: boolean) => void;
  onEdit: (task: Task) => void;
  onToggleStatus: (task: Task) => void;
  isSelected: boolean;
  /** 手动排序（拖拽）：仅手动排序模式传入 */
  onReorder?: (draggedLocalId: string, targetLocalId: string) => void;
}

const RECURRENCE_LABELS: Record<string, string> = {
  daily: '每天',
  weekly: '每周',
  monthly: '每月',
  weekdays: '工作日',
};

const TaskItem: React.FC<TaskItemProps> = memo(({ task, onDelete, onToggleSelect, onEdit, onToggleStatus, isSelected, onReorder }) => {
  const handleDelete = (e: React.MouseEvent) => {
    e.stopPropagation();
    const confirmed = window.confirm(`确定要删除任务"${task.title}"吗？`);
    if (!confirmed) {
      return;
    }
    onDelete(task);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const draggedLocalId = e.dataTransfer.getData('text/plain');
    if (draggedLocalId && draggedLocalId !== task.local_id) {
      onReorder?.(draggedLocalId, task.local_id);
    }
  };

  const dueLabel = task.due_at ? task.due_at.slice(0, 10) : '';
  const recurrenceLabel = task.recurrence_type ? RECURRENCE_LABELS[task.recurrence_type] : '';

  const dragProps = onReorder
    ? {
        draggable: true,
        onDragStart: (e: React.DragEvent) => {
          e.dataTransfer.setData('text/plain', task.local_id);
          e.dataTransfer.effectAllowed = 'move';
        },
        onDragOver: (e: React.DragEvent) => {
          e.preventDefault();
          e.dataTransfer.dropEffect = 'move';
        },
        onDrop: handleDrop,
      }
    : {};

  return (
    <div
      className={`task-item ${task.is_deleted ? 'deleted' : ''} ${isSelected ? 'selected' : ''}`}
      onClick={() => onEdit(task)}
      {...dragProps}
    >
      <input
        type="checkbox"
        checked={isSelected}
        onChange={(e) => onToggleSelect(task.local_id, e.target.checked)}
        onClick={(e) => e.stopPropagation()}
        className="task-checkbox"
      />
      <div className="task-main">
        <span className="task-title">{task.title}</span>
        {task.description && <div className="task-desc">{task.description}</div>}
        {dueLabel && <div className="task-due">截止: {dueLabel}</div>}
        {recurrenceLabel && (
          <div className="task-recurrence" title="循环任务">↻ {recurrenceLabel}</div>
        )}
      </div>
      {task.status && (
        <span
          className={`task-status status-${task.status}`}
          title={task.status === 'todo' || task.status === 'done' ? '点击切换 待办/已完成' : '点击编辑'}
          onClick={(e) => {
            e.stopPropagation();
            // 仅 待办/已完成 支持快速切换，其余状态走编辑弹窗
            if (task.status === 'todo' || task.status === 'done') {
              onToggleStatus(task);
            } else {
              onEdit(task);
            }
          }}
        >
          {task.status}
        </span>
      )}
      {task.priority && <span className={`task-priority priority-${task.priority}`}>{task.priority}</span>}
      <button onClick={handleDelete} className="delete-btn">Delete</button>
    </div>
  );
});

TaskItem.displayName = 'TaskItem';

export default TaskItem;
