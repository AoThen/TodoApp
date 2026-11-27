package websocket

import (
	"encoding/json"
	"errors"
	"sync"
)

// Hub WebSocket连接管理器（同一用户允许多条连接，如多设备/多标签页同时在线）
type Hub struct {
	clients    map[int64]map[*Client]struct{} // userID -> 该用户的连接集合
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.RWMutex
}

// NewHub 创建新的Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[int64]map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
	}
}

// removeLocked 从指定用户的连接集合中移除客户端并关闭其发送通道（须持有写锁）。
// 重复移除（已不在集合中）时直接跳过，避免重复 close。
func (h *Hub) removeLocked(client *Client) {
	set, exists := h.clients[client.userID]
	if !exists {
		return
	}
	if _, ok := set[client]; ok {
		delete(set, client)
		close(client.send)
	}
	if len(set) == 0 {
		delete(h.clients, client.userID)
	}
}

// Run 启动Hub主循环
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			set, exists := h.clients[client.userID]
			if !exists {
				set = make(map[*Client]struct{})
				h.clients[client.userID] = set
			}
			set[client] = struct{}{}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			h.removeLocked(client)
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			var failed []*Client
			for _, set := range h.clients {
				for client := range set {
					if !safeSend(client.send, message) {
						failed = append(failed, client)
					}
				}
			}
			h.mu.RUnlock()
			// 发送缓冲已满的连接：移除并关闭（在写锁下操作，避免读锁内改共享 map）
			if len(failed) > 0 {
				h.mu.Lock()
				for _, client := range failed {
					h.removeLocked(client)
				}
				h.mu.Unlock()
			}
		}
	}
}

// BroadcastToUser 向指定用户的所有在线连接发送消息（至少送达一条即视为成功）
func (h *Hub) BroadcastToUser(userID int64, msg Message) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	set, exists := h.clients[userID]
	if !exists || len(set) == 0 {
		return errors.New("user not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	delivered := 0
	for client := range set {
		if safeSend(client.send, data) {
			delivered++
		}
	}
	if delivered == 0 {
		return errors.New("send buffer full")
	}
	return nil
}

// CloseDevice 关闭指定用户下指定设备的所有在线连接（设备撤销时调用，使停用立即生效）
func (h *Hub) CloseDevice(userID int64, deviceID string) {
	h.mu.RLock()
	var targets []*Client
	for client := range h.clients[userID] {
		if client.deviceID == deviceID {
			targets = append(targets, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range targets {
		_ = client.conn.Close()
	}
}

// IsUserConnected 检查用户是否在线
func (h *Hub) IsUserConnected(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	set, exists := h.clients[userID]
	return exists && len(set) > 0
}

// GetConnectedClientCount 获取在线客户端数量（含同一用户的多条连接）
func (h *Hub) GetConnectedClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for _, set := range h.clients {
		count += len(set)
	}
	return count
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
