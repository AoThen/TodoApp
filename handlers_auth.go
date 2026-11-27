package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

import (
	"todoapp/internal/auth"
	"todoapp/internal/db"
	"todoapp/internal/response"
	"todoapp/internal/validator"
)

// maskEmail 日志脱敏：仅保留本地部分首字符与域名（如 a***@example.com），避免日志泄露完整邮箱
func maskEmail(email string) string {
	at := strings.Index(email, "@")
	if at <= 0 {
		return "***"
	}
	return email[:1] + "***" + email[at:]
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		response.ErrorResponse(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	// 验证输入
	if req.Email == "" || req.Password == "" {
		response.ErrorResponse(w, "邮箱和密码是必填项", http.StatusBadRequest)
		return
	}

	// 验证邮箱格式
	if !validator.IsValidEmail(req.Email) {
		response.ErrorResponse(w, "邮箱格式无效", http.StatusBadRequest)
		return
	}

	// 获取客户端 IP 并检查速率限制
	ip := getClientIP(r)
	if !loginAllowed(ip) {
		response.ErrorResponse(w, "登录尝试次数过多，请稍后再试", http.StatusTooManyRequests)
		return
	}

	// 验证用户凭据
	userID, err := db.ValidateUserCredentials(req.Email, req.Password)
	if err != nil {
		log.Printf("登录失败 %s: %v", maskEmail(req.Email), err)

		// 记录失败的登录尝试（仅失败消耗 IP 限额，成功登录不计数）
		recordLoginFailure(ip)
		if err := db.RecordFailedLogin(req.Email, maxLoginAttempts, lockoutDuration); err != nil {
			log.Printf("记录失败登录尝试错误: %v", err)
		}
		if err := db.LogLoginAttempt(req.Email, ip, false); err != nil {
			log.Printf("记录登录尝试错误: %v", err)
		}

		// 检查账户是否被锁定
		locked, lockedUntil, lockErr := db.IsAccountLocked(req.Email)
		if lockErr == nil && locked {
			response.ErrorResponse(w, fmt.Sprintf("账户已被锁定，将在 %s 后解锁", lockedUntil.Format("2006-01-02 15:04:05")), http.StatusLocked)
			return
		}

		response.ErrorResponse(w, "邮箱或密码错误", http.StatusUnauthorized)
		return
	}

	// 重置失败登录尝试
	if err := db.ResetFailedLogin(req.Email); err != nil {
		log.Printf("重置失败登录尝试错误: %v", err)
	}
	if err := db.LogLoginAttempt(req.Email, ip, true); err != nil {
		log.Printf("记录登录尝试错误: %v", err)
	}

	// 生成令牌
	accessToken, err := auth.GenerateAccessToken(userID, req.Email, accessTokenDuration)
	if err != nil {
		log.Printf("生成访问令牌失败: %v", err)
		response.ErrorResponse(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(userID, refreshTokenDuration)
	if err != nil {
		log.Printf("生成刷新令牌失败: %v", err)
		response.ErrorResponse(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}

	// 持久化刷新令牌
	expiresAt := time.Now().Add(refreshTokenDuration)
	if err := db.SaveRefreshToken(userID, refreshToken, expiresAt); err != nil {
		log.Printf("保存刷新令牌失败: %v", err)
	}

	// 在生产环境设置安全 cookie
	isSecure := os.Getenv("ENVIRONMENT") == "production"

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
		Expires:  expiresAt,
		SameSite: cookieSameSite,
	})

	response.SuccessResponse(w, map[string]interface{}{
		"access_token": accessToken,
		"expires_in":   int(accessTokenDuration.Seconds()),
	}, http.StatusOK)
}

func handleRefresh(w http.ResponseWriter, r *http.Request) {
	// 从 cookie 读取刷新令牌并验证
	c, err := r.Cookie("refresh_token")
	if err != nil {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}
	claims, err := auth.ValidateToken(c.Value)
	if err != nil || claims.TokenType != "refresh" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	// 原子认领刷新令牌：校验 + 撤销一步完成，防止并发重放同一令牌多次换发
	claimed, err := db.ConsumeRefreshToken(claims.UserID, c.Value)
	if err != nil || !claimed {
		response.ErrorResponse(w, "无效的刷新令牌", http.StatusUnauthorized)
		return
	}

	// 轮换令牌（时长与登录接口一致，均来自可配置环境变量）
	newAccess, err := auth.GenerateAccessToken(claims.UserID, claims.Email, accessTokenDuration)
	if err != nil {
		log.Printf("生成访问令牌失败: %v", err)
		response.ErrorResponse(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}

	newRefresh, err := auth.GenerateRefreshToken(claims.UserID, refreshTokenDuration)
	if err != nil {
		log.Printf("生成刷新令牌失败: %v", err)
		response.ErrorResponse(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}

	expiresAt := time.Now().Add(refreshTokenDuration)
	if err := db.SaveRefreshToken(claims.UserID, newRefresh, expiresAt); err != nil {
		log.Printf("保存刷新令牌失败: %v", err)
	}

	isSecure := os.Getenv("ENVIRONMENT") == "production"
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    newRefresh,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
		Expires:  expiresAt,
		SameSite: cookieSameSite,
	})

	response.SuccessResponse(w, map[string]interface{}{
		"access_token": newAccess,
		"expires_in":   int(accessTokenDuration.Seconds()),
	}, http.StatusOK)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	// 从上下文获取用户 ID
	userID := getUserIDFromContext(r.Context())
	if userID == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	// 将当前访问令牌加入黑名单：登出后立即失效，而非等待自然过期（至多 15 分钟）
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			if claims, err := auth.ValidateToken(parts[1]); err == nil && claims.TokenType == "access" {
				expiry := time.Now().Add(accessTokenDuration)
				if claims.ExpiresAt != nil {
					expiry = claims.ExpiresAt.Time
				}
				if err := db.BlacklistToken(parts[1], expiry); err != nil {
					log.Printf("加入令牌黑名单失败: %v", err)
				}
			}
		}
	}

	// 从 cookie 获取刷新令牌
	c, err := r.Cookie("refresh_token")
	if err == nil {
		// 在数据库中撤销令牌
		if err := db.RevokeRefreshToken(userID, c.Value); err != nil {
			log.Printf("撤销刷新令牌错误: %v", err)
		}
	}

	// 清除 cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   os.Getenv("ENVIRONMENT") == "production",
		MaxAge:   -1,
		SameSite: cookieSameSite,
	})

	response.SuccessResponse(w, map[string]string{"status": "已登出"}, http.StatusOK)
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromContext(r.Context())
	if userID == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	user, err := db.GetUserByID(userID)
	if err != nil {
		if strings.Contains(err.Error(), "用户不存在") {
			response.ErrorResponse(w, "用户不存在", http.StatusNotFound)
			return
		}
		response.ErrorResponse(w, "获取用户信息失败", http.StatusInternalServerError)
		return
	}

	response.SuccessResponse(w, user, http.StatusOK)
}

type task struct {
	ID          int    `json:"id,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
	Priority    string `json:"priority,omitempty"`
	DueAt       string `json:"due_at,omitempty"`
}
