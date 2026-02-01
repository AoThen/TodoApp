import { encrypt as cryptoEncrypt, decrypt as cryptoDecrypt } from '../utils/crypto';

const PING_INTERVAL = 30000; // 30秒
const RECONNECT_DELAY_BASE = 1000; // 1秒
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
  private encryptionEnabled: boolean = true;
  private ready: boolean = false;
  private messageHandlers: Map<string, (msg: WSMessage) => void> = new Map();
  private reconnectAttempts: number = 0;
  private pingTimer: NodeJS.Timeout | null = null;
  private reconnectTimer: NodeJS.Timeout | null = null;

  async connect(token: string, enableEncryption: boolean = true): Promise<void> {
    this.token = token;
    this.encryptionEnabled = enableEncryption;
    this.reconnectAttempts = 0;

    const encryptionParam = enableEncryption ? '&encryption=true' : '';
    const wsUrl = `ws://localhost:8080/ws?token=${token}${encryptionParam}`;

    try {
      this.ws = new WebSocket(wsUrl);

      this.ws.onopen = () => {
        console.log('[WebSocket] Connected');
        this.reconnectAttempts = 0;
        this.sendHandshake();
        this.startPingTimer();
      };

      this.ws.onmessage = async (event) => {
        await this.handleMessage(event);
      };

      this.ws.onclose = () => {
        console.log('[WebSocket] Disconnected');
        this.ready = false;
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

    await this.send(handshake);
  }

  private async handleMessage(event: MessageEvent): Promise<void> {
    try {
      let messageData: WSMessage;

      if (event.data instanceof Blob) {
        const decrypted = await this.decryptMessage(event.data);
        messageData = JSON.parse(decrypted);
      } else {
        messageData = JSON.parse(event.data);
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

  private async decryptMessage(blob: Blob): Promise<string> {
    const data = await blob.arrayBuffer();

    if (!this.encryptionEnabled) {
      return new TextDecoder().decode(data);
    }

    const encrypted = btoa(String.fromCharCode(...Array.from(new Uint8Array(data))));
    const decrypted = await cryptoDecrypt(encrypted);
    return decrypted;
  }

  private async send(message: any): Promise<void> {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket not connected');
    }

    if (this.encryptionEnabled && !this.ready) {
      await new Promise(resolve => setTimeout(resolve, 100));
      return this.send(message);
    }

    const messageStr = JSON.stringify(message);
    const encrypted = await this.encryptMessage(messageStr);
    this.ws.send(encrypted);
  }

  private startPingTimer(): void {
    this.stopPingTimer();
    this.pingTimer = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.send({ type: 'ping' });
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

      this.reconnectTimer = setTimeout(() => {
        if (this.token) {
          this.connect(this.token, this.encryptionEnabled);
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
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
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
