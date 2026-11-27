import { encrypt as cryptoEncrypt, decrypt as cryptoDecrypt } from '../utils/crypto';

const PING_INTERVAL = 30000;
const RECONNECT_DELAY_BASE = 1000;
const MAX_RECONNECT_ATTEMPTS = 5;

export interface WSMessage {
  type: 'notification' | 'ping' | 'pong' | 'error' | 'handshake' | 'handshake_ack';
  data?: any;
  timestamp: string;
  message_id?: string;
  error?: string;
}

export interface HandshakeResponse {
  type: 'handshake';
  encryption_enabled: boolean;
  server_nonce?: string;
  timestamp: string;
}

class WebSocketService {
  private ws: WebSocket | null = null;
  private token: string | null = null;
  private deviceId: string | null = null;
  private encryptionEnabled: boolean = true;
  private ready: boolean = false;
  private messageHandlers: Map<string, (msg: WSMessage) => void> = new Map();
  private reconnectAttempts: number = 0;
  private pingTimer: number | null = null;
  private reconnectTimer: number | null = null;
  private handshakeTimer: number | null = null;
  private handshakeAttempts: number = 0;

  async connect(token: string, enableEncryption: boolean = true, isReconnect: boolean = false, deviceId?: string): Promise<void> {
    this.token = token;
    if (deviceId) {
      this.deviceId = deviceId;
    }
    this.encryptionEnabled = enableEncryption;
    // 仅手动连接时重置重连计数；自动重连路径必须保留计数，否则退避上限永不生效
    if (!isReconnect) {
      this.reconnectAttempts = 0;
    }

    // Determine WebSocket protocol based on current page protocol
    const isSecure = window.location.protocol === 'https:';
    const wsProtocol = isSecure ? 'wss://' : 'ws://';
    const serverHost = import.meta.env.VITE_WS_HOST || window.location.host;
    // token 走 query 参数：gorilla Upgrader 未配置子协议协商，
    // 携带自定义 Sec-WebSocket-Protocol 会被直接拒绝（401）
    const deviceParam = this.deviceId ? `&device_id=${encodeURIComponent(this.deviceId)}` : '';
    const wsUrl = `${wsProtocol}${serverHost}/ws?token=${encodeURIComponent(token)}&encryption=${enableEncryption}${deviceParam}`;

    try {
      this.ws = new WebSocket(wsUrl);
      this.ws.binaryType = 'arraybuffer';

      this.ws.onopen = () => {
        console.log('[WebSocket] Connected');
        this.reconnectAttempts = 0;
        // 握手失败由 onclose/onerror 驱动重连；握手本身带重试与超时
        this.startHandshake();
        this.startPingTimer();
      };

      this.ws.onmessage = async (event) => {
        await this.handleMessage(event);
      };

      this.ws.onclose = () => {
        console.log('[WebSocket] Disconnected');
        this.ready = false;
        this.clearHandshakeTimer();
        this.stopPingTimer();
        this.handleReconnect();
      };

      this.ws.onerror = (error) => {
        console.error('[WebSocket] Error:', error);
      };

    } catch (error) {
      console.error('[WebSocket] Connection failed:', error);
      this.handleReconnect();
    }
  }

  private async sendHandshake(): Promise<void> {
    const handshake = {
      type: 'handshake',
      encryption: this.encryptionEnabled,
      timestamp: new Date().toISOString()
    };

    // 握手以明文文本帧发送：服务端仅对二进制帧解密，握手本身不含任何机密；
    // 不能等待加密就绪再发，否则会与"服务端须先收到握手才回执"形成死锁。
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket not connected');
    }
    this.ws.send(JSON.stringify(handshake));
  }

  private startHandshake(): void {
    // 非加密模式无需等待协商，发送一次握手后即可就绪，避免触发重试超时
    if (!this.encryptionEnabled) {
      this.ready = true;
      this.sendHandshake().catch(() => {});
      return;
    }

    this.handshakeAttempts = 0;
    this.clearHandshakeTimer();

    const attempt = () => {
      if (this.ws?.readyState !== WebSocket.OPEN || this.ready) {
        return;
      }
      this.sendHandshake().catch(() => {});
      this.handshakeAttempts += 1;
      if (this.handshakeAttempts >= 10) {
        console.error('[WebSocket] Handshake timeout');
        this.ws?.close();
        return;
      }
      this.handshakeTimer = window.setTimeout(attempt, 500);
    };
    attempt();
  }

  private clearHandshakeTimer(): void {
    if (this.handshakeTimer) {
      clearTimeout(this.handshakeTimer);
      this.handshakeTimer = null;
    }
  }

  private async handleMessage(event: MessageEvent): Promise<void> {
    try {
      let messageData: WSMessage;

      if (typeof event.data === 'string') {
        messageData = JSON.parse(event.data);
      } else {
        const decrypted = await this.decryptMessage(event.data as Blob | ArrayBuffer);
        messageData = JSON.parse(decrypted);
      }

      if (messageData.type === 'handshake') {
        await this.handleHandshakeResponse(messageData as HandshakeResponse);
        return;
      }

      if (messageData.type === 'pong') {
        return;
      }

      if (messageData.type === 'error') {
        console.error('[WebSocket] Server error:', messageData.error);
        return;
      }

      const handler = this.messageHandlers.get(messageData.type);
      if (handler) {
        handler(messageData);
      }

    } catch (error) {
      console.error('[WebSocket] Failed to process message:', error);
    }
  }

  private async handleHandshakeResponse(resp: HandshakeResponse): Promise<void> {
    if (resp.encryption_enabled !== this.encryptionEnabled) {
      console.error('[WebSocket] Encryption negotiation failed');
      this.ws?.close();
      return;
    }

    this.ready = true;
    this.clearHandshakeTimer();
    console.log('[WebSocket] Handshake completed, encryption:', resp.encryption_enabled);
  }

  private async encryptMessage(data: string): Promise<ArrayBuffer> {
    if (!this.encryptionEnabled) {
      return new TextEncoder().encode(data).buffer;
    }

    const encrypted = await cryptoEncrypt(data);
    const bytes = new Uint8Array(
      atob(encrypted).split('').map(c => c.charCodeAt(0))
    );
    return bytes.buffer;
  }

  private async decryptMessage(data: Blob | ArrayBuffer): Promise<string> {
    const buffer = data instanceof Blob ? await data.arrayBuffer() : data;

    if (!this.encryptionEnabled) {
      return new TextDecoder().decode(buffer);
    }

    const encrypted = btoa(String.fromCharCode(...Array.from(new Uint8Array(buffer))));
    const decrypted = await cryptoDecrypt(encrypted);
    return decrypted;
  }

  private async send(message: any): Promise<void> {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket not connected');
    }

    if (this.encryptionEnabled && !this.ready) {
      // 握手未完成时拒绝发送普通消息（握手本身走 sendHandshake 明文通道，不受此限）
      throw new Error('WebSocket handshake not ready');
    }

    const messageStr = JSON.stringify(message);
    const encrypted = await this.encryptMessage(messageStr);
    this.ws.send(encrypted);
  }

  private startPingTimer(): void {
    this.stopPingTimer();
    this.pingTimer = window.setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        // 心跳失败静默处理：连接状态由 onclose/onerror 驱动重连，避免 unhandled rejection
        this.send({ type: 'ping' }).catch(() => {});
      }
    }, PING_INTERVAL);
  }

  private stopPingTimer(): void {
    if (this.pingTimer) {
      clearInterval(this.pingTimer);
      this.pingTimer = null;
    }
  }

  private handleReconnect(): void {
    if (this.reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
      this.reconnectAttempts++;
      const delay = Math.pow(2, this.reconnectAttempts) * RECONNECT_DELAY_BASE;

      console.log(`[WebSocket] Reconnecting in ${delay}ms... (attempt ${this.reconnectAttempts}/${MAX_RECONNECT_ATTEMPTS})`);

      this.reconnectTimer = window.setTimeout(() => {
        if (this.token) {
          this.connect(this.token, this.encryptionEnabled, true);
        }
      }, delay);
    } else {
      console.error('[WebSocket] Max reconnect attempts reached');
    }
  }

  onMessage(type: string, handler: (msg: WSMessage) => void): void {
    this.messageHandlers.set(type, handler);
  }

  offMessage(type: string): void {
    this.messageHandlers.delete(type);
  }

  disconnect(): void {
    this.stopPingTimer();
    this.clearHandshakeTimer();
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      // 主动断开不应触发自动重连
      this.ws.onclose = null;
      this.ws.close();
      this.ws = null;
    }
    this.ready = false;
    this.reconnectAttempts = 0;
  }

  isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN && this.ready;
  }
}

export const websocketService = new WebSocketService();
