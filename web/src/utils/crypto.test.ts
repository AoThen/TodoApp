import { describe, it, expect, beforeEach, beforeAll, vi } from 'vitest';
import { webcrypto } from 'node:crypto';
import { encrypt, decrypt, generateKey, setDeviceKey, clearDeviceKey, isEncryptionConfigured, generateClientEphemeralKey, unwrapDeviceKey } from './crypto';

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

  it('localStorage 读取失败时视为未配置', () => {
    const spy = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('denied');
    });
    expect(isEncryptionConfigured()).toBe(false);
    spy.mockRestore();
  });
});

function bytesToB64(input: ArrayBuffer | Uint8Array): string {
  const bytes = input instanceof Uint8Array ? input : new Uint8Array(input);
  let binary = '';
  for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
  return btoa(binary);
}

function hexToBytes(hex: string): Uint8Array {
  const out = new Uint8Array(hex.length / 2);
  for (let i = 0; i < out.length; i++) out[i] = parseInt(hex.substr(i * 2, 2), 16);
  return out;
}

function concatU8(...parts: Uint8Array[]): Uint8Array {
  const total = parts.reduce((n, p) => n + p.length, 0);
  const out = new Uint8Array(total);
  let offset = 0;
  for (const part of parts) {
    out.set(part, offset);
    offset += part.length;
  }
  return out;
}

// 模拟服务端 WrapDeviceKey：用客户端公钥包装设备密钥，验证客户端能解包
async function serverWrap(
  clientPublicSpki: ArrayBuffer,
  deviceKeyHex: string,
  deviceId: string,
  pairingKey: string
): Promise<{ wrapped: string; serverPub: string; nonce: string }> {
  const serverKp = await crypto.subtle.generateKey({ name: 'ECDH', namedCurve: 'P-256' }, false, ['deriveBits']);
  const serverSpki = await crypto.subtle.exportKey('spki', serverKp.publicKey);
  const clientPub = await crypto.subtle.importKey('spki', clientPublicSpki, { name: 'ECDH', namedCurve: 'P-256' }, false, []);
  const shared = await crypto.subtle.deriveBits({ name: 'ECDH', public: clientPub }, serverKp.privateKey, 256);

  const clientDer = new Uint8Array(clientPublicSpki);
  const serverDer = new Uint8Array(serverSpki);
  const salt = await crypto.subtle.digest('SHA-256', concatU8(clientDer, serverDer));
  const info = new TextEncoder().encode(`todoapp/device-key-v2|${deviceId}`);
  const hkdfKey = await crypto.subtle.importKey('raw', shared, 'HKDF', false, ['deriveKey']);
  const kek = await crypto.subtle.deriveKey(
    { name: 'HKDF', hash: 'SHA-256', salt, info },
    hkdfKey,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt']
  );

  const nonce = crypto.getRandomValues(new Uint8Array(12));
  const aad = concatU8(
    clientDer,
    serverDer,
    new TextEncoder().encode(deviceId),
    new TextEncoder().encode(pairingKey)
  );
  const ct = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv: nonce, additionalData: aad },
    kek,
    hexToBytes(deviceKeyHex)
  );

  return { wrapped: bytesToB64(ct), serverPub: bytesToB64(serverSpki), nonce: bytesToB64(nonce) };
}

describe('crypto (ECDH 每设备密钥领取)', () => {
  it('客户端可解包服务端包装的每设备密钥', async () => {
    const client = await generateClientEphemeralKey();
    const deviceKeyHex = 'b'.repeat(64);
    const envelope = await serverWrap(client.publicKeySpki, deviceKeyHex, 'dev-1', 'pair-1');

    const got = await unwrapDeviceKey(
      client.privateKey,
      client.publicKeySpki,
      envelope.serverPub,
      envelope.wrapped,
      envelope.nonce,
      'dev-1',
      'pair-1'
    );
    expect(got).toBe(deviceKeyHex);
  });

  it('错误配对码导致解包失败', async () => {
    const client = await generateClientEphemeralKey();
    const envelope = await serverWrap(client.publicKeySpki, 'c'.repeat(64), 'dev-1', 'pair-1');
    await expect(
      unwrapDeviceKey(
        client.privateKey,
        client.publicKeySpki,
        envelope.serverPub,
        envelope.wrapped,
        envelope.nonce,
        'dev-1',
        'pair-WRONG'
      )
    ).rejects.toThrow();
  });
});
