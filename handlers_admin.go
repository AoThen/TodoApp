package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

import (
	"github.com/gorilla/mux"
)

import (
	"todoapp/internal/db"
	"todoapp/internal/response"
	"todoapp/internal/validator"
	wsclient "todoapp/internal/websocket"
)

func handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	// 解析分页参数
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 解析过滤参数
	email := r.URL.Query().Get("email")
	role := r.URL.Query().Get("role")

	users, total, err := db.GetUsersPaginated(page, pageSize, email, role)
	if err != nil {
		log.Printf("Failed to get users: %v", err)
		response.ErrorResponse(w, "获取用户列表失败", http.StatusInternalServerError)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	hasPrev := page > 1
	hasNext := page < totalPages

	response.SuccessResponse(w, map[string]interface{}{
		"users": users,
		"pagination": map[string]interface{}{
			"page":      page,
			"page_size": pageSize,
			"total":     total,
			"pages":     totalPages,
			"has_prev":  hasPrev,
			"has_next":  hasNext,
		},
	}, http.StatusOK)
}

// handleAdminCreateUserWithNotification 创建用户并发送通知

func handleAdminCreateUserWithNotification(w http.ResponseWriter, r *http.Request, adminEmail string, wsHub *wsclient.Hub) {
	var req adminCreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		response.ErrorResponse(w, "邮箱和密码是必填项", http.StatusBadRequest)
		return
	}

	if !validator.IsValidEmail(req.Email) {
		response.ErrorResponse(w, "邮箱格式无效", http.StatusBadRequest)
		return
	}

	if req.Role != "admin" && req.Role != "user" {
		response.ErrorResponse(w, "无效的角色", http.StatusBadRequest)
		return
	}

	if err := validator.ValidatePassword(req.Password); err != nil {
		response.ErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, err := db.CreateUser(req.Email, req.Password, req.Role)
	if err != nil {
		response.ErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	adminID := getUserIDFromContext(r.Context())
	ip := getClientIP(r)
	if err := db.LogAdminAction(toInt(adminID), adminEmail, "create_user", req.Email, userID,
		fmt.Sprintf("Created user with role: %s", req.Role), ip); err != nil {
		log.Printf("记录管理操作日志错误: %v", err)
	}

	response.SuccessResponse(w, map[string]interface{}{"id": userID, "email": req.Email, "role": req.Role}, http.StatusCreated)
}

func handleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	var req adminUpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if req.Email != "" && !validator.IsValidEmail(req.Email) {
		response.ErrorResponse(w, "邮箱格式无效", http.StatusBadRequest)
		return
	}

	if req.Role != "" && req.Role != "admin" && req.Role != "user" {
		response.ErrorResponse(w, "无效的角色", http.StatusBadRequest)
		return
	}

	oldEmail, _ := db.GetUserEmail(userID)

	if err := db.UpdateUser(userID, req.Email, req.Role); err != nil {
		response.ErrorResponse(w, "更新用户失败: "+err.Error(), http.StatusBadRequest)
		return
	}

	adminID := getUserIDFromContext(r.Context())
	adminEmail := getEmailFromContext(r.Context())
	ip := getClientIP(r)

	details := "用户信息更新"
	if req.Email != "" && oldEmail != "" && oldEmail != fmt.Sprintf("ID:%d", userID) {
		details = fmt.Sprintf("邮箱: %s -> %s", oldEmail, req.Email)
	}
	if req.Role != "" {
		if details != "用户信息更新" {
			details = fmt.Sprintf("%s, 角色: %s", details, req.Role)
		} else {
			details = fmt.Sprintf("角色: %s", req.Role)
		}
	}

	if err := db.LogAdminAction(toInt(adminID), adminEmail, "update_user", req.Email, userID, details, ip); err != nil {
		log.Printf("记录管理操作日志错误: %v", err)
	}

	log.Printf("Admin %s updated user ID %d", adminEmail, userID)
	response.SuccessResponse(w, map[string]string{"status": "updated"}, http.StatusOK)
}

func handleAdminResetPassword(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	var req adminResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if req.NewPassword == "" {
		response.ErrorResponse(w, "新密码不能为空", http.StatusBadRequest)
		return
	}
	if err := validator.ValidatePassword(req.NewPassword); err != nil {
		response.ErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := db.ResetUserPassword(userID, req.NewPassword); err != nil {
		response.ErrorResponse(w, "重置密码失败", http.StatusInternalServerError)
		return
	}

	targetEmail, _ := db.GetUserEmail(userID)
	adminID := getUserIDFromContext(r.Context())
	adminEmail := getEmailFromContext(r.Context())
	ip := getClientIP(r)
	if err := db.LogAdminAction(toInt(adminID), adminEmail, "reset_password", targetEmail, userID, "Password reset by admin", ip); err != nil {
		log.Printf("记录管理操作日志错误: %v", err)
	}

	log.Printf("Admin %s reset password for user %s (ID: %d)", adminEmail, targetEmail, userID)
	response.SuccessResponse(w, map[string]string{"status": "password reset"}, http.StatusOK)
}

func handleAdminLockUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	var req adminLockUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.DurationMinutes = 30
	}

	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 30
	}

	if err := db.LockUserAccount(userID, req.DurationMinutes); err != nil {
		response.ErrorResponse(w, "锁定账户失败", http.StatusInternalServerError)
		return
	}

	targetEmail, err := db.GetUserEmail(userID)
	if err != nil {
		log.Printf("获取用户邮箱错误: %v", err)
		targetEmail = fmt.Sprintf("ID:%d", userID)
	}
	adminID := getUserIDFromContext(r.Context())
	adminEmail := getEmailFromContext(r.Context())
	ip := getClientIP(r)
	if err := db.LogAdminAction(toInt(adminID), adminEmail, "lock_user", targetEmail, userID,
		fmt.Sprintf("Locked for %d minutes", req.DurationMinutes), ip); err != nil {
		log.Printf("记录管理操作日志错误: %v", err)
	}

	response.SuccessResponse(w, map[string]string{"status": "user locked"}, http.StatusOK)
}

func handleAdminUnlockUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	if err := db.UnlockUserAccount(userID); err != nil {
		response.ErrorResponse(w, "解锁账户失败", http.StatusInternalServerError)
		return
	}

	targetEmail, err := db.GetUserEmail(userID)
	if err != nil {
		log.Printf("获取用户邮箱错误: %v", err)
		targetEmail = fmt.Sprintf("ID:%d", userID)
	}
	adminID := getUserIDFromContext(r.Context())
	adminEmail := getEmailFromContext(r.Context())
	ip := getClientIP(r)
	if err := db.LogAdminAction(toInt(adminID), adminEmail, "unlock_user", targetEmail, userID, "Account unlocked by admin", ip); err != nil {
		log.Printf("记录管理操作日志错误: %v", err)
	}

	response.SuccessResponse(w, map[string]string{"status": "user unlocked"}, http.StatusOK)
}

func handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	targetEmail, _ := db.GetUserEmail(userID)
	if err := db.DeleteUser(userID); err != nil {
		response.ErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	adminID := getUserIDFromContext(r.Context())
	adminEmail := getEmailFromContext(r.Context())
	ip := getClientIP(r)
	if err := db.LogAdminAction(toInt(adminID), adminEmail, "delete_user", targetEmail, userID, "User deleted by admin", ip); err != nil {
		log.Printf("记录管理操作日志错误: %v", err)
	}

	log.Printf("Admin %s deleted user %s (ID: %d)", adminEmail, targetEmail, userID)
	response.SuccessResponse(w, map[string]string{"status": "user deleted"}, http.StatusOK)
}

func handleAdminGetLoginLogs(w http.ResponseWriter, r *http.Request) {
	filters := map[string]string{
		"email":      r.URL.Query().Get("email"),
		"success":    r.URL.Query().Get("success"),
		"start_time": r.URL.Query().Get("start_time"),
		"end_time":   r.URL.Query().Get("end_time"),
		"page":       r.URL.Query().Get("page"),
		"page_size":  r.URL.Query().Get("page_size"),
	}

	logs, err := db.GetLoginLogsWithFilters(filters)
	if err != nil {
		log.Printf("Failed to get login logs: %v", err)
		response.ErrorResponse(w, "获取登录日志失败", http.StatusInternalServerError)
		return
	}
	response.SuccessResponse(w, logs, http.StatusOK)
}

func handleAdminGetActionLogs(w http.ResponseWriter, r *http.Request) {
	filters := map[string]string{
		"email":        r.URL.Query().Get("email"),
		"target_email": r.URL.Query().Get("target_email"),
		"action":       r.URL.Query().Get("action"),
		"start_time":   r.URL.Query().Get("start_time"),
		"end_time":     r.URL.Query().Get("end_time"),
		"page":         r.URL.Query().Get("page"),
		"page_size":    r.URL.Query().Get("page_size"),
	}

	logs, err := db.GetAdminLogs(filters)
	if err != nil {
		log.Printf("Failed to get admin logs: %v", err)
		response.ErrorResponse(w, "获取操作日志失败", http.StatusInternalServerError)
		return
	}
	response.SuccessResponse(w, logs, http.StatusOK)
}

func handleAdminGetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := db.GetSystemConfig()
	if err != nil {
		response.ErrorResponse(w, "获取配置失败", http.StatusInternalServerError)
		return
	}
	response.SuccessResponse(w, config, http.StatusOK)
}

func handleAdminSetConfig(w http.ResponseWriter, r *http.Request) {
	var req adminConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if req.Key == "" || req.Value == "" {
		response.ErrorResponse(w, "配置键和值不能为空", http.StatusBadRequest)
		return
	}

	adminID := getUserIDFromContext(r.Context())
	if err := db.SetSystemConfig(req.Key, req.Value, req.Description, adminID); err != nil {
		response.ErrorResponse(w, "设置配置失败", http.StatusInternalServerError)
		return
	}

	adminEmail := getEmailFromContext(r.Context())
	ip := getClientIP(r)
	if err := db.LogAdminAction(toInt(adminID), adminEmail, "update_config", "", 0, fmt.Sprintf("Updated config: %s = %s", req.Key, req.Value), ip); err != nil {
		log.Printf("记录管理操作日志错误: %v", err)
	}

	response.SuccessResponse(w, map[string]string{"status": "config updated"}, http.StatusOK)
}

// Helper functions
