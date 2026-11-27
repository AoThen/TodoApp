const STORAGE_KEY = 'todoapp_device_key_v2';
const LEGACY_STORAGE_KEY = 'todoapp_device_key';
const DEVICE_KEY_INFO_PREFIX = 'todoapp/device-key-v2';

function getStoredKey(): string | null {
  try {
    const key = localStorage.getItem(STORAGE_KEY);
    // 仅接受合法 64 位 hex 的每设备密钥；旧全局密钥/脏数据一律视为未配置
    if (key && /^[0-9a-fA-F]{64}$/.test(key)) {
      return key;
    }
    return null;
  } catch {
    return null;
  }
}

export function setDeviceKey(key: string): void {
  try {
    localStorage.setItem(STORAGE_KEY, key);
    // 迁移：清理可能残留的旧版全局密钥
    localStorage.removeItem(LEGACY_STORAGE_KEY);
  } catch (e) {
    console.error('Failed to store device key:', e);
  }
}

export function clearDeviceKey(): void {
  try {
    localStorage.removeItem(STORAGE_KEY);
    localStorage.removeItem(LEGACY_STORAGE_KEY);
  } catch (e) {
    console.error('Failed to clear device key:', e);
  }
}

async function deriveKey(keyMaterial: ArrayBuffer): Promise<CryptoKey> {
  const key = await crypto.subtle.importKey(
    'raw',
    keyMaterial,
    { name: 'AES-GCM' },
    false,
    ['encrypt', 'decrypt']
  );
  return key;
}

async function getKeyFromHex(hexKey: string): Promise<CryptoKey> {
  const keyBytes = new Uint8Array(hexKey.match(/.{1,2}/g)!.map(byte => parseInt(byte, 16)));
  return deriveKey(keyBytes.buffer);
}

export async function encrypt(plaintext: string, keyHex?: string): Promise<string> {
  const key = keyHex || getStoredKey();
  if (!key) {
    throw new Error('Encryption key not configured');
  }

  const cryptoKey = await getKeyFromHex(key);
  const iv = crypto.getRandomValues(new Uint8Array(12));

  const encoder = new TextEncoder();
  const data = encoder.encode(plaintext);

  const ciphertext = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv },
    cryptoKey,
    data
  );

  const combined = new Uint8Array(iv.length + ciphertext.byteLength);
  combined.set(iv);
  combined.set(new Uint8Array(ciphertext), iv.length);

  return btoa(String.fromCharCode(...Array.from(combined)));
}

export async function decrypt(encryptedBase64: string, keyHex?: string): Promise<string> {
  const key = keyHex || getStoredKey();
  if (!key) {
    throw new Error('Encryption key not configured');
  }

  const cryptoKey = await getKeyFromHex(key);
  const combined = new Uint8Array(
    atob(encryptedBase64).split('').map(c => c.charCodeAt(0))
  );

  const iv = combined.slice(0, 12);
  const ciphertext = combined.slice(12);

  const decrypted = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv },
    cryptoKey,
    ciphertext
  );

  const decoder = new TextDecoder();
  return decoder.decode(decrypted);
}

export function generateKey(): string {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('');
}

export function isEncryptionConfigured(): boolean {
  return !!getStoredKey();
}

// ---------------- ECDH 每设备密钥领取 ----------------

export interface ClientEphemeralKey {
  privateKey: CryptoKey;
  publicKeySpki: ArrayBuffer;
  publicKeyB64: string;
}

function arrayBufferToBase64(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf);
  let binary = '';
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

function base64ToBytes(b64: string): Uint8Array {
  const binary = atob(b64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

function concatBytes(...parts: Uint8Array[]): Uint8Array {
  const total = parts.reduce((n, p) => n + p.length, 0);
  const out = new Uint8Array(total);
  let offset = 0;
  for (const part of parts) {
    out.set(part, offset);
    offset += part.length;
  }
  return out;
}

function bytesToHex(bytes: Uint8Array): string {
  return Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('');
}

/** 生成一次性 P-256 客户端密钥对（私钥不可导出） */
export async function generateClientEphemeralKey(): Promise<ClientEphemeralKey> {
  const keyPair = await crypto.subtle.generateKey(
    { name: 'ECDH', namedCurve: 'P-256' },
    false,
    ['deriveBits']
  );
  const spki = await crypto.subtle.exportKey('spki', keyPair.publicKey);
  return {
    privateKey: keyPair.privateKey,
    publicKeySpki: spki,
    publicKeyB64: arrayBufferToBase64(spki),
  };
}

/**
 * 用客户端临时私钥解包服务端下发的每设备密钥。
 * 与 Go 端 WrapDeviceKey 严格对齐：P-256 + HKDF-SHA256 + AES-256-GCM，
 * AAD = clientSPKI || serverSPKI || deviceId || pairingKey。
 */
export async function unwrapDeviceKey(
  clientPrivateKey: CryptoKey,
  clientPublicSpki: ArrayBuffer,
  serverPublicKeyB64: string,
  wrappedKeyB64: string,
  nonceB64: string,
  deviceId: string,
  pairingKey: string
): Promise<string> {
  const clientDer = new Uint8Array(clientPublicSpki);
  const serverDer = base64ToBytes(serverPublicKeyB64);

  const serverPublicKey = await crypto.subtle.importKey(
    'spki',
    serverDer,
    { name: 'ECDH', namedCurve: 'P-256' },
    false,
    []
  );

  const sharedBits = await crypto.subtle.deriveBits(
    { name: 'ECDH', public: serverPublicKey },
    clientPrivateKey,
    256
  );

  const salt = await crypto.subtle.digest('SHA-256', concatBytes(clientDer, serverDer));
  const info = new TextEncoder().encode(`${DEVICE_KEY_INFO_PREFIX}|${deviceId}`);

  const hkdfKey = await crypto.subtle.importKey('raw', sharedBits, 'HKDF', false, ['deriveKey']);
  const kek = await crypto.subtle.deriveKey(
    { name: 'HKDF', hash: 'SHA-256', salt, info },
    hkdfKey,
    { name: 'AES-GCM', length: 256 },
    false,
    ['decrypt']
  );

  const aad = concatBytes(
    clientDer,
    serverDer,
    new TextEncoder().encode(deviceId),
    new TextEncoder().encode(pairingKey)
  );

  const plaintext = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv: base64ToBytes(nonceB64), additionalData: aad },
    kek,
    base64ToBytes(wrappedKeyB64)
  );

  return bytesToHex(new Uint8Array(plaintext));
}
