import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import DevicePairing from './DevicePairing';

const { mockApiService } = vi.hoisted(() => ({
  mockApiService: {
    initPairing: vi.fn(),
    pairDevice: vi.fn(),
    getDevices: vi.fn(),
    regenerateKey: vi.fn(),
    revokeDevice: vi.fn(),
  },
}));

vi.mock('../services/api', () => ({ apiService: mockApiService }));
vi.mock('qrcode.react', () => ({
  QRCodeSVG: ({ value }: { value: string }) => <svg id="pairing-qr-code" data-value={value} />,
}));

const pairingInfo = {
  key: 'a'.repeat(64),
  device_id: 'web-device-1',
  server_url: 'http://localhost:8080',
  expires_at: '2026-01-01T10:05:00Z',
  encryption_key: 'b'.repeat(64),
};

const makeDevice = (overrides: Record<string, unknown> = {}) => ({
  id: 1,
  device_type: 'android',
  device_id: 'android-abc',
  server_url: 'http://localhost:8080',
  paired_at: '2026-01-01T09:00:00Z',
  last_seen: '2026-01-01T09:30:00Z',
  is_active: true,
  ...overrides,
});

const renderPairing = () => render(<DevicePairing onClose={vi.fn()} />);

describe('DevicePairing', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockApiService.initPairing.mockResolvedValue(pairingInfo);
    mockApiService.pairDevice.mockResolvedValue(undefined);
    mockApiService.getDevices.mockResolvedValue({ devices: [] });
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
      configurable: true,
    });
  });
  afterEach(() => vi.useRealTimers());

  it('挂载后发起配对流程并加载设备列表', async () => {
    renderPairing();
    await waitFor(() => expect(mockApiService.initPairing).toHaveBeenCalledWith('web'));
    await waitFor(() =>
      expect(mockApiService.pairDevice).toHaveBeenCalledWith({
        key: pairingInfo.key,
        device_type: 'web',
        device_id: pairingInfo.device_id,
      })
    );
    expect(mockApiService.getDevices).toHaveBeenCalledTimes(1);
    expect(screen.getByText('http://localhost:8080')).toBeInTheDocument();
  });

  it('配对成功显示提示、已配对徽标与二维码内容', async () => {
    renderPairing();
    await waitFor(() => expect(screen.getByText('设备配对成功！')).toBeInTheDocument());
    expect(screen.getByText('已配对')).toBeInTheDocument();

    const svg = document.getElementById('pairing-qr-code')!;
    const payload = JSON.parse(svg.getAttribute('data-value')!);
    expect(payload).toMatchObject({
      v: 1,
      type: 'todoapp-pairing',
      key: pairingInfo.key,
      server: pairingInfo.server_url,
      expires: Math.floor(new Date(pairingInfo.expires_at).getTime() / 1000),
      encryption_key: pairingInfo.encryption_key,
    });
    expect(screen.getByDisplayValue(pairingInfo.key)).toBeInTheDocument();
  });

  it('initPairing 返回 409 时视为已配对但不显示成功提示', async () => {
    mockApiService.initPairing.mockRejectedValue({ response: { status: 409 } });
    renderPairing();
    await waitFor(() => expect(screen.getByText('已配对')).toBeInTheDocument());
    expect(screen.queryByText('设备配对成功！')).not.toBeInTheDocument();
  });

  it('initPairing 其他错误时不显示已配对徽标', async () => {
    mockApiService.initPairing.mockRejectedValue(new Error('boom'));
    renderPairing();
    await waitFor(() => expect(mockApiService.getDevices).toHaveBeenCalled());
    expect(screen.queryByText('已配对')).not.toBeInTheDocument();
    expect(screen.queryByText('设备配对成功！')).not.toBeInTheDocument();
  });

  it('渲染已配对设备列表并映射设备类型', async () => {
    mockApiService.getDevices.mockResolvedValue({
      devices: [
        makeDevice({ id: 1, device_type: 'web', device_id: 'web-xyz' }),
        makeDevice({ id: 2, device_type: 'android', device_id: 'android-abc' }),
      ],
    });
    renderPairing();

    await waitFor(() => expect(screen.getByText('已配对设备 (2)')).toBeInTheDocument());
    expect(screen.getByText('Web')).toBeInTheDocument();
    expect(screen.getByText('Android')).toBeInTheDocument();
    expect(screen.getByText('ID: web-xyz...')).toBeInTheDocument();
  });

  it('已撤销设备显示撤销状态且不提供操作按钮', async () => {
    mockApiService.getDevices.mockResolvedValue({
      devices: [makeDevice({ id: 1, is_active: false })],
    });
    renderPairing();

    await waitFor(() => expect(screen.getByText('已撤销')).toBeInTheDocument());
    expect(screen.queryByText('重新生成密钥')).not.toBeInTheDocument();
    expect(screen.queryByText('撤销')).not.toBeInTheDocument();
  });

  it('无设备时显示空状态', async () => {
    renderPairing();
    await waitFor(() => expect(screen.getByText('暂无已配对设备')).toBeInTheDocument());
  });

  it('复制密钥写入剪贴板并短暂显示已复制', async () => {
    vi.useFakeTimers();
    renderPairing();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    fireEvent.click(screen.getByText('复制'));
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(pairingInfo.key);
    expect(screen.getByText('已复制!')).toBeInTheDocument();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });
    expect(screen.getByText('复制')).toBeInTheDocument();
  });

  it('刷新密钥需要两段确认，确认后重新配对', async () => {
    vi.useFakeTimers();
    renderPairing();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    const initCallsAfterMount = mockApiService.initPairing.mock.calls.length;

    fireEvent.click(screen.getByText('刷新'));
    expect(screen.getByText('确定刷新？')).toBeInTheDocument();
    expect(mockApiService.initPairing.mock.calls.length).toBe(initCallsAfterMount);

    await act(async () => {
      fireEvent.click(screen.getByText('确定刷新？'));
      await vi.advanceTimersByTimeAsync(50);
    });
    expect(mockApiService.initPairing).toHaveBeenCalledTimes(initCallsAfterMount + 1);
    expect(screen.getByText('密钥已重新生成并配对')).toBeInTheDocument();
  });

  it('刷新确认窗口超时后回到初始状态', async () => {
    vi.useFakeTimers();
    renderPairing();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    fireEvent.click(screen.getByText('刷新'));
    expect(screen.getByText('确定刷新？')).toBeInTheDocument();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000);
    });
    expect(screen.getByText('刷新')).toBeInTheDocument();
  });

  it('确认后重新生成设备密钥', async () => {
    mockApiService.getDevices.mockResolvedValue({ devices: [makeDevice()] });
    mockApiService.regenerateKey.mockResolvedValue({ new_key: 'b'.repeat(64) });
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderPairing();

    await waitFor(() => expect(screen.getByText('重新生成密钥')).toBeInTheDocument());
    fireEvent.click(screen.getByText('重新生成密钥'));

    await waitFor(() => expect(mockApiService.regenerateKey).toHaveBeenCalledWith('android-abc'));
    await waitFor(() =>
      expect(screen.getByText(`密钥已重新生成: ${'b'.repeat(16)}...`)).toBeInTheDocument()
    );
  });

  it('取消重新生成时不调用接口', async () => {
    mockApiService.getDevices.mockResolvedValue({ devices: [makeDevice()] });
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    renderPairing();

    await waitFor(() => expect(screen.getByText('重新生成密钥')).toBeInTheDocument());
    fireEvent.click(screen.getByText('重新生成密钥'));
    expect(mockApiService.regenerateKey).not.toHaveBeenCalled();
  });

  it('确认后撤销设备', async () => {
    mockApiService.getDevices.mockResolvedValue({ devices: [makeDevice()] });
    mockApiService.revokeDevice.mockResolvedValue(undefined);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderPairing();

    await waitFor(() => expect(screen.getByText('撤销')).toBeInTheDocument());
    fireEvent.click(screen.getByText('撤销'));

    await waitFor(() => expect(mockApiService.revokeDevice).toHaveBeenCalledWith('android-abc'));
    await waitFor(() => expect(screen.getByText('设备已撤销')).toBeInTheDocument());
  });

  it('撤销失败时显示错误信息', async () => {
    mockApiService.getDevices.mockResolvedValue({ devices: [makeDevice()] });
    mockApiService.revokeDevice.mockRejectedValue({
      response: { data: { error: '不能撤销最后一个设备' } },
    });
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderPairing();

    await waitFor(() => expect(screen.getByText('撤销')).toBeInTheDocument());
    fireEvent.click(screen.getByText('撤销'));
    await waitFor(() => expect(screen.getByText(/撤销设备失败: 不能撤销最后一个设备/)).toBeInTheDocument());
  });

  it('点击下载二维码按钮不抛错', async () => {
    renderPairing();
    await waitFor(() => expect(screen.getByText('下载二维码图片')).toBeInTheDocument());
    expect(() => fireEvent.click(screen.getByText('下载二维码图片'))).not.toThrow();
  });

  it('关闭按钮触发 onClose', async () => {
    const onClose = vi.fn();
    render(<DevicePairing onClose={onClose} />);
    await waitFor(() => expect(screen.getByText('完成')).toBeInTheDocument());
    fireEvent.click(screen.getByText('完成'));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
