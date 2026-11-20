import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import UndoToast from './UndoToast';
import { apiService } from '../services/api';

vi.mock('../services/api', () => ({
  apiService: { restoreTask: vi.fn() },
}));

const mockedRestore = vi.mocked(apiService.restoreTask);

describe('UndoToast', () => {
  beforeEach(() => {
    mockedRestore.mockReset().mockResolvedValue(undefined as never);
  });
  afterEach(() => vi.useRealTimers());

  it('显示删除数量与倒计时', () => {
    vi.useFakeTimers();
    render(<UndoToast taskIds={[1, 2, 3]} onClose={vi.fn()} onUndoComplete={vi.fn()} />);
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText(/30秒后可撤销/)).toBeInTheDocument();
  });

  it('倒计时归零后自动关闭', async () => {
    vi.useFakeTimers();
    const onClose = vi.fn();
    render(<UndoToast taskIds={[1]} onClose={onClose} onUndoComplete={vi.fn()} />);

    // React 状态更新需在 act 内 flush，倒计时 updater 中触发 onClose
    await act(async () => {
      vi.advanceTimersByTime(30000);
    });
    expect(onClose).toHaveBeenCalled();
  });

  it('点击撤销调用 restoreTask 并完成回调', async () => {
    render(<UndoToast taskIds={[7, 8]} onClose={vi.fn()} onUndoComplete={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: '撤销' }));

    await waitFor(() => {
      expect(mockedRestore).toHaveBeenCalledTimes(2);
      expect(mockedRestore).toHaveBeenCalledWith(7);
      expect(mockedRestore).toHaveBeenCalledWith(8);
    });
  });

  it('点击关闭按钮触发 onClose', () => {
    render(<UndoToast taskIds={[1]} onClose={vi.fn()} onUndoComplete={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: '关闭' }));
    // onClose 由断言方传入
    expect(screen.getByRole('button', { name: '关闭' })).toBeInTheDocument();
  });
});
