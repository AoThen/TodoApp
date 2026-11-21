import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import SystemLogs from './SystemLogs';
import { adminService } from '../../services/admin';
import type { LoginLog, ActionLog } from '../../services/admin';

vi.mock('../../services/admin', () => ({
  adminService: { getLoginLogs: vi.fn(), getActionLogs: vi.fn() },
}));

const loginLogs: LoginLog[] = [
  { id: 1, email: 'admin@example.com', ip_address: '192.168.1.10', success: true, timestamp: '2026-01-01T08:00:00Z' },
  { id: 2, email: 'user@example.com', ip_address: '192.168.1.11', success: false, timestamp: '2026-01-01T09:00:00Z' },
];

const actionLogs: ActionLog[] = [
  {
    id: 10,
    admin_id: 1,
    admin_email: 'admin@example.com',
    action: 'create_user',
    target_user_id: 2,
    target_email: 'user@example.com',
    details: '创建新用户',
    ip_address: '192.168.1.10',
    timestamp: '2026-01-02T08:00:00Z',
  },
];

const mocked = vi.mocked(adminService);

describe('SystemLogs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocked.getLoginLogs.mockResolvedValue(loginLogs);
    mocked.getActionLogs.mockResolvedValue(actionLogs);
  });

  it('默认加载并渲染登录日志', async () => {
    render(<SystemLogs />);
    await waitFor(() => expect(mocked.getLoginLogs).toHaveBeenCalledTimes(1));
    expect(await screen.findByText('admin@example.com')).toBeInTheDocument();
    expect(screen.getByText('192.168.1.11')).toBeInTheDocument();
    expect(document.querySelector('.badge-active')).toHaveTextContent('成功');
    expect(document.querySelector('.badge-locked')).toHaveTextContent('失败');
    expect(mocked.getActionLogs).not.toHaveBeenCalled();
  });

  it('加载中显示 loading', () => {
    mocked.getLoginLogs.mockReturnValue(new Promise(() => {}));
    render(<SystemLogs />);
    expect(screen.getByText('加载中...')).toBeInTheDocument();
  });

  it('切换到操作日志后加载操作日志并翻译操作类型', async () => {
    render(<SystemLogs />);
    await screen.findByText('admin@example.com');

    fireEvent.click(screen.getByText('操作日志'));
    await waitFor(() => expect(mocked.getActionLogs).toHaveBeenCalledTimes(1));
    expect(await screen.findByText('创建用户', { selector: 'td' })).toBeInTheDocument();
    expect(screen.getByText('创建新用户')).toBeInTheDocument();
    expect(mocked.getLoginLogs).toHaveBeenCalledTimes(1);
  });

  it('登录日志下显示状态筛选，操作日志下显示操作筛选', async () => {
    render(<SystemLogs />);
    expect(await screen.findByDisplayValue('全部状态')).toBeInTheDocument();
    expect(screen.queryByDisplayValue('全部操作')).not.toBeInTheDocument();

    fireEvent.click(screen.getByText('操作日志'));
    expect(await screen.findByDisplayValue('全部操作')).toBeInTheDocument();
    expect(screen.queryByDisplayValue('全部状态')).not.toBeInTheDocument();
  });

  it('邮箱与状态筛选参数正确传递', async () => {
    render(<SystemLogs />);
    await screen.findByText('admin@example.com');

    fireEvent.change(screen.getByPlaceholderText('邮箱搜索'), { target: { value: 'admin' } });
    fireEvent.change(screen.getByDisplayValue('全部状态'), { target: { value: 'false' } });

    await waitFor(() => {
      expect(mocked.getLoginLogs).toHaveBeenLastCalledWith(
        expect.objectContaining({ email: 'admin', success: false })
      );
    });
  });

  it('时间范围筛选参数正确传递', async () => {
    render(<SystemLogs />);
    await screen.findByText('admin@example.com');

    fireEvent.change(document.querySelector('input[type=datetime-local]')!, {
      target: { value: '2026-01-01T00:00' },
    });
    await waitFor(() => {
      expect(mocked.getLoginLogs).toHaveBeenLastCalledWith(
        expect.objectContaining({ start_time: '2026-01-01T00:00' })
      );
    });
  });

  it('重置按钮清空筛选并重新加载', async () => {
    render(<SystemLogs />);
    await screen.findByText('admin@example.com');

    fireEvent.change(screen.getByPlaceholderText('邮箱搜索'), { target: { value: 'admin' } });
    await waitFor(() => expect(mocked.getLoginLogs).toHaveBeenCalledTimes(2));

    fireEvent.click(screen.getByText('重置'));
    await waitFor(() => expect(mocked.getLoginLogs).toHaveBeenCalledTimes(3));
    expect((screen.getByPlaceholderText('邮箱搜索') as HTMLInputElement).value).toBe('');
  });

  it('无日志时显示空状态', async () => {
    mocked.getLoginLogs.mockResolvedValue([]);
    render(<SystemLogs />);
    await waitFor(() => expect(screen.getByText('暂无日志记录')).toBeInTheDocument());
  });

  it('目标用户为空时显示占位符', async () => {
    mocked.getActionLogs.mockResolvedValue([{ ...actionLogs[0], target_email: '' }]);
    render(<SystemLogs />);
    await screen.findByText('admin@example.com');

    fireEvent.click(screen.getByText('操作日志'));
    await waitFor(() =>
      expect(screen.getAllByText('-', { selector: 'td' }).length).toBeGreaterThan(0)
    );
  });
});
