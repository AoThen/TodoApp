package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"strconv"
	"time"
)

// randomString 生成密码学安全的随机字符串（hex 编码），用于 local_id 冲突副本后缀
func randomString(length int) string {
	b := make([]byte, (length+1)/2)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 极少失败；降级为时间戳源，保证功能不中断
		log.Printf("生成随机数失败，降级为时间戳源: %v", err)
		const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
		out := make([]byte, length)
		for i := range out {
			out[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		}
		return string(out)
	}
	return hex.EncodeToString(b)[:length]
}

// ============ Admin Handlers ============

type adminCreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type adminUpdateUserRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type adminResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

type adminLockUserRequest struct {
	DurationMinutes int `json:"duration_minutes"`
}

type adminConfigRequest struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

func toInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

// ============ Notification Handlers ============

type notificationCreateRequest struct {
	Type      string  `json:"type"` // system_error, sync_failed, maintenance, etc.
	Title     string  `json:"title"`
	Content   string  `json:"content"`
	Priority  string  `json:"priority"`   // urgent, high, normal, low
	ExpiresAt *string `json:"expires_at"` // Optional: ISO 8601 format
}
