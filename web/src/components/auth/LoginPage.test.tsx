import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import LoginPage from './LoginPage';
import { apiService } from '../../services/api';

vi.mock('../../services/api', () => ({
  apiService: { login: vi.fn() },
}));

const login = apiService.login as ReturnType<typeof vi.fn>;

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
});

async function fillAndSubmit(email = 'user@test.local', password = 'password123') {
  fireEvent.change(screen.getByPlaceholderText('Email'), { target: { value: email } });
  fireEvent.change(screen.getByPlaceholderText('Password'), { target: { value: password } });
  await act(async () => {
    fireEvent.click(screen.getByRole('button'));
  });
}

describe('LoginPage', () => {
  it('渲染登录表单', () => {
    render(<LoginPage onSuccess={vi.fn()} />);
    expect(screen.getByPlaceholderText('Email')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Password')).toBeInTheDocument();
    expect(screen.getByRole('button')).toBeInTheDocument();
  });

  it('登录成功时调用 onSuccess 并记录邮箱', async () => {
    login.mockResolvedValue({ access_token: 'token-abc', expires_in: 900 });
    const onSuccess = vi.fn().mockResolvedValue(undefined);
    render(<LoginPage onSuccess={onSuccess} />);

    await fillAndSubmit('admin@test.local', 'secret123');

    expect(login).toHaveBeenCalledWith('admin@test.local', 'secret123');
    expect(onSuccess).toHaveBeenCalledWith('token-abc');
    expect(localStorage.getItem('user_email')).toBe('admin@test.local');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('登录失败时显示内联错误且不调用 onSuccess', async () => {
    login.mockRejectedValue(new Error('401'));
    const onSuccess = vi.fn().mockResolvedValue(undefined);
    render(<LoginPage onSuccess={onSuccess} />);

    await fillAndSubmit('user@test.local', 'wrong');

    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent('登录失败，请检查邮箱和密码');
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('提交过程中按钮禁用，结束后恢复', async () => {
    let resolveLogin!: (v: { access_token: string; expires_in: number }) => void;
    login.mockReturnValue(new Promise((resolve) => { resolveLogin = resolve; }));
    render(<LoginPage onSuccess={vi.fn()} />);

    fireEvent.change(screen.getByPlaceholderText('Email'), { target: { value: 'u@t.local' } });
    fireEvent.change(screen.getByPlaceholderText('Password'), { target: { value: 'p' } });

    await act(async () => {
      fireEvent.click(screen.getByRole('button'));
    });

    // 登录请求仍处于 pending，应停留在提交中状态
    expect(screen.getByRole('button')).toBeDisabled();
    expect(screen.getByText('登录中...')).toBeInTheDocument();

    await act(async () => {
      resolveLogin({ access_token: 't', expires_in: 900 });
    });

    expect(screen.getByRole('button')).toBeEnabled();
  });
});
