import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import TaskEditModal from './TaskEditModal';
import { Task } from '../../services/indexedDB';

const baseTask: Task = {
  local_id: 'local-9',
  user_id: 'u1',
  server_version: 2,
  title: '旧标题',
  description: '旧备注',
  status: 'todo',
  priority: 'medium',
  due_at: '2026-12-01T00:00:00Z',
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-01T00:00:00Z',
  is_deleted: false,
  last_modified: '2026-11-01T00:00:00Z',
};

function setup(overrides: Partial<Task> = {}) {
  const onSave = vi.fn();
  const onClose = vi.fn();
  render(<TaskEditModal task={{ ...baseTask, ...overrides }} onSave={onSave} onClose={onClose} />);
  return { onSave, onClose };
}

describe('TaskEditModal', () => {
  it('以任务现有值初始化表单（due_at 取日期部分）', () => {
    setup();
    expect(screen.getByDisplayValue('旧标题')).toBeInTheDocument();
    expect(screen.getByDisplayValue('旧备注')).toBeInTheDocument();
    expect(screen.getByDisplayValue('2026-12-01')).toBeInTheDocument();
  });

  it('编辑后保存回调携带合并字段', () => {
    const { onSave } = setup();
    fireEvent.change(screen.getByDisplayValue('旧标题'), { target: { value: '新标题' } });
    // 状态/优先级是 select（role=combobox）：[0]=状态，[1]=优先级
    fireEvent.change(screen.getAllByRole('combobox')[0], { target: { value: 'done' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onSave).toHaveBeenCalledWith({
      title: '新标题',
      description: '旧备注',
      status: 'done',
      priority: 'medium',
      due_at: '2026-12-01',
    });
  });

  it('标题为空白时保存按钮禁用', () => {
    setup({ title: '   ' });
    expect(screen.getByRole('button', { name: '保存' })).toBeDisabled();
  });

  it('点击遮罩关闭弹窗，点击弹窗内部不关闭', () => {
    const { onClose } = setup();
    fireEvent.click(screen.getByRole('button', { name: '保存' })); // 内部点击
    expect(onClose).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText('编辑任务').closest('.task-edit-overlay') as HTMLElement);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('点击取消与关闭按钮均触发 onClose', () => {
    const { onClose } = setup();
    fireEvent.click(screen.getByRole('button', { name: '取消' }));
    fireEvent.click(screen.getByRole('button', { name: '关闭' }));
    expect(onClose).toHaveBeenCalledTimes(2);
  });
});
