package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
)

import (
	"github.com/gorilla/mux"
)

import (
	"todoapp/internal/crypto"
	"todoapp/internal/db"
	"todoapp/internal/response"
	"todoapp/internal/websocket"
)

func handleInitPairing(w http.ResponseWriter, r *http.Request) {
	userIDStr := getUserIDFromContext(r.Context())
	if userIDStr == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	var req struct {
		DeviceType string `json:"device_type"`
		DeviceID   string `json:"device_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, "无效的请求数据", http.StatusBadRequest)
		return
	}

	if req.DeviceType == "" {
		req.DeviceType = "web"
	}
	if req.DeviceID == "" {
		req.DeviceID = generateDeviceID(req.DeviceType)
	}

	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	// 顺带清理该用户已过期的待配对记录，避免 devices 表堆积垃圾行
	if err := db.DeleteExpiredPendingPairings(userID); err != nil {
		log.Printf("清理过期待配对记录失败: %v", err)
	}

	key, expiresAt, err := db.CreatePendingPairing(userID, req.DeviceType)
	if err != nil {
		log.Printf("创建配对密钥失败: %v", err)
		response.ErrorResponse(w, "创建配对密钥失败", http.StatusInternalServerError)
		return
	}

	response.SuccessResponse(w, map[string]interface{}{
		"key":        key,
		"device_id":  req.DeviceID,
		"server_url": serverURL,
		"expires_at": expiresAt,
	}, http.StatusOK)
}

func generateDeviceID(deviceType string) string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return fmt.Sprintf("%s-%s", deviceType, hex.EncodeToString(bytes))
}

// handleDevicePairing 处理设备配对请求

func handleDevicePairing(w http.ResponseWriter, r *http.Request) {
	userIDStr := getUserIDFromContext(r.Context())
	if userIDStr == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Key             string `json:"key"`
		DeviceType      string `json:"device_type"`
		DeviceID        string `json:"device_id"`
		ClientPublicKey string `json:"client_public_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, "无效的请求数据", http.StatusBadRequest)
		return
	}

	// 验证必填字段
	if req.Key == "" || req.DeviceType == "" || req.DeviceID == "" {
		response.ErrorResponse(w, "配对密钥、设备类型和设备ID为必填项", http.StatusBadRequest)
		return
	}

	// 验证密钥格式（64字符hex）
	if len(req.Key) != 64 {
		response.ErrorResponse(w, "配对密钥格式无效", http.StatusBadRequest)
		return
	}

	// 验证设备类型
	validDeviceTypes := map[string]bool{"web": true, "android": true, "ios": true}
	if !validDeviceTypes[req.DeviceType] {
		response.ErrorResponse(w, "无效的设备类型", http.StatusBadRequest)
		return
	}

	// 客户端临时公钥为必填，用于包装每设备密钥
	if req.ClientPublicKey == "" {
		response.ErrorResponse(w, "缺少客户端公钥", http.StatusBadRequest)
		return
	}
	if _, err := crypto.ParsePublicKeySPKI(req.ClientPublicKey); err != nil {
		response.ErrorResponse(w, "客户端公钥格式无效", http.StatusBadRequest)
		return
	}

	// 检查设备是否已配对
	existing, _ := db.ValidatePairingKey(userID, req.Key)
	if existing {
		response.ErrorResponse(w, "该设备已配对", http.StatusConflict)
		return
	}

	// 校验待配对记录：必须命中本人名下未消费且未过期的配对密钥，防止伪造密钥注册设备
	pendingExists, pendingExpired, err := db.ValidatePendingPairing(userID, req.Key)
	if err != nil {
		log.Printf("查询待配对记录失败: %v", err)
		response.ErrorResponse(w, "配对校验失败", http.StatusInternalServerError)
		return
	}
	if !pendingExists {
		response.ErrorResponse(w, "无效的配对密钥，请先在网页端生成配对二维码", http.StatusBadRequest)
		return
	}
	if pendingExpired {
		_ = db.DeleteExpiredPendingPairings(userID)
		response.ErrorResponse(w, "配对密钥已过期，请重新生成二维码", http.StatusGone)
		return
	}

	// 获取服务器URL
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	// 生成每设备随机密钥，并用客户端临时公钥包装后下发
	deviceKeyHex := crypto.GenerateKey()
	wrappedKey, serverPublicKey, nonce, err := crypto.WrapDeviceKey(req.ClientPublicKey, deviceKeyHex, req.DeviceID, req.Key)
	if err != nil {
		log.Printf("包装设备密钥失败: %v", err)
		response.ErrorResponse(w, "设备密钥包装失败", http.StatusInternalServerError)
		return
	}

	// 用服务端 KEK 封装后入库，避免数据库文件泄露时设备密钥明文可见
	sealedKey, err := crypto.SealWithKEK(deviceKEK, deviceKeyHex, req.DeviceID)
	if err != nil {
		log.Printf("封装设备密钥失败: %v", err)
		response.ErrorResponse(w, "设备密钥包装失败", http.StatusInternalServerError)
		return
	}

	// 注册设备
	err = db.RegisterDevice(userID, req.DeviceType, req.DeviceID, req.Key, serverURL, sealedKey)
	if err != nil {
		if errors.Is(err, db.ErrDeviceOwnedByOther) {
			response.ErrorResponse(w, "设备已被其他账号占用", http.StatusConflict)
			return
		}
		log.Printf("设备注册失败: %v", err)
		response.ErrorResponse(w, "设备注册失败", http.StatusInternalServerError)
		return
	}

	// 记录审计日志
	log.Printf("用户 %d 注册设备: type=%s, id=%s", userID, req.DeviceType, req.DeviceID)

	// 清理已消费的待配对记录，避免 devices 表堆积 is_active=0 的行
	if err := db.ClearPendingPairing(userID, req.Key); err != nil {
		log.Printf("清理待配对记录失败: %v", err)
	}

	response.SuccessResponse(w, map[string]interface{}{
		"status":            "paired",
		"device_id":         req.DeviceID,
		"server_url":        serverURL,
		"wrapped_key":       wrappedKey,
		"server_public_key": serverPublicKey,
		"nonce":             nonce,
		"key_exchange":      crypto.DeviceKeyWrapInfoPrefix,
	}, http.StatusOK)
}

// handleListDevices 获取用户的设备列表

func handleListDevices(w http.ResponseWriter, r *http.Request) {
	userIDStr := getUserIDFromContext(r.Context())
	if userIDStr == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	userID, _ := strconv.Atoi(userIDStr)

	devices, err := db.GetUserDevices(userID)
	if err != nil {
		log.Printf("获取设备列表失败: %v", err)
		response.ErrorResponse(w, "获取设备列表失败", http.StatusInternalServerError)
		return
	}

	response.SuccessResponse(w, map[string]interface{}{
		"devices": devices,
		"count":   len(devices),
	}, http.StatusOK)
}

// handleRegenerateDeviceKey 重新生成设备配对密钥

func handleRegenerateDeviceKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["id"]

	if deviceID == "" {
		response.ErrorResponse(w, "设备ID不能为空", http.StatusBadRequest)
		return
	}

	// 验证用户是否有权限操作此设备
	userID, serverURL, _, isActive, err := db.GetDeviceInfoByDeviceID(deviceID)
	if err != nil {
		response.ErrorResponse(w, "设备不存在", http.StatusNotFound)
		return
	}

	if !isActive {
		response.ErrorResponse(w, "设备已撤销", http.StatusBadRequest)
		return
	}

	// 检查当前用户是否是设备所有者
	currentUserIDStr := getUserIDFromContext(r.Context())
	currentUserID, _ := strconv.Atoi(currentUserIDStr)
	if currentUserID != userID {
		response.ErrorResponse(w, "无权操作此设备", http.StatusForbidden)
		return
	}

	// 重新生成用于该设备的配对码：以新的待配对记录下发，避免覆盖活动设备的密钥导致无法再配对
	deviceType, typeErr := db.GetDeviceTypeByDeviceID(deviceID)
	if typeErr != nil || deviceType == "" {
		deviceType = "android"
	}
	newKey, expiresAt, err := db.CreatePendingPairing(currentUserID, deviceType)
	if err != nil {
		log.Printf("重新生成配对密钥失败: %v", err)
		response.ErrorResponse(w, "密钥更新失败", http.StatusInternalServerError)
		return
	}

	log.Printf("用户 %d 重新生成了设备 %s 的配对密钥", currentUserID, deviceID)

	response.SuccessResponse(w, map[string]interface{}{
		"status":     "regenerated",
		"new_key":    newKey,
		"device_id":  deviceID,
		"server_url": serverURL,
		"expires_at": expiresAt,
	}, http.StatusOK)
}

// handleRevokeDevice 撤销设备

func handleRevokeDevice(w http.ResponseWriter, r *http.Request, wsHub *websocket.Hub) {
	vars := mux.Vars(r)
	deviceID := vars["id"]

	if deviceID == "" {
		response.ErrorResponse(w, "设备ID不能为空", http.StatusBadRequest)
		return
	}

	// 验证用户是否有权限操作此设备
	userID, _, _, isActive, err := db.GetDeviceInfoByDeviceID(deviceID)
	if err != nil {
		response.ErrorResponse(w, "设备不存在", http.StatusNotFound)
		return
	}

	if !isActive {
		response.ErrorResponse(w, "设备已撤销", http.StatusBadRequest)
		return
	}

	// 检查当前用户是否是设备所有者
	currentUserIDStr := getUserIDFromContext(r.Context())
	currentUserID, _ := strconv.Atoi(currentUserIDStr)
	if currentUserID != userID {
		response.ErrorResponse(w, "无权操作此设备", http.StatusForbidden)
		return
	}

	// 撤销设备
	err = db.RevokeDevice(deviceID)
	if err != nil {
		log.Printf("撤销设备失败: %v", err)
		response.ErrorResponse(w, "撤销设备失败", http.StatusInternalServerError)
		return
	}

	log.Printf("用户 %d 撤销了设备 %s", currentUserID, deviceID)

	// 立即断开该设备的在线实时消息连接，使撤销即时生效
	if wsHub != nil {
		wsHub.CloseDevice(int64(currentUserID), deviceID)
	}

	response.SuccessResponse(w, map[string]interface{}{
		"status":    "revoked",
		"device_id": deviceID,
	}, http.StatusOK)
}

// toImportString 安全地转换JSON中的值为字符串（用于导入功能）
