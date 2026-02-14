import React, { memo, useState, useCallback, FormEvent } from 'react';

interface TaskFormProps {
  onAddTask: (title: string) => void;
}

const TaskForm: React.FC<TaskFormProps> = memo(({ onAddTask }) => {
  const [title, setTitle] = useState('');

  const handleSubmit = useCallback((e: FormEvent) => {
    e.preventDefault();
    if (title.trim()) {
      onAddTask(title.trim());
      setTitle('');
    }
  }, [title, onAddTask]);

  return (
    <form onSubmit={handleSubmit} className="task-form">
      <input
        type="text"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        placeholder="输入新任务..."
        className="task-input"
      />
      <button type="submit" className="add-task-btn">添加</button>
    </form>
  );
});

TaskForm.displayName = 'TaskForm';

export default TaskForm;
