import React, { useState, useEffect } from 'react';
import { Task } from '../../services/indexedDB';
import './TaskEditModal.css';

interface TaskEditModalProps {
  task: Task;
  onSave: (updates: Partial<Task>) => void;
  onClose: () => void;
}

const STATUS_OPTIONS: Array<{ value: Task['status']; label: string }> = [
  { value: 'todo', label: '待办' },
  { value: 'in_progress', label: '进行中' },
  { value: 'done', label: '已完成' },
  { value: 'archived', label: '已归档' },
];

const PRIORITY_OPTIONS: Array<{ value: Task['priority']; label: string }> = [
  { value: 'low', label: '低' },
  { value: 'medium', label: '中' },
  { value: 'high', label: '高' },
];

const TaskEditModal: React.FC<TaskEditModalProps> = ({ task, onSave, onClose }) => {
  const [title, setTitle] = useState(task.title);
  const [description, setDescription] = useState(task.description || '');
  const [status, setStatus] = useState<Task['status']>(task.status);
  const [priority, setPriority] = useState<Task['priority']>(task.priority);
  const [dueAt, setDueAt] = useState(task.due_at ? task.due_at.slice(0, 10) : '');

  useEffect(() => {
    setTitle(task.title);
    setDescription(task.description || '');
    setStatus(task.status);
    setPriority(task.priority);
    setDueAt(task.due_at ? task.due_at.slice(0, 10) : '');
  }, [task]);

  const handleSave = () => {
    if (!title.trim()) return;
    onSave({
      title: title.trim(),
      description,
      status,
      priority,
      due_at: dueAt,
    });
  };

  return (
    <div className="task-edit-overlay" onClick={onClose}>
      <div className="task-edit-modal" onClick={(e) => e.stopPropagation()}>
        <div className="task-edit-header">
          <h2>编辑任务</h2>
          <button className="task-edit-close" onClick={onClose} aria-label="关闭">×</button>
        </div>
        <div className="task-edit-body">
          <div className="task-edit-field">
            <label>标题</label>
            <input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="任务标题"
              autoFocus
            />
          </div>
          <div className="task-edit-field">
            <label>备注</label>
            <textarea
              rows={4}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="补充说明…"
            />
          </div>
          <div className="task-edit-row">
            <div className="task-edit-field">
              <label>状态</label>
              <select value={status} onChange={(e) => setStatus(e.target.value as Task['status'])}>
                {STATUS_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
            </div>
            <div className="task-edit-field">
              <label>优先级</label>
              <select value={priority} onChange={(e) => setPriority(e.target.value as Task['priority'])}>
                {PRIORITY_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
            </div>
            <div className="task-edit-field">
              <label>截止日期</label>
              <input
                type="date"
                value={dueAt}
                onChange={(e) => setDueAt(e.target.value)}
              />
            </div>
          </div>
        </div>
        <div className="task-edit-footer">
          <button className="task-edit-btn" onClick={onClose}>取消</button>
          <button className="task-edit-btn primary" onClick={handleSave} disabled={!title.trim()}>
            保存
          </button>
        </div>
      </div>
    </div>
  );
};

export default TaskEditModal;
