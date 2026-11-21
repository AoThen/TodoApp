import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import AdminPanel from './AdminPanel';
import { adminService } from '../../services/admin';
import type { User } from '../../services/admin';

vi.mock('../../services/admin', () => ({
  adminService: { getUsers: vi.fn() },
}));

vi.mock('./UserManagement', () => ({ default: () => <div data-testid="tab-users">用户管理内容</div> }));
vi.mock('./SystemLogs', () => ({ default: () => <div data-testid="tab-logs">系统日志内容</div> }));
vi.mock('./SystemConfig', () => ({ default: () => <div data-testid="tab-config">系统配置内容</div> }));

const makeUser = (overrides: Partial<User> = {}): User => ({
  id: 1,
  email: 'admin@example.com',
  role: 'admin',
  failed_attempts: 0,
  locked_until: null,
  must_change_password: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  is_locked: false,
  ...overrides,
});

const mockedGetUsers = vi.mocked(adminService.getUsers);

describe('AdminPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.setItem('user_email', 'admin@example.com');
  });

  it('权限校验期间显示加载中', () => {
    mockedGetUsers.mockReturnValue(new Promise(() => {}));
    render(<AdminPanel onBack={vi.fn()} />);
    expect(screen.getByText('加载中...')).toBeInTheDocument();
  });

  it('当前用户是管理员时渲染管理面板与默认用户管理标签', async () => {
    mockedGetUsers.mockResolvedValue({ users: [makeUser()], pagination: {} as never });
    render(<AdminPanel onBack={vi.fn()} />);

    await waitFor(() => expect(screen.getByText('管理面板')).toBeInTheDocument());
    expect(screen.getByTestId('tab-users')).toBeInTheDocument();
    expect(screen.queryByTestId('tab-logs')).not.toBeInTheDocument();
  });

  it('当前用户非管理员时显示无权限提示', async () => {
    localStorage.setItem('user_email', 'user@example.com');
    mockedGetUsers.mockResolvedValue({
      users: [makeUser(), makeUser({ id: 2, email: 'user@example.com', role: 'user' })],
      pagination: {} as never,
    });
    render(<AdminPanel onBack={vi.fn()} />);

    await waitFor(() => expect(screen.getByText('您没有管理员权限')).toBeInTheDocument());
  });

  it('权限校验失败时显示错误提示', async () => {
    mockedGetUsers.mockRejectedValue(new Error('network'));
    render(<AdminPanel onBack={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('无法验证管理员权限')).toBeInTheDocument());
  });

  it('标签页切换渲染对应子面板', async () => {
    mockedGetUsers.mockResolvedValue({ users: [makeUser()], pagination: {} as never });
    render(<AdminPanel onBack={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('管理面板')).toBeInTheDocument());

    fireEvent.click(screen.getByText('系统日志'));
    expect(screen.getByTestId('tab-logs')).toBeInTheDocument();

    fireEvent.click(screen.getByText('系统配置'));
    expect(screen.getByTestId('tab-config')).toBeInTheDocument();

    fireEvent.click(screen.getByText('用户管理'));
    expect(screen.getByTestId('tab-users')).toBeInTheDocument();
  });

  it('返回按钮触发 onBack', async () => {
    const onBack = vi.fn();
    mockedGetUsers.mockResolvedValue({ users: [makeUser()], pagination: {} as never });
    render(<AdminPanel onBack={onBack} />);
    await waitFor(() => expect(screen.getByText('返回应用')).toBeInTheDocument());

    fireEvent.click(screen.getByText('返回应用'));
    expect(onBack).toHaveBeenCalledTimes(1);
  });
});
