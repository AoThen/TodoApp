import { apiService } from './api';
import {
  generateClientEphemeralKey,
  unwrapDeviceKey,
  setDeviceKey,
  isEncryptionConfigured,
} from '../utils/crypto';

const DEVICE_ID_KEY = 'todoapp_web_device_id';
let cachedDeviceId: string | null = null;

// 网页设备编号持久化：避免每次登录/打开面板都注册一台新网页设备。
// 内存缓存保证 localStorage 不可用（隐私模式）时同一次运行内编号稳定。
export function getOrCreateWebDeviceId(): string {
  if (cachedDeviceId) {
    return cachedDeviceId;
  }
  try {
    const existing = window.localStorage.getItem(DEVICE_ID_KEY);
    if (existing) {
      cachedDeviceId = existing;
      return existing;
    }
  } catch {
    // 隐私模式下 localStorage 可能不可用，退化为进程内生成
  }
  const id = `web-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
  cachedDeviceId = id;
  try {
    window.localStorage.setItem(DEVICE_ID_KEY, id);
  } catch {
    // 存储失败仍使用内存缓存，保证本次运行内一致
  }
  return id;
}

export function clearWebDeviceId(): void {
  cachedDeviceId = null;
  try {
    window.localStorage.removeItem(DEVICE_ID_KEY);
  } catch {
    // ignore
  }
}

/**
 * 确保网页已持有自己的每设备密钥：缺失则完成 一次性配对码 + ECDH 公钥包装领取。
 * 失败时抛出异常（调用方决定是否阻断实时消息连接）。
 */
let inflight: Promise<boolean> | null = null;

export async function ensureDeviceKey(): Promise<boolean> {
  if (isEncryptionConfigured()) {
    return true;
  }
  // 并发调用（如多组件同时挂载）合并为一次领取，避免重复配对消耗多个一次性码
  if (inflight) {
    return inflight;
  }

  inflight = (async () => {
    const deviceId = getOrCreateWebDeviceId();
    const init = await apiService.initPairing('web', deviceId);
    const ephemeral = await generateClientEphemeralKey();
    const pair = await apiService.pairDevice({
      key: init.key,
      device_type: 'web',
      device_id: deviceId,
      client_public_key: ephemeral.publicKeyB64,
    });

    const deviceKey = await unwrapDeviceKey(
      ephemeral.privateKey,
      ephemeral.publicKeySpki,
      pair.server_public_key,
      pair.wrapped_key,
      pair.nonce,
      deviceId,
      init.key
    );
    setDeviceKey(deviceKey);
    if (!isEncryptionConfigured()) {
      throw new Error('本地存储不可用，无法保存设备密钥');
    }
    return true;
  })();

  try {
    return await inflight;
  } finally {
    inflight = null;
  }
}
