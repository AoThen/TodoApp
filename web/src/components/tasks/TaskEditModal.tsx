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

const RECURRENCE_OPTIONS: Array<{ value: string; label: string }> = [
  { value: '', label: '不重复' },
  { value: 'daily', label: '每天' },
  { value: 'weekly', label: '每周' },
  { value: 'monthly', label: '每月' },
  { value: 'weekdays', label: '工作日' },
];

// 生成循环系列 ID：同一系列的任务由服务端据此串成循环链
const generateSeriesId = (): string => {
  try {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
      return crypto.randomUUID();
    }
  } catch { /* ignore */ }
  return `series-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
};

const TaskEditModal: React.FC<TaskEditModalProps> = ({ task, onSave, onClose }) => {
  const [title, setTitle] = useState(task.title);
  const [description, setDescription] = useState(task.description || '');
  const [status, setStatus] = useState<Task['status']>(task.status);
  const [priority, setPriority] = useState<Task['priority']>(task.priority);
  const [dueAt, setDueAt] = useState(task.due_at ? task.due_at.slice(0, 10) : '');
  const [recurrenceType, setRecurrenceType] = useState(task.recurrence_type || '');

  useEffect(() => {
    setTitle(task.title);
    setDescription(task.description || '');
    setStatus(task.status);
    setPriority(task.priority);
    setDueAt(task.due_at ? task.due_at.slice(0, 10) : '');
    setRecurrenceType(task.recurrence_type || '');
  }, [task]);

  const handleSave = () => {
    if (!title.trim()) return;
    onSave({
      title: title.trim(),
      description,
      status,
      priority,
      due_at: dueAt,
      recurrence_type: recurrenceType,
      // 首次设置循环时生成系列 ID；清除循环时不携带（服务端保留旧值但规则为空即不循环）
      recurrence_series_id: recurrenceType
        ? (task.recurrence_series_id || generateSeriesId())
        : undefined,
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
            <div className="task-edit-field">
              <label>循环</label>
              <select value={recurrenceType} onChange={(e) => setRecurrenceType(e.target.value)}>
                {RECURRENCE_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
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
