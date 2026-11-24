import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import TaskList from './TaskList';
import { Task } from '../../services/indexedDB';

function makeTask(localId: string, title: string): Task {
  return {
    local_id: localId,
    user_id: 'u1',
    server_version: 0,
    title,
    description: '',
    status: 'todo',
    priority: 'medium',
    is_deleted: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    last_modified: '2026-01-01T00:00:00Z',
  } as Task;
}

describe('TaskList', () => {
  const handlers = {
    onDeleteTask: vi.fn(),
    onToggleSelect: vi.fn(),
    onEditTask: vi.fn(),
    onToggleStatus: vi.fn(),
  };

  it('空任务时渲染空态', () => {
    render(<TaskList {...handlers} tasks={[]} selectedTasks={new Set()} />);
    expect(screen.getByText('暂无任务')).toBeInTheDocument();
  });

  it('渲染全部任务卡片', () => {
    render(
      <TaskList {...handlers} tasks={[makeTask('a', '任务A'), makeTask('b', '任务B')]} selectedTasks={new Set()} />
    );
    expect(screen.getByText('任务A')).toBeInTheDocument();
    expect(screen.getByText('任务B')).toBeInTheDocument();
  });

  it('透传编辑与状态切换回调', () => {
    const tasks = [makeTask('a', '任务A')];
    render(<TaskList {...handlers} tasks={tasks} selectedTasks={new Set()} />);

    fireEvent.click(screen.getByText('任务A'));
    expect(handlers.onEditTask).toHaveBeenCalledWith(tasks[0]);
  });

  it('选中集合驱动 TaskItem 选中态', () => {
    const tasks = [makeTask('a', '任务A'), makeTask('b', '任务B')];
    const { container } = render(
      <TaskList {...handlers} tasks={tasks} selectedTasks={new Set(['a'])} />
    );
    const items = container.querySelectorAll('.task-item');
    expect(items[0].className).toContain('selected');
    expect(items[1].className).not.toContain('selected');
  });

  it('透传 onReorder 到任务卡片（手动排序）', () => {
    const onReorder = vi.fn();
    const task = makeTask('target-1', '目标卡片');
    render(<TaskList {...handlers} tasks={[task]} selectedTasks={new Set()} onReorder={onReorder} />);
    const card = screen.getByText('目标卡片').closest('.task-item') as HTMLElement;
    const dataTransfer = { getData: () => 'dragged-1', setData: vi.fn(), effectAllowed: '', dropEffect: '' };
    fireEvent.drop(card, { dataTransfer });
    expect(onReorder).toHaveBeenCalledWith('dragged-1', 'target-1');
  });
});
