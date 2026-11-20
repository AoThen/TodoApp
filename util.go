package main

import (
	"strconv"
	"time"
)

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
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
