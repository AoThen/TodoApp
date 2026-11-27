package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"
)

import (
	"todoapp/internal/auth"
	"todoapp/internal/crypto"
	"todoapp/internal/db"
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
			token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
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

	// 加密模式下必须能按 (用户, 设备) 解析出该设备的密钥
	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		deviceID = r.Header.Get("X-Device-ID")
	}
	var deviceCM *crypto.CryptoManager
	if encryptionEnabled {
		if deviceID == "" {
			response.ErrorResponse(w, "缺少设备标识", http.StatusBadRequest)
			return
		}
		sealed, found, err := db.GetDeviceEncryptionKeySealed(userID, deviceID)
		if err != nil {
			log.Printf("查询设备密钥失败: %v", err)
			response.ErrorResponse(w, "设备加密密钥不可用", http.StatusInternalServerError)
			return
		}
		if !found {
			response.ErrorResponse(w, "设备未配对或已撤销", http.StatusForbidden)
			return
		}
		keyHex, err := crypto.OpenWithKEK(deviceKEK, sealed, deviceID)
		if err != nil {
			log.Printf("解封设备密钥失败: %v", err)
			response.ErrorResponse(w, "设备加密密钥不可用", http.StatusInternalServerError)
			return
		}
		deviceCM, err = crypto.NewManagerFromHex(keyHex)
		if err != nil {
			response.ErrorResponse(w, "设备加密密钥不可用", http.StatusInternalServerError)
			return
		}
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
	client := websocket.NewClient(wsHub, conn, int64(userID), claims.Email, deviceID, encryptionEnabled, deviceCM)
	wsHub.Register(client)

	// 启动读写goroutine
	go client.WritePump()
	client.ReadPump()
}
