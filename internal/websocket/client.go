package websocket

import (
	"encoding/json"
	"net/http"
	"time"

	gorillawebsocket "github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024 * 1024 // 1MB
)

// Upgrader WebSocket升级器（导出供外部使用）
var Upgrader = gorillawebsocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 生产环境应该验证Origin
	},
}

// Message WebSocket消息结构
type Message struct {
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp string                 `json:"timestamp"`
	MessageID string                 `json:"message_id,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// Client WebSocket客户端连接
type Client struct {
	hub       *Hub
	conn      *gorillawebsocket.Conn
	userID    int64
	email     string
	send      chan []byte
	encryptor *WebSocketEncryptor
}

// NewClient 创建新客户端
func NewClient(hub *Hub, conn *gorillawebsocket.Conn, userID int64, email string, encryptionEnabled bool) *Client {
	return &Client{
		hub:       hub,
		conn:      conn,
		userID:    userID,
		email:     email,
		send:      make(chan []byte, 256),
		encryptor: NewWebSocketEncryptor(encryptionEnabled, email),
	}
}

// ReadPump 读取消息循环
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			if gorillawebsocket.IsUnexpectedCloseError(err, gorillawebsocket.CloseGoingAway, gorillawebsocket.CloseAbnormalClosure) {
				// Log error but don't crash
			}
			break
		}

		// 处理消息
		if err := c.handleMessage(messageType, message); err != nil {
			c.sendError("消息处理失败")
		}
	}
}

// WritePump 发送消息循环
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(gorillawebsocket.CloseMessage, []byte{})
				return
			}

			// 加密后发送
			encrypted, err := c.encryptor.EncryptMessage(message)
			if err != nil {
				// Log error
				continue
			}

			if err := c.conn.WriteMessage(gorillawebsocket.BinaryMessage, encrypted); err != nil {
				return
			}

		case <-ticker.C:
			c.sendPing()
		}
	}
}

// handleMessage 处理接收到的消息
func (c *Client) handleMessage(messageType int, data []byte) error {
	// 如果是二进制消息，先解密
	if messageType == gorillawebsocket.BinaryMessage {
		decrypted, err := c.encryptor.DecryptMessage(data)
		if err != nil {
			return err
		}
		data = decrypted
	}

	// 解析JSON
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}

	// 处理握手
	if msg.Type == "handshake" {
		return c.handleHandshake(data)
	}

	// 其他消息类型处理
	switch msg.Type {
	case "ping":
		c.sendPong()
	case "subscribe":
		// 处理订阅请求
	case "unsubscribe":
		// 处理取消订阅
	default:
		// Log unknown message type
	}

	return nil
}

// handleHandshake 处理握手
func (c *Client) handleHandshake(data []byte) error {
	if err := c.encryptor.ProcessHandshake(data); err != nil {
		return err
	}

	// 发送握手响应
	resp, err := c.encryptor.CreateHandshakeResponse()
	if err != nil {
		return err
	}

	respData, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	// 握手消息不加密（或可选加密）
	c.send <- respData

	return nil
}

// sendPing 发送ping
func (c *Client) sendPing() {
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := c.conn.WriteMessage(gorillawebsocket.PingMessage, nil); err != nil {
		// Log error
	}
}

// sendPong 发送pong
func (c *Client) sendPong() {
	msg := Message{
		Type:      "pong",
		Timestamp: time.Now().Format(time.RFC3339),
	}
	c.SendMessage(msg)
}

// sendError 发送错误消息
func (c *Client) sendError(errMsg string) {
	msg := Message{
		Type:      "error",
		Data:      map[string]interface{}{"error": errMsg},
		Timestamp: time.Now().Format(time.RFC3339),
	}
	c.SendMessage(msg)
}

// SendMessage 发送消息（自动加密）
func (c *Client) SendMessage(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.send <- data:
	default:
		// Send buffer full
	}

	return nil
}

// SendNotification 发送通知消息（便捷方法）
func (c *Client) SendNotification(notif map[string]interface{}) error {
	msg := Message{
		Type:      "notification",
		Data:      notif,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	return c.SendMessage(msg)
}
