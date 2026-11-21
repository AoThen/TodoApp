import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import UserManagement from './UserManagement';
import { adminService } from '../../services/admin';
import type { User } from '../../services/admin';

vi.mock('../../services/admin', () => ({
  adminService: {
    getUsers: vi.fn(),
    createUser: vi.fn(),
    updateUser: vi.fn(),
    resetPassword: vi.fn(),
    lockUser: vi.fn(),
    unlockUser: vi.fn(),
    deleteUser: vi.fn(),
  },
}));

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

const adminUser = makeUser();
const lockedUser = makeUser({ id: 2, email: 'locked@example.com', role: 'user', is_locked: true });

const pagination = (overrides: Record<string, unknown> = {}) => ({
  page: 1,
  page_size: 20,
  total: 2,
  pages: 1,
  has_prev: false,
  has_next: false,
  ...overrides,
});

const mocked = vi.mocked(adminService);

const renderPanel = async () => {
  const utils = render(<UserManagement />);
  await waitFor(() => expect(screen.getByText('admin@example.com')).toBeInTheDocument());
  return utils;
};

describe('UserManagement', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocked.getUsers.mockResolvedValue({ users: [adminUser, lockedUser], pagination: pagination() });
    mocked.createUser.mockResolvedValue({ id: 3, email: 'x', role: 'user' });
    mocked.updateUser.mockResolvedValue(undefined);
    mocked.resetPassword.mockResolvedValue(undefined);
    mocked.lockUser.mockResolvedValue(undefined);
    mocked.unlockUser.mockResolvedValue(undefined);
    mocked.deleteUser.mockResolvedValue(undefined);
  });

  it('加载期间显示 loading', () => {
    mocked.getUsers.mockReturnValue(new Promise(() => {}));
    render(<UserManagement />);
    expect(screen.getByText('加载中...')).toBeInTheDocument();
  });

  it('渲染用户列表的角色与锁定状态', async () => {
    await renderPanel();
    expect(screen.getByText('admin@example.com')).toBeInTheDocument();
    expect(screen.getByText('locked@example.com')).toBeInTheDocument();
    expect(document.querySelector('.badge-admin')).toHaveTextContent('管理员');
    expect(document.querySelector('.badge-user')).toHaveTextContent('普通用户');
    expect(document.querySelector('.badge-locked')).toHaveTextContent('已锁定');
    expect(document.querySelector('.badge-active')).toHaveTextContent('正常');
  });

  it('加载失败显示错误提示', async () => {
    mocked.getUsers.mockRejectedValue(new Error('boom'));
    render(<UserManagement />);
    await waitFor(() => expect(screen.getByText('加载用户列表失败')).toBeInTheDocument());
  });

  it('创建用户成功后刷新列表', async () => {
    await renderPanel();
    fireEvent.click(screen.getByText('创建用户'));
    expect(screen.getByText('创建用户', { selector: 'h2' })).toBeInTheDocument();

    fireEvent.change(document.querySelector('.modal input[type=email]')!, {
      target: { value: 'new@example.com' },
    });
    fireEvent.change(document.querySelector('.modal input[type=password]')!, {
      target: { value: 'password123' },
    });
    fireEvent.submit(document.querySelector('.modal form')!);

    await waitFor(() =>
      expect(mocked.createUser).toHaveBeenCalledWith({
        email: 'new@example.com',
        password: 'password123',
        role: 'user',
      })
    );
    await waitFor(() => expect(screen.getByText('用户创建成功')).toBeInTheDocument());
    expect(mocked.getUsers).toHaveBeenCalledTimes(2);
  });

  it('创建用户失败时显示后端错误', async () => {
    mocked.createUser.mockRejectedValue({ response: { data: { error: '邮箱已存在' } } });
    await renderPanel();

    fireEvent.click(screen.getByText('创建用户'));
    fireEvent.change(document.querySelector('.modal input[type=email]')!, {
      target: { value: 'dup@example.com' },
    });
    fireEvent.change(document.querySelector('.modal input[type=password]')!, {
      target: { value: 'password123' },
    });
    fireEvent.submit(document.querySelector('.modal form')!);

    await waitFor(() => expect(screen.getByText('邮箱已存在')).toBeInTheDocument());
  });

  it('编辑用户时表单预填并提交更新', async () => {
    await renderPanel();
    fireEvent.click(screen.getAllByText('编辑')[0]);
    expect(screen.getByText('编辑用户', { selector: 'h2' })).toBeInTheDocument();
    expect((document.querySelector('.modal input[type=email]') as HTMLInputElement).value).toBe(
      'admin@example.com'
    );

    fireEvent.submit(document.querySelector('.modal form')!);
    await waitFor(() =>
      expect(mocked.updateUser).toHaveBeenCalledWith(1, {
        email: 'admin@example.com',
        role: 'admin',
      })
    );
    await waitFor(() => expect(screen.getByText('用户更新成功')).toBeInTheDocument());
  });

  it('重置密码表单提交调用 resetPassword', async () => {
    await renderPanel();
    fireEvent.click(screen.getAllByText('重置密码')[1]);
    expect(screen.getByText('重置密码 - locked@example.com')).toBeInTheDocument();

    fireEvent.change(document.querySelector('.modal input[type=password]')!, {
      target: { value: 'newpass123' },
    });
    fireEvent.submit(document.querySelector('.modal form')!);

    await waitFor(() => expect(mocked.resetPassword).toHaveBeenCalledWith(2, 'newpass123'));
    await waitFor(() => expect(screen.getByText('密码重置成功')).toBeInTheDocument());
  });

  it('锁定与解锁按钮分别调用对应接口', async () => {
    await renderPanel();
    fireEvent.click(screen.getByText('解锁'));
    await waitFor(() => expect(mocked.unlockUser).toHaveBeenCalledWith(2));

    fireEvent.click(screen.getByText('锁定'));
    await waitFor(() => expect(mocked.lockUser).toHaveBeenCalledWith(1));
  });

  it('确认后删除用户', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    await renderPanel();
    fireEvent.click(screen.getAllByText('删除')[0]);
    await waitFor(() => expect(mocked.deleteUser).toHaveBeenCalledWith(1));
    await waitFor(() => expect(screen.getByText('用户已删除')).toBeInTheDocument());
  });

  it('取消删除时不调用接口', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    await renderPanel();
    fireEvent.click(screen.getAllByText('删除')[0]);
    expect(mocked.deleteUser).not.toHaveBeenCalled();
  });

  it('邮箱筛选触发按条件重新加载', async () => {
    await renderPanel();
    fireEvent.change(screen.getByPlaceholderText('搜索邮箱'), {
      target: { value: 'admin' },
    });
    await waitFor(() => {
      expect(mocked.getUsers).toHaveBeenLastCalledWith(
        expect.objectContaining({ email: 'admin' })
      );
    });
    expect(screen.getByText('清除筛选')).toBeInTheDocument();
  });

  it('角色筛选与清除筛选', async () => {
    await renderPanel();
    fireEvent.change(screen.getByDisplayValue('全部角色'), { target: { value: 'admin' } });
    await waitFor(() => {
      expect(mocked.getUsers).toHaveBeenLastCalledWith(expect.objectContaining({ role: 'admin' }));
    });

    fireEvent.click(screen.getByText('清除筛选'));
    await waitFor(() => {
      expect(mocked.getUsers).toHaveBeenLastCalledWith(
        expect.objectContaining({ email: '', role: '' })
      );
    });
  });

  it('多页时翻页按钮按 hasPrev/hasNext 控制可用性', async () => {
    mocked.getUsers.mockResolvedValue({
      users: [adminUser],
      pagination: pagination({ pages: 2, total: 21, has_next: true }),
    });
    render(<UserManagement />);
    await waitFor(() => expect(screen.getByText(/共 2 页/)).toBeInTheDocument());

    expect(screen.getByText('上一页')).toBeDisabled();
    expect(screen.getByText('下一页')).toBeEnabled();
    fireEvent.click(screen.getByText('下一页'));

    await waitFor(() => {
      expect(mocked.getUsers).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }));
    });
  });
});
