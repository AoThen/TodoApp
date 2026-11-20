package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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
	"todoapp/internal/db"
	"todoapp/internal/response"
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
		Key        string `json:"key"`
		DeviceType string `json:"device_type"`
		DeviceID   string `json:"device_id"`
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

	// 检查设备是否已配对
	existing, _ := db.ValidatePairingKey(userID, req.Key)
	if existing {
		response.ErrorResponse(w, "该设备已配对", http.StatusConflict)
		return
	}

	// 获取服务器URL
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	// 注册设备
	err = db.RegisterDevice(userID, req.DeviceType, req.DeviceID, req.Key, serverURL)
	if err != nil {
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
		"status":     "paired",
		"device_id":  req.DeviceID,
		"server_url": serverURL,
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

	// 生成新密钥
	newKey := generateRandomKey()

	// 更新数据库
	err = db.RegeneratePairingKey(deviceID, newKey)
	if err != nil {
		log.Printf("密钥更新失败: %v", err)
		response.ErrorResponse(w, "密钥更新失败", http.StatusInternalServerError)
		return
	}

	log.Printf("用户 %d 重新生成了设备 %s 的配对密钥", currentUserID, deviceID)

	response.SuccessResponse(w, map[string]interface{}{
		"status":     "regenerated",
		"new_key":    newKey,
		"device_id":  deviceID,
		"server_url": serverURL,
	}, http.StatusOK)
}

// handleRevokeDevice 撤销设备

func handleRevokeDevice(w http.ResponseWriter, r *http.Request) {
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

	response.SuccessResponse(w, map[string]interface{}{
		"status":    "revoked",
		"device_id": deviceID,
	}, http.StatusOK)
}

// generateRandomKey 生成32字节随机密钥（hex格式，64字符）

func generateRandomKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		log.Fatalf("生成随机密钥失败: %v", err)
	}
	return hex.EncodeToString(bytes)
}

// toImportString 安全地转换JSON中的值为字符串（用于导入功能）
