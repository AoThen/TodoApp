import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import TaskForm from './TaskForm';

describe('TaskForm', () => {
  it('输入标题后提交回调并清空输入', () => {
    const onAddTask = vi.fn();
    const { container } = render(<TaskForm onAddTask={onAddTask} />);

    const input = screen.getByPlaceholderText('输入新任务...');
    fireEvent.change(input, { target: { value: '  新任务  ' } });
    fireEvent.submit(container.querySelector('form')!);

    expect(onAddTask).toHaveBeenCalledWith('新任务');
    expect((input as HTMLInputElement).value).toBe('');
  });

  it('空白标题不提交', () => {
    const onAddTask = vi.fn();
    const { container } = render(<TaskForm onAddTask={onAddTask} />);

    const input = screen.getByPlaceholderText('输入新任务...');
    fireEvent.change(input, { target: { value: '   ' } });
    fireEvent.submit(container.querySelector('form')!);

    expect(onAddTask).not.toHaveBeenCalled();
  });

  it('点击添加按钮等效提交', () => {
    const onAddTask = vi.fn();
    render(<TaskForm onAddTask={onAddTask} />);

    fireEvent.change(screen.getByPlaceholderText('输入新任务...'), { target: { value: '按钮提交' } });
    fireEvent.click(screen.getByRole('button', { name: '添加' }));

    expect(onAddTask).toHaveBeenCalledWith('按钮提交');
  });
});
