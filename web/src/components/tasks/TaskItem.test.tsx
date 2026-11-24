import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import TaskItem from './TaskItem';
import { Task } from '../../services/indexedDB';

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    local_id: 'local-1',
    user_id: 'u1',
    server_version: 1,
    title: '写周报',
    description: '本周总结',
    status: 'todo',
    priority: 'high',
    due_at: '2026-12-01T00:00:00Z',
    created_at: '2026-11-01T00:00:00Z',
    updated_at: '2026-11-01T00:00:00Z',
    is_deleted: false,
    last_modified: '2026-11-01T00:00:00Z',
    ...overrides,
  };
}

describe('TaskItem', () => {
  let onEdit: ReturnType<typeof vi.fn>;
  let onDelete: ReturnType<typeof vi.fn>;
  let onToggleSelect: ReturnType<typeof vi.fn>;
  let onToggleStatus: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    onEdit = vi.fn();
    onDelete = vi.fn();
    onToggleSelect = vi.fn();
    onToggleStatus = vi.fn();
    vi.stubGlobal('confirm', vi.fn(() => true));
  });

  it('渲染标题、备注与截止日期（取日期部分）', () => {
    render(
      <TaskItem task={makeTask()} onDelete={onDelete} onToggleSelect={onToggleSelect} onEdit={onEdit} onToggleStatus={onToggleStatus} isSelected={false} />
    );
    expect(screen.getByText('写周报')).toBeInTheDocument();
    expect(screen.getByText('本周总结')).toBeInTheDocument();
    expect(screen.getByText('截止: 2026-12-01')).toBeInTheDocument();
  });

  it('点击卡片触发 onEdit', () => {
    const task = makeTask();
    render(
      <TaskItem task={task} onDelete={onDelete} onToggleSelect={onToggleSelect} onEdit={onEdit} onToggleStatus={onToggleStatus} isSelected={false} />
    );
    fireEvent.click(screen.getByText('写周报'));
    expect(onEdit).toHaveBeenCalledWith(task);
  });

  it('勾选触发 onToggleSelect，且不冒泡到卡片', () => {
    render(
      <TaskItem task={makeTask()} onDelete={onDelete} onToggleSelect={onToggleSelect} onEdit={onEdit} onToggleStatus={onToggleStatus} isSelected={false} />
    );
    const checkbox = screen.getByRole('checkbox');
    fireEvent.click(checkbox);
    expect(onEdit).not.toHaveBeenCalled();
    fireEvent.click(checkbox, { target: { checked: true } });
    expect(onToggleSelect).toHaveBeenCalledWith('local-1', true);
  });

  it('todo 状态徽章点击触发 onToggleStatus（快速切换）', () => {
    const task = makeTask({ status: 'todo' });
    render(
      <TaskItem task={task} onDelete={onDelete} onToggleSelect={onToggleSelect} onEdit={onEdit} onToggleStatus={onToggleStatus} isSelected={false} />
    );
    fireEvent.click(screen.getByText('todo'));
    expect(onToggleStatus).toHaveBeenCalledWith(task);
    expect(onEdit).not.toHaveBeenCalled();
  });

  it('in_progress 状态徽章点击走编辑弹窗', () => {
    const task = makeTask({ status: 'in_progress' });
    render(
      <TaskItem task={task} onDelete={onDelete} onToggleSelect={onToggleSelect} onEdit={onEdit} onToggleStatus={onToggleStatus} isSelected={false} />
    );
    fireEvent.click(screen.getByText('in_progress'));
    expect(onEdit).toHaveBeenCalledWith(task);
    expect(onToggleStatus).not.toHaveBeenCalled();
  });

  it('确认后删除，且删除点击不冒泡到卡片', () => {
    const task = makeTask();
    render(
      <TaskItem task={task} onDelete={onDelete} onToggleSelect={onToggleSelect} onEdit={onEdit} onToggleStatus={onToggleStatus} isSelected={false} />
    );
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    expect(onDelete).toHaveBeenCalledWith(task);
    expect(onEdit).not.toHaveBeenCalled();
  });

  it('取消确认时不删除', () => {
    vi.stubGlobal('confirm', vi.fn(() => false));
    render(
      <TaskItem task={makeTask()} onDelete={onDelete} onToggleSelect={onToggleSelect} onEdit={onEdit} onToggleStatus={onToggleStatus} isSelected={false} />
    );
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    expect(onDelete).not.toHaveBeenCalled();
  });

  it('选中态：checkbox 勾选且容器带 selected 类', () => {
    render(
      <TaskItem task={makeTask()} onDelete={onDelete} onToggleSelect={onToggleSelect} onEdit={onEdit} onToggleStatus={onToggleStatus} isSelected={true} />
    );
    expect(screen.getByRole('checkbox')).toBeChecked();
    expect(screen.getByText('写周报').closest('.task-item')).toHaveClass('selected');
  });

  it('循环任务渲染循环角标', () => {
    render(
      <TaskItem task={makeTask({ recurrence_type: 'weekly' })} onDelete={onDelete} onToggleSelect={onToggleSelect} onEdit={onEdit} onToggleStatus={onToggleStatus} isSelected={false} />
    );
    expect(screen.getByText('↻ 每周')).toBeInTheDocument();
  });

  it('手动排序：drop 到目标卡片触发 onReorder', () => {
    const onReorder = vi.fn();
    render(
      <TaskItem task={makeTask({ local_id: 'target-1' })} onDelete={onDelete} onToggleSelect={onToggleSelect} onEdit={onEdit} onToggleStatus={onToggleStatus} isSelected={false} onReorder={onReorder} />
    );
    const card = screen.getByText('写周报').closest('.task-item') as HTMLElement;

    const dataTransfer = {
      getData: () => 'dragged-1',
      setData: vi.fn(),
      effectAllowed: '',
      dropEffect: '',
    };
    fireEvent.dragStart(card, { dataTransfer });
    fireEvent.dragOver(card, { dataTransfer });
    fireEvent.drop(card, { dataTransfer });
    expect(onReorder).toHaveBeenCalledWith('dragged-1', 'target-1');
  });

  it('未启用手动排序时卡片不可拖拽', () => {
    render(
      <TaskItem task={makeTask()} onDelete={onDelete} onToggleSelect={onToggleSelect} onEdit={onEdit} onToggleStatus={onToggleStatus} isSelected={false} />
    );
    const card = screen.getByText('写周报').closest('.task-item') as HTMLElement;
    expect(card.getAttribute('draggable')).toBeNull();
  });
});
