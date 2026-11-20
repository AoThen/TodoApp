import React, { memo, useCallback } from 'react';
import { Task } from '../../services/indexedDB';
import TaskItem from './TaskItem';

interface TaskListProps {
  tasks: Task[];
  onDeleteTask: (task: Task) => void;
  onToggleSelect: (localId: string, checked: boolean) => void;
  onEditTask: (task: Task) => void;
  onToggleStatus: (task: Task) => void;
  selectedTasks: Set<string>;
}

const TaskList: React.FC<TaskListProps> = memo(({ tasks, onDeleteTask, onToggleSelect, onEditTask, onToggleStatus, selectedTasks }) => {
  const handleToggleSelect = useCallback((localId: string, checked: boolean) => {
    onToggleSelect(localId, checked);
  }, [onToggleSelect]);

  const handleDelete = useCallback((task: Task) => {
    onDeleteTask(task);
  }, [onDeleteTask]);

  const handleEdit = useCallback((task: Task) => {
    onEditTask(task);
  }, [onEditTask]);

  const handleToggleStatus = useCallback((task: Task) => {
    onToggleStatus(task);
  }, [onToggleStatus]);

  if (tasks.length === 0) {
    return (
      <div className="empty-state">
        <p>暂无任务</p>
      </div>
    );
  }

  return (
    <div className="task-list">
      {tasks.map((task) => (
        <TaskItem
          key={task.local_id}
          task={task}
          onDelete={handleDelete}
          onToggleSelect={handleToggleSelect}
          onEdit={handleEdit}
          onToggleStatus={handleToggleStatus}
          isSelected={selectedTasks.has(task.local_id)}
        />
      ))}
    </div>
  );
});

TaskList.displayName = 'TaskList';

export default TaskList;
