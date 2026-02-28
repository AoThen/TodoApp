import React from 'react';

export interface FilterState {
  status: string;
  priority: string;
  search: string;
  sortBy: string;
  sortOrder: string;
}

interface TaskFilterProps {
  filters: FilterState;
  onFilterChange: (filters: FilterState) => void;
}

const TaskFilter: React.FC<TaskFilterProps> = ({ filters, onFilterChange }) => {
  const handleChange = (key: keyof FilterState, value: string) => {
    onFilterChange({ ...filters, [key]: value });
  };

  return (
    <div className="task-filter">
      <div className="filter-row">
        <input
          type="text"
          placeholder="搜索任务..."
          value={filters.search}
          onChange={(e) => handleChange('search', e.target.value)}
          className="search-input"
        />
        
        <select
          value={filters.status}
          onChange={(e) => handleChange('status', e.target.value)}
          className="filter-select"
        >
          <option value="">全部状态</option>
          <option value="todo">待办</option>
          <option value="in_progress">进行中</option>
          <option value="done">已完成</option>
          <option value="archived">已归档</option>
        </select>

        <select
          value={filters.priority}
          onChange={(e) => handleChange('priority', e.target.value)}
          className="filter-select"
        >
          <option value="">全部优先级</option>
          <option value="high">高</option>
          <option value="medium">中</option>
          <option value="low">低</option>
        </select>

        <select
          value={filters.sortBy}
          onChange={(e) => handleChange('sortBy', e.target.value)}
          className="filter-select"
        >
          <option value="created_at">创建时间</option>
          <option value="updated_at">更新时间</option>
          <option value="priority">优先级</option>
        </select>

        <select
          value={filters.sortOrder}
          onChange={(e) => handleChange('sortOrder', e.target.value)}
          className="filter-select"
        >
          <option value="desc">降序</option>
          <option value="asc">升序</option>
        </select>

        <button 
          className="btn btn-secondary"
          onClick={() => onFilterChange({
            status: '',
            priority: '',
            search: '',
            sortBy: 'created_at',
            sortOrder: 'desc'
          })}
        >
          重置
        </button>
      </div>
    </div>
  );
};

export default TaskFilter;