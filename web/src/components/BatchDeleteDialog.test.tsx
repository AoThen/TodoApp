import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import BatchDeleteDialog from './BatchDeleteDialog';

describe('BatchDeleteDialog', () => {
  it('isOpen=false 时不渲染', () => {
    const { container } = render(
      <BatchDeleteDialog isOpen={false} count={3} onConfirm={vi.fn()} onCancel={vi.fn()} />
    );
    expect(container.firstChild).toBeNull();
  });

  it('显示待删除数量与撤销提示', () => {
    render(<BatchDeleteDialog isOpen={true} count={5} onConfirm={vi.fn()} onCancel={vi.fn()} />);
    expect(screen.getByText('确认批量删除')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText(/删除后30秒内可以撤销/)).toBeInTheDocument();
  });

  it('点击确定删除触发 onConfirm', async () => {
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    render(<BatchDeleteDialog isOpen={true} count={2} onConfirm={onConfirm} onCancel={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: '确定删除' }));
    await waitFor(() => expect(onConfirm).toHaveBeenCalledTimes(1));
  });

  it('点击取消触发 onCancel', () => {
    const onCancel = vi.fn();
    render(<BatchDeleteDialog isOpen={true} count={2} onConfirm={vi.fn()} onCancel={onCancel} />);
    fireEvent.click(screen.getByRole('button', { name: '取消' }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('删除进行中按钮禁用', async () => {
    let resolveConfirm!: () => void;
    const onConfirm = vi.fn().mockImplementation(() => new Promise<void>(r => { resolveConfirm = r; }));
    render(<BatchDeleteDialog isOpen={true} count={2} onConfirm={onConfirm} onCancel={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: '确定删除' }));
    expect(screen.getByText('删除中...')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '取消' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '确定删除' })).toBeDisabled();

    resolveConfirm();
    await waitFor(() => expect(screen.getByRole('button', { name: '确定删除' })).toBeEnabled());
  });
});
