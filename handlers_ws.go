package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"
)

import (
	"todoapp/internal/auth"
	"todoapp/internal/response"
	"todoapp/internal/websocket"
	wsclient "todoapp/internal/websocket"
)

func handleWebSocket(w http.ResponseWriter, r *http.Request, wsHub *websocket.Hub) {
	// 从query参数获取加密设置
	encryptionEnabled := r.URL.Query().Get("encryption") == "true"

	// ✅ 优先从 Sec-WebSocket-Protocol subprotocol 获取token（更安全）
	var token string
	protocols := strings.Split(r.Header.Get("Sec-WebSocket-Protocol"), ",")
	for _, protocol := range protocols {
		trimmed := strings.TrimSpace(protocol)
		if strings.HasPrefix(trimmed, "todoapp.") {
			token = strings.TrimPrefix(trimmed, "todoapp.")
			break
		}
	}

	// 降级：从query参数或header获取token（向后兼容）
	if token == "" {
		token = r.URL.Query().Get("token")
		if token == "" {
			token = r.Header.Get("Authorization")
			if strings.HasPrefix(token, "Bearer ") {
				token = token[7:]
			}
		}
	}

	if token == "" {
		log.Printf("WebSocket connection rejected: missing token from %s", r.RemoteAddr)
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	// 验证token
	claims, err := auth.ValidateToken(token)
	if err != nil || claims.TokenType != "access" {
		log.Printf("WebSocket connection rejected: invalid token from %s", r.RemoteAddr)
		response.ErrorResponse(w, "无效的令牌", http.StatusUnauthorized)
		return
	}

	userID, err := strconv.Atoi(claims.UserID)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	// 升级HTTP连接为WebSocket连接
	// 如果使用subprotocol，需要传递给Upgrade
	conn, err := wsclient.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed for user %s: %v", claims.Email, err)
		return
	}

	log.Printf("WebSocket connected: %s (ID: %d, encryption: %v)", claims.Email, userID, encryptionEnabled)

	// 创建客户端并启动读写循环
	client := websocket.NewClient(wsHub, conn, int64(userID), claims.Email, encryptionEnabled)
	wsHub.Register(client)

	// 启动读写goroutine
	go client.WritePump()
	client.ReadPump()
}
