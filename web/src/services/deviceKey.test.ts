import { describe, it, expect, vi, beforeEach } from 'vitest';

const state = vi.hoisted(() => ({ configured: false }));

const mocks = vi.hoisted(() => ({
  initPairing: vi.fn(),
  pairDevice: vi.fn(),
  generateClientEphemeralKey: vi.fn(),
  unwrapDeviceKey: vi.fn(),
  setDeviceKey: vi.fn(),
  isEncryptionConfigured: vi.fn(),
}));

vi.mock('./api', () => ({
  apiService: { initPairing: mocks.initPairing, pairDevice: mocks.pairDevice },
}));

vi.mock('../utils/crypto', () => ({
  generateClientEphemeralKey: mocks.generateClientEphemeralKey,
  unwrapDeviceKey: mocks.unwrapDeviceKey,
  setDeviceKey: mocks.setDeviceKey,
  isEncryptionConfigured: mocks.isEncryptionConfigured,
}));

import { ensureDeviceKey, getOrCreateWebDeviceId, clearWebDeviceId } from './deviceKey';

const DEVICE_KEY = 'a'.repeat(64);

describe('deviceKey', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    clearWebDeviceId();
    state.configured = false;

    mocks.isEncryptionConfigured.mockImplementation(() => state.configured);
    mocks.setDeviceKey.mockImplementation(() => {
      state.configured = true;
    });
    mocks.initPairing.mockResolvedValue({
      key: 'k'.repeat(64),
      device_id: 'web-dev',
      server_url: 'http://localhost:8080',
      expires_at: '2026-01-01T10:05:00Z',
    });
    mocks.generateClientEphemeralKey.mockResolvedValue({
      privateKey: {} as CryptoKey,
      publicKeySpki: new ArrayBuffer(0),
      publicKeyB64: 'client-pub',
    });
    mocks.pairDevice.mockResolvedValue({
      status: 'paired',
      device_id: 'web-dev',
      server_url: 'http://localhost:8080',
      wrapped_key: 'wrapped',
      server_public_key: 'server-pub',
      nonce: 'nonce',
      key_exchange: 'todoapp/device-key-v2',
    });
    mocks.unwrapDeviceKey.mockResolvedValue(DEVICE_KEY);
  });

  it('已持有密钥时直接返回，不重复配对', async () => {
    state.configured = true;
    expect(await ensureDeviceKey()).toBe(true);
    expect(mocks.initPairing).not.toHaveBeenCalled();
  });

  it('缺少密钥时完成领取并保存', async () => {
    expect(await ensureDeviceKey()).toBe(true);
    expect(mocks.initPairing).toHaveBeenCalledWith('web', expect.any(String));
    expect(mocks.pairDevice).toHaveBeenCalledWith({
      key: 'k'.repeat(64),
      device_type: 'web',
      device_id: expect.any(String),
      client_public_key: 'client-pub',
    });
    expect(mocks.unwrapDeviceKey).toHaveBeenCalled();
    expect(mocks.setDeviceKey).toHaveBeenCalledWith(DEVICE_KEY);
  });

  it('并发领取合并为一次配对', async () => {
    const p1 = ensureDeviceKey();
    const p2 = ensureDeviceKey();
    const results = await Promise.all([p1, p2]);
    expect(results).toEqual([true, true]);
    expect(mocks.initPairing).toHaveBeenCalledTimes(1);
  });

  it('保存失败时抛出（fail-closed）', async () => {
    mocks.setDeviceKey.mockImplementation(() => {
      // 存储不可用，保持未配置
    });
    await expect(ensureDeviceKey()).rejects.toThrow();
  });

  it('localStorage 不可用时设备编号仍在本次运行内保持稳定', () => {
    const spy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota');
    });
    const id1 = getOrCreateWebDeviceId();
    const id2 = getOrCreateWebDeviceId();
    expect(id1).toBe(id2);
    spy.mockRestore();
  });

  it('clearWebDeviceId 后会重新生成设备编号', () => {
    const id1 = getOrCreateWebDeviceId();
    clearWebDeviceId();
    const id2 = getOrCreateWebDeviceId();
    expect(id1).not.toBe(id2);
  });

  it('复用已持久化的设备编号', () => {
    window.localStorage.setItem('todoapp_web_device_id', 'persisted-id');
    expect(getOrCreateWebDeviceId()).toBe('persisted-id');
  });

  it('localStorage 读取异常时仍生成编号', () => {
    const spy = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('denied');
    });
    expect(getOrCreateWebDeviceId()).toMatch(/^web-/);
    spy.mockRestore();
  });
});
