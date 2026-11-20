import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { websocketService, WSMessage } from './websocket';

// jsdom 的 Blob 未实现 arrayBuffer，测试环境补齐（浏览器原生支持）
if (typeof Blob !== 'undefined' && !Blob.prototype.arrayBuffer) {
  Blob.prototype.arrayBuffer = function (): Promise<ArrayBuffer> {
    return new Promise((resolve, reject) => {
      const fr = new FileReader();
      fr.onload = () => resolve(fr.result as ArrayBuffer);
      fr.onerror = () => reject(fr.error);
      fr.readAsArrayBuffer(this);
    });
  };
}

vi.mock('../utils/crypto', () => ({
  encrypt: vi.fn(async (plain: string) => btoa('enc:' + plain)),
  decrypt: vi.fn(async (b64: string) => atob(b64).replace(/^enc:/, '')),
}));

class MockWebSocket {
  static OPEN = 1;
  static CONNECTING = 0;
  static CLOSING = 2;
  static CLOSED = 3;
  static instances: MockWebSocket[] = [];

  url: string;
  protocols: string[];
  readyState = MockWebSocket.OPEN;
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: unknown }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: ((e: unknown) => void) | null = null;
  sent: unknown[] = [];

  constructor(url: string, protocols?: string[]) {
    this.url = url;
    this.protocols = protocols || [];
    MockWebSocket.instances.push(this);
  }

  send(data: unknown) {
    this.sent.push(data);
  }

  close() {
    if (this.readyState === MockWebSocket.CLOSED) return;
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.();
  }

  // 测试辅助：模拟服务端下发 Blob 帧
  emitBlobMessage(plain: string) {
    const bytes = new TextEncoder().encode(plain);
    this.onmessage?.({ data: new Blob([bytes]) });
  }
}

beforeEach(() => {
  MockWebSocket.instances = [];
  vi.stubGlobal('WebSocket', MockWebSocket);
});

afterEach(() => {
  websocketService.disconnect();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

async function flushAsync(times = 5) {
  for (let i = 0; i < times; i++) {
    await Promise.resolve();
    if (vi.isFakeTimers()) {
      await vi.advanceTimersByTimeAsync(0);
    } else {
      await new Promise(r => setTimeout(r, 0));
    }
  }
}

describe('websocketService', () => {
  it('connect 建立 WebSocket 连接并携带加密子协议', async () => {
    await websocketService.connect('tok-1', true);
    const ws = MockWebSocket.instances[0];
    expect(ws.url).toBe('ws://localhost:3000/ws');
    expect(ws.protocols[0]).toBe('todoapp.encryption.tok-1');
  });

  it('非加密模式使用 todoapp 子协议', async () => {
    await websocketService.connect('tok-2', false);
    const ws = MockWebSocket.instances[0];
    expect(ws.protocols[0]).toBe('todoapp.tok-2');
  });

  it('onopen 后发送握手，收到匹配握手响应后进入就绪', async () => {
    await websocketService.connect('tok-3', true);
    const ws = MockWebSocket.instances[0];

    ws.onopen?.();
    // 握手在就绪前每 100ms 重试；等待握手响应处理后发出（真实定时器，避开 jsdom Blob 的内部定时器）
    await new Promise(r => setTimeout(r, 250));
    ws.emitBlobMessage(JSON.stringify({ type: 'handshake', encryption_enabled: true, timestamp: 't' }));
    await flushAsync();
    await new Promise(r => setTimeout(r, 250));

    expect(websocketService.isConnected()).toBe(true);
    const sentHandshake = ws.sent.some(frame => {
      const text = new TextDecoder().decode(frame as ArrayBuffer);
      return text.includes('"type":"handshake"');
    });
    expect(sentHandshake).toBe(true);
  });

  it('握手加密协商失败时关闭连接', async () => {
    await websocketService.connect('tok-4', true);
    const ws = MockWebSocket.instances[0];
    const closeSpy = vi.spyOn(ws, 'close');

    ws.onopen?.();
    ws.emitBlobMessage(JSON.stringify({ type: 'handshake', encryption_enabled: false, timestamp: 't' }));
    await flushAsync();
    await new Promise(r => setTimeout(r, 50));

    expect(closeSpy).toHaveBeenCalled();
  });

  it('Blob 消息解密后分发给对应 handler', async () => {
    await websocketService.connect('tok-5', true);
    const ws = MockWebSocket.instances[0];
    const handler = vi.fn();
    websocketService.onMessage('notification', handler);

    ws.onopen?.();
    const msg: WSMessage = { type: 'notification', data: { title: 'hi' }, timestamp: 't' };
    ws.emitBlobMessage(JSON.stringify(msg));
    await flushAsync();
    await new Promise(r => setTimeout(r, 50));

    expect(handler).toHaveBeenCalledTimes(1);
    expect(handler.mock.calls[0][0].data).toEqual({ title: 'hi' });
    websocketService.offMessage('notification');
  });

  it('pong 与 error 消息不触发业务 handler', async () => {
    await websocketService.connect('tok-6', true);
    const ws = MockWebSocket.instances[0];
    const handler = vi.fn();
    websocketService.onMessage('notification', handler);

    ws.onopen?.();
    ws.emitBlobMessage(JSON.stringify({ type: 'pong', timestamp: 't' }));
    ws.emitBlobMessage(JSON.stringify({ type: 'error', error: 'boom', timestamp: 't' }));
    await flushAsync();

    expect(handler).not.toHaveBeenCalled();
  });

  it('断开后按指数退避重连（2^n * 1000ms），最多 5 次', async () => {
    vi.useFakeTimers();
    await websocketService.connect('tok-7', true);
    await flushAsync();

    // 第一次断开 → 2s 后重连
    MockWebSocket.instances[0].onclose?.();
    await vi.advanceTimersByTimeAsync(2000);
    expect(MockWebSocket.instances).toHaveLength(2);

    // 第二次断开 → 4s 后重连
    MockWebSocket.instances[1].onclose?.();
    await vi.advanceTimersByTimeAsync(4000);
    expect(MockWebSocket.instances).toHaveLength(3);

    // 连续断开至超过上限（共 5 次重连）
    MockWebSocket.instances[2].onclose?.();
    await vi.advanceTimersByTimeAsync(8000);
    MockWebSocket.instances[3].onclose?.();
    await vi.advanceTimersByTimeAsync(16000);
    MockWebSocket.instances[4].onclose?.();
    await vi.advanceTimersByTimeAsync(32000);
    expect(MockWebSocket.instances).toHaveLength(6);

    // 第 6 次断开不再重连
    MockWebSocket.instances[5].onclose?.();
    await vi.advanceTimersByTimeAsync(60000);
    expect(MockWebSocket.instances).toHaveLength(6);
  });

  it('disconnect 后不再自动重连', async () => {
    vi.useFakeTimers();
    await websocketService.connect('tok-8', true);
    await flushAsync();

    const before = MockWebSocket.instances.length;
    websocketService.disconnect();
    await vi.advanceTimersByTimeAsync(10000);
    expect(MockWebSocket.instances.length).toBe(before);
    expect(websocketService.isConnected()).toBe(false);
  });
});
