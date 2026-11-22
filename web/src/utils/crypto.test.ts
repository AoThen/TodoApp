import { describe, it, expect, beforeEach, beforeAll, vi } from 'vitest';
import { webcrypto } from 'node:crypto';
import { encrypt, decrypt, generateKey, setDeviceKey, clearDeviceKey, isEncryptionConfigured } from './crypto';

const KEY_HEX = 'a'.repeat(64);

// CI(Node20) 的 jsdom 环境下 crypto.subtle 与 Uint8Array 跨 realm，统一注入 Node webcrypto
beforeAll(() => {
  vi.stubGlobal('crypto', webcrypto);
});

describe('crypto (WebCrypto AES-GCM)', () => {
  beforeEach(() => {
    localStorage.clear();
    clearDeviceKey();
  });

  it('generateKey 生成 64 位 hex 且随机', () => {
    const k1 = generateKey();
    const k2 = generateKey();
    expect(k1).toMatch(/^[0-9a-f]{64}$/);
    expect(k1).not.toBe(k2);
  });

  it('加解密往返一致（含中文与 emoji）', async () => {
    const plaintexts = ['', 'hello', '中文内容', 'emoji 🔐', 'line1\nline2'];
    for (const pt of plaintexts) {
      const ct = await encrypt(pt, KEY_HEX);
      expect(await decrypt(ct, KEY_HEX)).toBe(pt);
    }
  });

  it('同一明文两次加密产生不同密文（随机 IV）', async () => {
    const ct1 = await encrypt('same', KEY_HEX);
    const ct2 = await encrypt('same', KEY_HEX);
    expect(ct1).not.toBe(ct2);
  });

  it('篡改密文后解密失败', async () => {
    const ct = await encrypt('sensitive', KEY_HEX);
    const raw = atob(ct).split('').map(c => c.charCodeAt(0));
    raw[raw.length - 1] ^= 0x01;
    const tampered = btoa(String.fromCharCode(...raw));
    await expect(decrypt(tampered, KEY_HEX)).rejects.toThrow();
  });

  it('错误密钥解密失败', async () => {
    const ct = await encrypt('secret', KEY_HEX);
    const wrongKey = 'b'.repeat(64);
    await expect(decrypt(ct, wrongKey)).rejects.toThrow();
  });

  it('未配置密钥时抛出明确错误', async () => {
    await expect(encrypt('x')).rejects.toThrow('Encryption key not configured');
    await expect(decrypt('x')).rejects.toThrow('Encryption key not configured');
  });

  it('setDeviceKey 后 isEncryptionConfigured 为真，可免传 key 加解密', async () => {
    expect(isEncryptionConfigured()).toBe(false);
    setDeviceKey(KEY_HEX);
    expect(isEncryptionConfigured()).toBe(true);
    const ct = await encrypt('storage key works');
    expect(await decrypt(ct)).toBe('storage key works');
  });

  it('localStorage 写入失败时 setDeviceKey 静默降级', () => {
    const storageSpy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota exceeded');
    });
    const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    expect(() => setDeviceKey(KEY_HEX)).not.toThrow();
    expect(errSpy).toHaveBeenCalled();
    storageSpy.mockRestore();
    errSpy.mockRestore();
  });

  it('localStorage 移除失败时 clearDeviceKey 静默降级', () => {
    const storageSpy = vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
      throw new Error('boom');
    });
    const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    expect(() => clearDeviceKey()).not.toThrow();
    expect(errSpy).toHaveBeenCalled();
    storageSpy.mockRestore();
    errSpy.mockRestore();
  });

  it('无存储密钥时回退 VITE_DEVICE_KEY 环境变量', async () => {
    vi.stubEnv('VITE_DEVICE_KEY', KEY_HEX);
    expect(isEncryptionConfigured()).toBe(true);
    const ct = await encrypt('env key works');
    expect(await decrypt(ct)).toBe('env key works');
    vi.unstubAllEnvs();
  });
});
