import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import LoginPage from './LoginPage';
import { apiService } from '../../services/api';

vi.mock('../../services/api', () => ({
  apiService: {
    login: vi.fn(),
  },
}));

const mockedLogin = vi.mocked(apiService.login);

describe('LoginPage', () => {
  const mockOnSuccess = vi.fn().mockResolvedValue(undefined);

  const renderPage = () => render(<LoginPage onSuccess={mockOnSuccess} />);

  const fillAndSubmit = async (email: string, password: string) => {
    const user = userEvent.setup();
    await user.type(screen.getByPlaceholderText('Email'), email);
    await user.type(screen.getByPlaceholderText('Password'), password);
    await user.click(screen.getByRole('button', { name: 'Login' }));
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(window, 'alert').mockImplementation(() => {});
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {});
    mockedLogin.mockReset();
  });

  it('渲染登录表单标题与输入框', () => {
    renderPage();

    expect(screen.getByRole('heading', { name: 'TodoApp Login' })).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Email')).toHaveAttribute('type', 'email');
    expect(screen.getByPlaceholderText('Password')).toHaveAttribute('type', 'password');
    expect(screen.getByRole('button', { name: 'Login' })).toBeInTheDocument();
  });

  it('登录成功：携带表单值调用 login，写入 user_email 并回调 onSuccess', async () => {
    mockedLogin.mockResolvedValueOnce({ access_token: 'token-abc', refresh_token: 'r1' } as any);
    renderPage();

    await fillAndSubmit('user@example.com', 'secret123');

    await waitFor(() => {
      expect(mockedLogin).toHaveBeenCalledWith('user@example.com', 'secret123');
    });
    expect(Storage.prototype.setItem).toHaveBeenCalledWith('user_email', 'user@example.com');
    await waitFor(() => {
      expect(mockOnSuccess).toHaveBeenCalledWith('token-abc');
    });
    expect(window.alert).not.toHaveBeenCalled();
  });

  it('登录成功且 onSuccess 为同步函数时正常执行', async () => {
    mockedLogin.mockResolvedValueOnce({ access_token: 'token-sync' } as any);
    const syncOnSuccess = vi.fn();
    render(<LoginPage onSuccess={syncOnSuccess} />);

    await fillAndSubmit('sync@example.com', 'pw');

    await waitFor(() => {
      expect(syncOnSuccess).toHaveBeenCalledWith('token-sync');
    });
  });

  it('登录失败：弹出 alert 提示且不调用 onSuccess', async () => {
    mockedLogin.mockRejectedValueOnce(new Error('401 unauthorized'));
    renderPage();

    await fillAndSubmit('bad@example.com', 'wrongpass');

    await waitFor(() => {
      expect(window.alert).toHaveBeenCalledWith('Login failed. Please check your credentials.');
    });
    expect(mockOnSuccess).not.toHaveBeenCalled();
    expect(Storage.prototype.setItem).not.toHaveBeenCalled();
  });

  it('onSuccess 执行失败时同样走失败提示分支', async () => {
    mockedLogin.mockResolvedValueOnce({ access_token: 'token-late' } as any);
    mockOnSuccess.mockRejectedValueOnce(new Error('sync bootstrap failed'));
    renderPage();

    await fillAndSubmit('late@example.com', 'pw');

    await waitFor(() => {
      expect(window.alert).toHaveBeenCalledWith('Login failed. Please check your credentials.');
    });
    mockOnSuccess.mockResolvedValue(undefined);
  });

  it('表单 submit 事件触发登录流程', async () => {
    mockedLogin.mockResolvedValueOnce({ access_token: 't' } as any);
    renderPage();
    const user = userEvent.setup();
    await user.type(screen.getByPlaceholderText('Email'), 'form@example.com');
    await user.type(screen.getByPlaceholderText('Password'), 'pw');
    fireEvent.submit(screen.getByRole('button', { name: 'Login' }).closest('form')!);

    await waitFor(() => {
      expect(mockedLogin).toHaveBeenCalledWith('form@example.com', 'pw');
    });
  });
});
