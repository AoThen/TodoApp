package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

import (
	"github.com/gorilla/mux"
)

import (
	"todoapp/internal/db"
	"todoapp/internal/response"
	wsclient "todoapp/internal/websocket"
)

func handleGetNotifications(w http.ResponseWriter, r *http.Request) {
	userIDStr := getUserIDFromContext(r.Context())
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	filters := map[string]string{
		"read":     r.URL.Query().Get("read"),
		"type":     r.URL.Query().Get("type"),
		"priority": r.URL.Query().Get("priority"),
	}

	notifications, total, err := db.GetNotificationsPaginated(userID, page, pageSize, filters)
	if err != nil {
		response.ErrorResponse(w, "获取通知失败", http.StatusInternalServerError)
		return
	}

	response.SuccessResponse(w, map[string]interface{}{
		"notifications": notifications,
		"pagination": map[string]interface{}{
			"page":      page,
			"page_size": pageSize,
			"total":     total,
			"pages":     (total + pageSize - 1) / pageSize,
		},
	}, http.StatusOK)
}

func handleCreateNotification(w http.ResponseWriter, r *http.Request) {
	userIDStr := getUserIDFromContext(r.Context())
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	var req notificationCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if req.Type == "" || req.Title == "" || req.Content == "" {
		response.ErrorResponse(w, "类型、标题和内容是必填项", http.StatusBadRequest)
		return
	}

	if req.Priority == "" {
		req.Priority = "normal"
	}

	validPriorities := map[string]bool{"urgent": true, "high": true, "normal": true, "low": true}
	if !validPriorities[req.Priority] {
		response.ErrorResponse(w, "无效的优先级", http.StatusBadRequest)
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			response.ErrorResponse(w, "无效的过期时间格式", http.StatusBadRequest)
			return
		}
		expiresAt = &t
	}

	notificationID, err := db.CreateNotification(userID, req.Type, req.Title, req.Content, req.Priority, expiresAt)
	if err != nil {
		response.ErrorResponse(w, "创建通知失败", http.StatusInternalServerError)
		return
	}

	notification, err := db.GetNotificationByID(userID, int(notificationID))
	if err != nil {
		response.ErrorResponse(w, "创建通知失败", http.StatusInternalServerError)
		return
	}

	response.SuccessResponse(w, notification, http.StatusCreated)
}

func handleMarkAsRead(w http.ResponseWriter, r *http.Request) {
	userIDStr := getUserIDFromContext(r.Context())
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	notificationIDStr := vars["id"]
	notificationID, err := strconv.Atoi(notificationIDStr)
	if err != nil {
		response.ErrorResponse(w, "无效的通知ID", http.StatusBadRequest)
		return
	}

	if err := db.MarkNotificationAsRead(userID, notificationID); err != nil {
		if err.Error() == "通知不存在" {
			response.ErrorResponse(w, err.Error(), http.StatusNotFound)
		} else {
			response.ErrorResponse(w, "标记已读失败", http.StatusInternalServerError)
		}
		return
	}

	response.SuccessResponse(w, map[string]string{"status": "已标记为已读"}, http.StatusOK)
}

func handleMarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	userIDStr := getUserIDFromContext(r.Context())
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	count, err := db.MarkAllNotificationsAsRead(userID)
	if err != nil {
		response.ErrorResponse(w, "标记全部已读失败", http.StatusInternalServerError)
		return
	}

	response.SuccessResponse(w, map[string]interface{}{
		"status":       "已标记全部为已读",
		"marked_count": count,
	}, http.StatusOK)
}

func handleDeleteNotification(w http.ResponseWriter, r *http.Request) {
	userIDStr := getUserIDFromContext(r.Context())
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	notificationIDStr := vars["id"]
	notificationID, err := strconv.Atoi(notificationIDStr)
	if err != nil {
		response.ErrorResponse(w, "无效的通知ID", http.StatusBadRequest)
		return
	}

	if err := db.DeleteNotification(userID, notificationID); err != nil {
		if err.Error() == "通知不存在" {
			response.ErrorResponse(w, err.Error(), http.StatusNotFound)
		} else {
			response.ErrorResponse(w, "删除通知失败", http.StatusInternalServerError)
		}
		return
	}

	response.SuccessResponse(w, map[string]string{"status": "已删除"}, http.StatusOK)
}

func handleClearNotifications(w http.ResponseWriter, r *http.Request) {
	userIDStr := getUserIDFromContext(r.Context())
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	olderThanDaysStr := r.URL.Query().Get("older_than_days")
	olderThanDays := 30
	if olderThanDaysStr != "" {
		var err error
		olderThanDays, err = strconv.Atoi(olderThanDaysStr)
		if err != nil {
			log.Printf("转换older_than_days错误: %v", err)
			olderThanDays = 30
		}
		if olderThanDays < 0 {
			olderThanDays = 0
		}
	}

	count, err := db.ClearNotifications(userID, olderThanDays)
	if err != nil {
		response.ErrorResponse(w, "清空通知失败", http.StatusInternalServerError)
		return
	}

	response.SuccessResponse(w, map[string]interface{}{
		"status":        "已清空",
		"cleared_count": count,
	}, http.StatusOK)
}

func handleGetUnreadCount(w http.ResponseWriter, r *http.Request) {
	userIDStr := getUserIDFromContext(r.Context())
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	typeParam := r.URL.Query().Get("type")
	var ntypes []string
	if typeParam != "" {
		ntypes = strings.Split(typeParam, ",")
	}

	count, err := db.GetUnreadNotificationsCount(userID, ntypes)
	if err != nil {
		response.ErrorResponse(w, "获取未读数量失败", http.StatusInternalServerError)
		return
	}

	response.SuccessResponse(w, map[string]interface{}{
		"unread_count": count,
	}, http.StatusOK)
}

// ============ Helper Functions ============

// startCleanupTasks 启动定期清理任务

func cleanupExpiredNotifications() {
	count, err := db.CleanupExpiredNotifications()
	if err != nil {
		log.Printf("Failed to cleanup expired notifications: %v", err)
	} else if count > 0 {
		log.Printf("Cleaned up %d expired notifications", count)
	}
}

// cleanupExpiredTokens 清理过期令牌

func sendNotificationToUser(userID int, ntype, title, content, priority string, wsHub *wsclient.Hub) (int64, error) {
	// 1. 保存到数据库
	notificationID, err := db.CreateNotification(userID, ntype, title, content, priority, nil)
	if err != nil {
		return 0, err
	}

	// 2. 通过WebSocket实时推送（如果用户在线）
	if wsHub.IsUserConnected(int64(userID)) {
		notif := map[string]interface{}{
			"id":         notificationID,
			"type":       ntype,
			"title":      title,
			"content":    content,
			"priority":   priority,
			"is_read":    false,
			"created_at": time.Now().Format(time.RFC3339),
		}

		err := wsHub.BroadcastToUser(int64(userID), wsclient.Message{
			Type:      "notification",
			Data:      notif,
			Timestamp: time.Now().Format(time.RFC3339),
		})

		if err != nil {
			// WebSocket发送失败，但不影响数据库存储
			log.Printf("Failed to send notification via WebSocket: %v", err)
		}
	}

	return notificationID, nil
}

// ============ WebSocket Handlers ============

// handleWebSocket WebSocket连接处理
