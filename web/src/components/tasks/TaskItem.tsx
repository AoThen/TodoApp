import React, { memo } from 'react';
import { Task } from '../../services/indexedDB';

interface TaskItemProps {
  task: Task;
  onDelete: (task: Task) => void;
  onToggleSelect: (localId: string, checked: boolean) => void;
  isSelected: boolean;
}

const TaskItem: React.FC<TaskItemProps> = memo(({ task, onDelete, onToggleSelect, isSelected }) => {
  const handleDelete = () => {
    const confirmed = window.confirm(`确定要删除任务"${task.title}"吗？`);
    if (!confirmed) {
      return;
    }
    onDelete(task);
  };

  return (
    <div className={`task-item ${task.is_deleted ? 'deleted' : ''} ${isSelected ? 'selected' : ''}`}>
      <input
        type="checkbox"
        checked={isSelected}
        onChange={(e) => onToggleSelect(task.local_id, e.target.checked)}
        className="task-checkbox"
      />
      <span className="task-title">{task.title}</span>
      {task.status && <span className={`task-status status-${task.status}`}>{task.status}</span>}
      {task.priority && <span className={`task-priority priority-${task.priority}`}>{task.priority}</span>}
      <button onClick={handleDelete} className="delete-btn">Delete</button>
    </div>
  );
});

TaskItem.displayName = 'TaskItem';

export default TaskItem;
