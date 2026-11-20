import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import TaskFilter, { FilterState } from './TaskFilter';

const baseFilters: FilterState = {
  status: '',
  priority: '',
  search: '',
  sortBy: 'created_at',
  sortOrder: 'desc',
};

describe('TaskFilter', () => {
  it('搜索输入触发筛选变更', () => {
    const onFilterChange = vi.fn();
    render(<TaskFilter filters={baseFilters} onFilterChange={onFilterChange} />);

    fireEvent.change(screen.getByPlaceholderText('搜索任务...'), { target: { value: '关键词' } });
    expect(onFilterChange).toHaveBeenCalledWith({ ...baseFilters, search: '关键词' });
  });

  it('状态下拉变更携带原筛选', () => {
    const onFilterChange = vi.fn();
    render(<TaskFilter filters={{ ...baseFilters, search: 'abc' }} onFilterChange={onFilterChange} />);

    const selects = screen.getAllByRole('combobox');
    fireEvent.change(selects[0], { target: { value: 'done' } });
    expect(onFilterChange).toHaveBeenCalledWith({ ...baseFilters, search: 'abc', status: 'done' });
  });

  it('优先级下拉变更', () => {
    const onFilterChange = vi.fn();
    render(<TaskFilter filters={baseFilters} onFilterChange={onFilterChange} />);

    const selects = screen.getAllByRole('combobox');
    fireEvent.change(selects[1], { target: { value: 'high' } });
    expect(onFilterChange).toHaveBeenCalledWith({ ...baseFilters, priority: 'high' });
  });

  it('排序字段与排序方向变更', () => {
    const onFilterChange = vi.fn();
    render(<TaskFilter filters={baseFilters} onFilterChange={onFilterChange} />);

    const selects = screen.getAllByRole('combobox');
    fireEvent.change(selects[2], { target: { value: 'priority' } });
    expect(onFilterChange).toHaveBeenLastCalledWith({ ...baseFilters, sortBy: 'priority' });

    fireEvent.change(selects[3], { target: { value: 'asc' } });
    expect(onFilterChange).toHaveBeenLastCalledWith({ ...baseFilters, sortOrder: 'asc' });
  });

  it('重置按钮恢复默认筛选', () => {
    const onFilterChange = vi.fn();
    render(
      <TaskFilter
        filters={{ status: 'done', priority: 'high', search: 'x', sortBy: 'priority', sortOrder: 'asc' }}
        onFilterChange={onFilterChange}
      />
    );

    fireEvent.click(screen.getByRole('button', { name: '重置' }));
    expect(onFilterChange).toHaveBeenCalledWith(baseFilters);
  });
});
