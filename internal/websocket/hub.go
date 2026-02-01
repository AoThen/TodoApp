package websocket

import (
	"encoding/json"
	"errors"
	"sync"
)

// Hub WebSocket连接管理器
type Hub struct {
	clients      map[int64]*Client // userID -> Client
	register     chan *Client
	unregister   chan *Client
	broadcast    chan []byte
	userChannels map[int64]chan []byte // 每个用户的私有通道
	mu           sync.RWMutex
}

// NewHub 创建新的Hub
func NewHub() *Hub {
	return &Hub{
		clients:      make(map[int64]*Client),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
		broadcast:    make(chan []byte),
		userChannels: make(map[int64]chan []byte),
	}
}

// Run 启动Hub主循环
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.userID] = client
			h.userChannels[client.userID] = client.send
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, exists := h.clients[client.userID]; exists {
				delete(h.clients, client.userID)
				delete(h.userChannels, client.userID)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				select {
				case client.send <- message:
				default:
					// 发送失败，关闭连接
					close(client.send)
					delete(h.clients, client.userID)
					delete(h.userChannels, client.userID)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastToUser 向指定用户发送消息
func (h *Hub) BroadcastToUser(userID int64, msg Message) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	channel, exists := h.userChannels[userID]
	if !exists {
		return errors.New("user not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case channel <- data:
		return nil
	default:
		return errors.New("send buffer full")
	}
}

// IsUserConnected 检查用户是否在线
func (h *Hub) IsUserConnected(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	_, exists := h.clients[userID]
	return exists
}

// GetConnectedClientCount 获取在线客户端数量
func (h *Hub) GetConnectedClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.clients)
}

// Register 注册客户端（外部调用）
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister 注销客户端（外部调用）
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Broadcast 广播消息给所有客户端
func (h *Hub) Broadcast(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	h.broadcast <- data
	return nil
}
