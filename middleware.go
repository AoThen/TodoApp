package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

import (
	"todoapp/internal/auth"
	"todoapp/internal/db"
	"todoapp/internal/response"
)

// contextKey 自定义 context key 类型，避免使用内置 string 作为 key 产生碰撞
type contextKey string

const (
	ctxKeyUserID contextKey = "userID"
	ctxKeyEmail  contextKey = "email"
	ctxKeyRole   contextKey = "role"
)

func checkRateLimit(ip string) bool {
	attemptsMutex.Lock()
	defer attemptsMutex.Unlock()

	now := time.Now()

	// 清理旧的尝试记录
	var validAttempts []time.Time
	for _, t := range loginAttempts[ip] {
		if now.Sub(t) < loginAttemptWindow {
			validAttempts = append(validAttempts, t)
		}
	}

	if len(validAttempts) >= maxLoginAttempts {
		return false
	}

	loginAttempts[ip] = append(validAttempts, now)
	return true
}

// Global rate limiting structures
var (
	ipRequests      = make(map[string][]time.Time)
	ipMutex         sync.RWMutex
	globalRateLimit = getRateLimitFromEnv()
	rateLimitWindow = time.Minute
)

func getRateLimitFromEnv() int {
	defaultLimit := 600
	envLimit := os.Getenv("RATE_LIMIT_PER_MINUTE")
	if envLimit != "" {
		limit, err := strconv.Atoi(envLimit)
		if err != nil || limit <= 0 {
			log.Printf("RATE_LIMIT_PER_MINUTE 环境变量无效，使用默认值: %d", defaultLimit)
			return defaultLimit
		}
		log.Printf("全局限流已启用: 每分钟 %d 次请求", limit)
		return limit
	}
	log.Printf("全局限流已启用: 每分钟 %d 次请求 (默认)", defaultLimit)
	return defaultLimit
}

func cleanupIPRequests() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ipMutex.Lock()
		now := time.Now()
		activeCount := 0

		for ip, times := range ipRequests {
			hasActive := false
			for _, t := range times {
				if now.Sub(t) < rateLimitWindow {
					hasActive = true
					break
				}
			}
			if hasActive {
				activeCount++
			} else {
				delete(ipRequests, ip)
			}
		}
		ipMutex.Unlock()

		log.Printf("IP限流清理完成，活跃IP数: %d", activeCount)
	}
}

// globalRateLimitMiddleware 全局请求限流中间件

func globalRateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)

		ipMutex.Lock()
		defer ipMutex.Unlock()

		now := time.Now()

		// 清理过期请求记录
		var validRequests []time.Time
		for _, t := range ipRequests[ip] {
			if now.Sub(t) < rateLimitWindow {
				validRequests = append(validRequests, t)
			}
		}

		if len(validRequests) >= globalRateLimit {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "请求过于频繁，请稍后再试",
			})
			return
		}

		ipRequests[ip] = append(validRequests, now)
		next.ServeHTTP(w, r)
	})
}

// trustedProxy 是否信任反向代理的 X-Forwarded-For 头。
// 仅当后端部署在可信反向代理（如 Nginx）之后时设为 true；直连时必须为 false，
// 否则客户端可伪造 X-Forwarded-For 绕过基于 IP 的限流与审计。
var trustedProxy = os.Getenv("TRUSTED_PROXY") == "true"

// getClientIP 获取客户端 IP 地址
func getClientIP(r *http.Request) string {
	if trustedProxy {
		forwarded := r.Header.Get("X-Forwarded-For")
		if forwarded != "" {
			ips := strings.Split(forwarded, ",")
			if len(ips) > 0 {
				return strings.TrimSpace(ips[0])
			}
		}
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

// handleHealth 健康检查端点

func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate content type for POST/PUT/PATCH（导入接口使用 multipart 上传，需放行）
		if r.Method != "GET" && r.Method != "DELETE" {
			contentType := r.Header.Get("Content-Type")
			if !strings.Contains(contentType, "application/json") && !strings.Contains(contentType, "multipart/form-data") {
				http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
				return
			}
		}

		// Validate request size
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

		next.ServeHTTP(w, r)
	})
}

// headersMiddleware adds security headers

func headersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")

		// 请求 ID 端到端追踪：客户端携带 X-Request-ID 时原样回写，便于联调与日志关联
		if reqID := r.Header.Get("X-Request-ID"); reqID != "" {
			w.Header().Set("X-Request-ID", reqID)
		}

		// Cache control for API endpoints
		if strings.HasPrefix(r.URL.Path, "/api/v1/") {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
		}

		next.ServeHTTP(w, r)
	})
}

// authMiddleware validates JWT tokens

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
			return
		}

		token := parts[1]
		claims, err := auth.ValidateToken(token)
		if err != nil {
			log.Printf("Token validation failed: %v", err)
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Get user role from database
		role, err := db.GetUserRole(claims.UserID)
		if err != nil {
			log.Printf("Failed to get user role: %v", err)
			role = "user"
		}

		// Store user ID, email and role in context for handlers to use
		ctx := context.WithValue(r.Context(), ctxKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, ctxKeyEmail, claims.Email)
		ctx = context.WithValue(ctx, ctxKeyRole, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// adminMiddleware checks if the user has admin role

func adminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, ok := r.Context().Value(ctxKeyRole).(string)
		if !ok || role != "admin" {
			log.Printf("Unauthorized admin access attempt from user with role: %v", role)
			response.ErrorResponse(w, "禁止访问：需要管理员权限", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// webSocketEncryptionMiddleware 验证WebSocket加密设置

func webSocketEncryptionMiddleware(enforceEncryption bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			encryptionEnabled := r.URL.Query().Get("encryption") == "true"

			if enforceEncryption && !encryptionEnabled {
				log.Printf("拒绝非加密WebSocket连接（生产环境强制加密）: %s", r.RemoteAddr)
				response.ErrorResponse(w, "生产环境必须启用加密WebSocket连接", http.StatusForbidden)
				return
			}

			if !encryptionEnabled {
				log.Printf("警告：WebSocket连接未加密（%s）", r.RemoteAddr)
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getUserIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(ctxKeyUserID).(string); ok {
		return id
	}
	return ""
}

func getRoleFromContext(ctx context.Context) string {
	if role, ok := ctx.Value(ctxKeyRole).(string); ok {
		return role
	}
	return ""
}

// ---------------- handlers ----------------

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func getEmailFromContext(ctx context.Context) string {
	if email, ok := ctx.Value(ctxKeyEmail).(string); ok {
		return email
	}
	return ""
}
