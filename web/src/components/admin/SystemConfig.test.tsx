import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import SystemConfig from './SystemConfig';
import { adminService } from '../../services/admin';

vi.mock('../../services/admin', () => ({
  adminService: { getConfig: vi.fn(), setConfig: vi.fn() },
}));

const mocked = vi.mocked(adminService);

describe('SystemConfig', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocked.getConfig.mockResolvedValue({
      max_login_attempts: '5',
      unknown_key: 'custom',
    });
    mocked.setConfig.mockResolvedValue(undefined);
  });

  it('加载期间显示 loading', () => {
    mocked.getConfig.mockReturnValue(new Promise(() => {}));
    render(<SystemConfig />);
    expect(screen.getByText('加载中...')).toBeInTheDocument();
  });

  it('渲染配置项与已知键的中文描述', async () => {
    render(<SystemConfig />);
    await waitFor(() => expect(screen.getByText('max_login_attempts')).toBeInTheDocument());
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('最大登录失败尝试次数')).toBeInTheDocument();
    expect(screen.getByText('unknown_key')).toBeInTheDocument();
    expect(screen.getByText('custom')).toBeInTheDocument();
  });

  it('无配置时显示空状态', async () => {
    mocked.getConfig.mockResolvedValue({});
    render(<SystemConfig />);
    await waitFor(() => expect(screen.getByText('暂无配置项')).toBeInTheDocument());
  });

  it('加载失败显示错误提示', async () => {
    mocked.getConfig.mockRejectedValue(new Error('boom'));
    render(<SystemConfig />);
    await waitFor(() => expect(screen.getByText('加载配置失败')).toBeInTheDocument());
  });

  it('添加配置：填写键值后保存并重新加载', async () => {
    render(<SystemConfig />);
    await waitFor(() => expect(screen.getByText('max_login_attempts')).toBeInTheDocument());

    fireEvent.click(screen.getByText('添加配置'));
    expect(screen.getByText('添加配置', { selector: 'h2' })).toBeInTheDocument();

    const modal = document.querySelector('.modal')!;
    const inputs = modal.querySelectorAll('input');
    fireEvent.change(inputs[0], { target: { value: 'rate_limit_per_minute' } });
    fireEvent.change(inputs[1], { target: { value: '600' } });
    fireEvent.change(inputs[2], { target: { value: '每分钟请求上限' } });
    fireEvent.submit(modal.querySelector('form')!);

    await waitFor(() =>
      expect(mocked.setConfig).toHaveBeenCalledWith('rate_limit_per_minute', '600', '每分钟请求上限')
    );
    await waitFor(() => expect(screen.getByText('配置已更新')).toBeInTheDocument());
    expect(mocked.getConfig).toHaveBeenCalledTimes(2);
  });

  it('编辑已有配置时不显示键输入框', async () => {
    render(<SystemConfig />);
    await waitFor(() => expect(screen.getByText('max_login_attempts')).toBeInTheDocument());

    fireEvent.click(screen.getAllByText('编辑')[0]);
    expect(screen.getByText('编辑配置', { selector: 'h2' })).toBeInTheDocument();
    expect(document.querySelector('.modal input[placeholder]')).toBeNull();

    const modal = document.querySelector('.modal')!;
    const inputs = modal.querySelectorAll('input');
    fireEvent.change(inputs[0], { target: { value: '10' } });
    fireEvent.submit(modal.querySelector('form')!);

    await waitFor(() =>
      expect(mocked.setConfig).toHaveBeenCalledWith('max_login_attempts', '10', '最大登录失败尝试次数')
    );
  });

  it('保存失败时显示错误信息', async () => {
    mocked.setConfig.mockRejectedValue({ response: { data: { error: '配置键无效' } } });
    render(<SystemConfig />);
    await waitFor(() => expect(screen.getByText('max_login_attempts')).toBeInTheDocument());

    fireEvent.click(screen.getByText('添加配置'));
    const modal = document.querySelector('.modal')!;
    const inputs = modal.querySelectorAll('input');
    fireEvent.change(inputs[0], { target: { value: 'bad key!' } });
    fireEvent.change(inputs[1], { target: { value: '1' } });
    fireEvent.submit(modal.querySelector('form')!);

    await waitFor(() => expect(screen.getByText('配置键无效')).toBeInTheDocument());
  });

  it('取消按钮关闭弹窗', async () => {
    render(<SystemConfig />);
    await waitFor(() => expect(screen.getByText('max_login_attempts')).toBeInTheDocument());

    fireEvent.click(screen.getByText('添加配置'));
    expect(document.querySelector('.modal-overlay')).not.toBeNull();
    fireEvent.click(screen.getByText('取消'));
    expect(document.querySelector('.modal-overlay')).toBeNull();
  });
});
