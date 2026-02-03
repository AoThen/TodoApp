package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"todoapp/internal/auth"
	"todoapp/internal/crypto"
	"todoapp/internal/db"
	"todoapp/internal/response"
	"todoapp/internal/types"
	"todoapp/internal/utils"
	"todoapp/internal/validator"
	"todoapp/internal/websocket"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	wsclient "todoapp/internal/websocket"
)

const (
	// Security constants - should be overridden via environment variables in production
	defaultJWTSecret     = "change-this-in-production-use-environment-variable"
	maxRequestBodySize   = 10 * 1024 * 1024 // 10MB
	accessTokenDuration  = 15 * time.Minute
	refreshTokenDuration = 7 * 24 * time.Hour
	cookieSameSite       = http.SameSiteStrictMode
	maxLoginAttempts     = 5
	loginAttemptWindow   = 15 * time.Minute
)

// Rate limiting structures
var (
	loginAttempts = make(map[string][]time.Time)
	attemptsMutex sync.RWMutex
)

func init() {
	// Require JWT_SECRET from environment
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("错误: 必须设置 JWT_SECRET 环境变量")
	}

	// Validate minimum length
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		log.Fatal("错误: JWT_SECRET 至少需要 32 个字符")
	}

	// Initialize encryption module
	if err := crypto.Init(); err != nil {
		log.Fatalf("加密模块初始化失败: %v", err)
	}
	log.Println("加密模块已初始化")
}

// checkRateLimit 检查登录速率限制
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

// getClientIP 获取客户端 IP 地址
func getClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

// handleHealth 健康检查端点
func handleHealth(w http.ResponseWriter, r *http.Request) {
	response.SuccessResponse(w, map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	}, http.StatusOK)
}

func main() {
	// Load environment variables
	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "development"
		log.Println("未设置ENVIRONMENT，默认使用development模式")
	}

	// WebSocket 加密强制模式配置
	enforceWSEncryption := environment == "production"
	if envVal := os.Getenv("ENFORCE_WS_ENCRYPTION"); envVal != "" {
		enforceWSEncryption = envVal == "true"
	}
	log.Printf("环境: %s, WebSocket强制加密: %v", environment, enforceWSEncryption)

	// Initialize database with connection pool
	if err := db.InitDB("todoapp.db"); err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	log.Println("Database initialized.")

	// Initialize WebSocket Hub
	wsHub := websocket.NewHub()
	go wsHub.Run()
	log.Println("WebSocket Hub initialized.")

	// Start cleanup tasks
	go startCleanupTasks()
	log.Println("Cleanup tasks started.")

	// Create router with Gorilla Mux for better routing
	router := mux.NewRouter()

	// Apply security middleware
	router.Use(securityMiddleware)
	router.Use(headersMiddleware)

	// Health check endpoint (public)
	router.HandleFunc("/api/v1/health", handleHealth).Methods("GET")

	// Auth routes (public) - Registration disabled for admin-only user creation
	// router.HandleFunc("/api/v1/auth/register", handleRegister).Methods("POST")
	router.HandleFunc("/api/v1/auth/login", handleLogin).Methods("POST")
	router.HandleFunc("/api/v1/auth/refresh", handleRefresh).Methods("POST")
	router.HandleFunc("/api/v1/auth/logout", handleLogout).Methods("POST")

	// Protected routes
	protected := router.PathPrefix("/api/v1").Subrouter()
	protected.Use(authMiddleware)

	em := crypto.GetEncryptionMiddleware()
	protected.Use(em.DecryptRequest)
	protected.Use(em.EncryptResponse)

	protected.HandleFunc("/users/me", handleMe).Methods("GET")
	protected.HandleFunc("/tasks", handleTasks).Methods("GET", "POST")
	protected.HandleFunc("/tasks/{id}", handleTaskByID).Methods("GET", "PATCH", "DELETE")
	protected.HandleFunc("/tasks/batch", func(w http.ResponseWriter, r *http.Request) {
		handleBatchDeleteTasks(w, r, wsHub)
	}).Methods("DELETE")
	protected.HandleFunc("/tasks/{id}/restore", handleRestoreTask).Methods("POST")
	protected.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) { handleSync(w, r, wsHub) }).Methods("POST")
	protected.HandleFunc("/export", handleExport).Methods("GET")
	protected.HandleFunc("/notifications", handleGetNotifications).Methods("GET")
	protected.HandleFunc("/notifications", handleCreateNotification).Methods("POST")
	protected.HandleFunc("/notifications/{id}/read", handleMarkAsRead).Methods("PATCH")
	protected.HandleFunc("/notifications/read-all", handleMarkAllAsRead).Methods("PATCH")
	protected.HandleFunc("/notifications/{id}", handleDeleteNotification).Methods("DELETE")
	protected.HandleFunc("/notifications/clear", handleClearNotifications).Methods("DELETE")
	protected.HandleFunc("/notifications/unread-count", handleGetUnreadCount).Methods("GET")

	// Device pairing routes
	protected.HandleFunc("/devices/pair", handleDevicePairing).Methods("POST")
	protected.HandleFunc("/devices", handleListDevices).Methods("GET")
	protected.HandleFunc("/devices/{id}/regenerate", handleRegenerateDeviceKey).Methods("POST")
	protected.HandleFunc("/devices/{id}", handleRevokeDevice).Methods("DELETE")

	// Admin routes (requires authentication and admin role)
	admin := router.PathPrefix("/api/v1/admin").Subrouter()
	admin.Use(authMiddleware)
	admin.Use(adminMiddleware)

	admin.HandleFunc("/users", handleAdminListUsers).Methods("GET")
	admin.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r.Context())
		handleAdminCreateUserWithNotification(w, r, email, wsHub)
	}).Methods("POST")
	admin.HandleFunc("/users/{id}", handleAdminUpdateUser).Methods("PATCH")
	admin.HandleFunc("/users/{id}/password", handleAdminResetPassword).Methods("POST")
	admin.HandleFunc("/users/{id}/lock", handleAdminLockUser).Methods("POST")
	admin.HandleFunc("/users/{id}/unlock", handleAdminUnlockUser).Methods("POST")
	admin.HandleFunc("/users/{id}", handleAdminDeleteUser).Methods("DELETE")

	admin.HandleFunc("/logs/login", handleAdminGetLoginLogs).Methods("GET")
	admin.HandleFunc("/logs/actions", handleAdminGetActionLogs).Methods("GET")

	admin.HandleFunc("/config", handleAdminGetConfig).Methods("GET")
	admin.HandleFunc("/config", handleAdminSetConfig).Methods("PUT")

	// WebSocket endpoint (requires authentication, with optional encryption)
	// 应用WebSocket加密中间件
	wsMiddleware := webSocketEncryptionMiddleware(enforceWSEncryption)
	router.Handle("/ws", wsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(w, r, wsHub)
	}))).Methods("GET")

	// CORS handling for development
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"http://localhost:3000", "http://localhost:8080"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "X-User-ID"}),
		handlers.AllowCredentials(),
	)(router)

	// Configure server with timeouts
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      corsHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("Starting server on :8080...")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// securityMiddleware adds security headers and request validation
func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate content type for POST/PUT/PATCH
		if r.Method != "GET" && r.Method != "DELETE" {
			contentType := r.Header.Get("Content-Type")
			if !strings.Contains(contentType, "application/json") {
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
		ctx := context.WithValue(r.Context(), "userID", claims.UserID)
		ctx = context.WithValue(ctx, "email", claims.Email)
		ctx = context.WithValue(ctx, "role", role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// adminMiddleware checks if the user has admin role
func adminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, ok := r.Context().Value("role").(string)
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
	if id, ok := ctx.Value("userID").(string); ok {
		return id
	}
	return ""
}

func getRoleFromContext(ctx context.Context) string {
	if role, ok := ctx.Value("role").(string); ok {
		return role
	}
	return ""
}

// ---------------- handlers ----------------

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	// Placeholder: In real implementation, register user
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
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
	if !checkRateLimit(ip) {
		response.ErrorResponse(w, "登录尝试次数过多，请稍后再试", http.StatusTooManyRequests)
		return
	}

	// 验证用户凭据
	userID, err := db.ValidateUserCredentials(req.Email, req.Password)
	if err != nil {
		log.Printf("登录失败 %s: %v", req.Email, err)

		// 记录失败的登录尝试
		_ = db.RecordFailedLogin(req.Email)
		_ = db.LogLoginAttempt(req.Email, ip, false)

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
	_ = db.ResetFailedLogin(req.Email)
	_ = db.LogLoginAttempt(req.Email, ip, true)

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

	// 验证数据库中的令牌
	valid, err := db.ValidateRefreshToken(claims.UserID, c.Value)
	if err != nil || !valid {
		response.ErrorResponse(w, "无效的刷新令牌", http.StatusUnauthorized)
		return
	}

	// 轮换令牌
	newAccess, err := auth.GenerateAccessToken(claims.UserID, claims.Email, 15*time.Minute)
	if err != nil {
		log.Printf("生成访问令牌失败: %v", err)
		response.ErrorResponse(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}

	newRefresh, err := auth.GenerateRefreshToken(claims.UserID, 7*24*time.Hour)
	if err != nil {
		log.Printf("生成刷新令牌失败: %v", err)
		response.ErrorResponse(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}

	// 撤销旧令牌并保存新令牌
	if err := db.RevokeRefreshToken(claims.UserID, c.Value); err != nil {
		log.Printf("撤销刷新令牌失败: %v", err)
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
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
		"expires_in":   900,
	}, http.StatusOK)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	// 从上下文获取用户 ID
	userID := getUserIDFromContext(r.Context())
	if userID == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	// 从 cookie 获取刷新令牌
	c, err := r.Cookie("refresh_token")
	if err == nil {
		// 在数据库中撤销令牌
		_ = db.RevokeRefreshToken(userID, c.Value)
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
	json.NewEncoder(w).Encode(map[string]string{"id": "demo_user_id", "email": "test@example.com"})
}

type task struct {
	ID          int    `json:"id,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

func handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// 从上下文获取用户 ID
		userIDStr := getUserIDFromContext(r.Context())
		userID := 0
		if userIDStr != "" {
			userID, _ = strconv.Atoi(userIDStr)
		}

		// 解析分页参数
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}

		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		if pageSize < 1 || pageSize > 100 {
			pageSize = 20
		}

		tasks, total, err := db.GetTasksPaginated(userID, page, pageSize)
		if err != nil {
			response.ErrorResponse(w, "获取任务失败", http.StatusInternalServerError)
			return
		}

		response.SuccessResponse(w, map[string]interface{}{
			"tasks": tasks,
			"pagination": map[string]interface{}{
				"page":      page,
				"page_size": pageSize,
				"total":     total,
				"pages":     (total + pageSize - 1) / pageSize,
			},
		}, http.StatusOK)
		return
	}

	// POST: 创建任务（临时实现）
	var t task
	json.NewDecoder(r.Body).Decode(&t)
	t.ID = 1
	response.SuccessResponse(w, t, http.StatusCreated)
}

func handleTaskByID(w http.ResponseWriter, r *http.Request) {
	// path: /api/v1/tasks/{id}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
	if idStr == "" {
		http.NotFound(w, r)
		return
	}
	if _, err := strconv.Atoi(idStr); err != nil {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

type syncReq struct {
	LastSyncAt string `json:"last_sync_at"`
	Changes    []struct {
		LocalID string                 `json:"local_id"`
		Op      string                 `json:"op"`
		Payload map[string]interface{} `json:"payload"`
		CV      int                    `json:"client_version"`
	} `json:"changes"`
}

func handleSync(w http.ResponseWriter, r *http.Request, wsHub *wsclient.Hub) {
	var s syncReq
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		response.ErrorResponse(w, "错误的请求", http.StatusBadRequest)
		return
	}

	// 从上下文获取用户 ID（已认证的用户）
	userIDStr := getUserIDFromContext(r.Context())
	if userIDStr == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		response.ErrorResponse(w, "无效的用户 ID", http.StatusUnauthorized)
		return
	}

	now := time.Now().UTC()

	// ✅ 使用事务保护同步操作
	tx, err := db.DB.Begin()
	if err != nil {
		log.Printf("事务开始失败: %v", err)
		response.ErrorResponse(w, "事务开始失败", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	serverChanges := []map[string]interface{}{}
	clientChanges := []map[string]interface{}{}
	conflicts := []map[string]interface{}{}
	syncFailed := false

	for _, c := range s.Changes {
		op := strings.ToLower(c.Op)

		switch op {
		case "insert":
			title := ""
			description := ""
			status := "todo"
			priority := "medium"

			if v, ok := c.Payload["title"].(string); ok {
				title = v
			}
			if v, ok := c.Payload["description"].(string); ok {
				description = v
			}
			if v, ok := c.Payload["status"].(string); ok {
				status = v
			}
			if v, ok := c.Payload["priority"].(string); ok {
				priority = v
			}

			// ✅ 检查 local_id 是否已存在（冲突检测）
			existingID, _, checkErr := db.TaskExistsByLocalID(userID, c.LocalID)
			if checkErr == nil && existingID > 0 {
				// 冲突：local_id重复，使用生成的新标题插入
				newTitle := fmt.Sprintf("%s (副本: %d)", title, existingID)
				res, insertErr := tx.Exec(
					"INSERT INTO tasks (user_id, local_id, server_version, title, description, status, priority, created_at, updated_at, last_modified) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					userID, c.LocalID+"_"+randomString(8), 1, newTitle, description, status, priority, now, now, now,
				)

				if insertErr == nil {
					if serverID, err2 := res.LastInsertId(); err2 == nil {
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": serverID, "server_version": 1, "title": newTitle, "updated_at": now.Format(time.RFC3339), "is_deleted": false,
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": serverID, "op": "insert",
						})

						// 记录冲突
						recordConflict(c.LocalID, existingID, "duplicate_insert", userID)
						conflicts = append(conflicts, map[string]interface{}{
							"local_id":  c.LocalID,
							"server_id": existingID,
							"reason":    "duplicate_insert",
						})
					}
				} else {
					log.Printf("插入副本失败: %v", insertErr)
					syncFailed = true
				}
			} else {
				// 正常插入
				res, insertErr := tx.Exec(
					"INSERT INTO tasks (user_id, local_id, server_version, title, description, status, priority, created_at, updated_at, last_modified) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					userID, c.LocalID, 1, title, description, status, priority, now, now, now,
				)

				if insertErr == nil {
					if serverID, err2 := res.LastInsertId(); err2 == nil {
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": serverID, "server_version": 1, "title": title, "updated_at": now.Format(time.RFC3339),
							"description": description, "status": status, "priority": priority, "is_deleted": false,
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": serverID, "op": "insert",
						})
					}
				} else {
					log.Printf("插入任务失败: %v", insertErr)
					syncFailed = true
				}
			}

		case "update":
			if idVal, ok := c.Payload["id"].(float64); ok {
				id := int64(idVal)

				// ✅ 获取服务器当前版本和数据
				oldVer, serverTitle, serverDesc, serverStatus, _, err := db.GetTaskForSync(id)

				if err != nil {
					log.Printf("查询任务失败: %v", err)
					syncFailed = true
					continue
				}

				newVer := oldVer + 1

				// ✅ 版本检查和冲突检测
				if c.CV != oldVer {
					log.Printf("检测到冲突: client_version=%d, server_version=%d", c.CV, oldVer)

					// 获取客户端更新
					clientTitle := ""
					clientDesc := ""
					clientStatus := serverStatus

					if v, ok := c.Payload["title"].(string); ok {
						clientTitle = v
					}
					if v, ok := c.Payload["description"].(string); ok {
						clientDesc = v
					}
					if v, ok := c.Payload["status"].(string); ok {
						clientStatus = v
					}

					// ✅ 智能合并
					mergedTitle, mergedDesc, mergedStatus := intelligentMerge(serverTitle, serverDesc, serverStatus, map[string]interface{}{
						"title":       clientTitle,
						"description": clientDesc,
						"status":      clientStatus,
					})

					// 应用合并结果
					priority := "medium"
					if v, ok := c.Payload["priority"].(string); ok {
						priority = v
					}

					updateErr := db.UpdateTaskWithVersion(id, mergedTitle, mergedDesc, mergedStatus, priority, newVer)
					if updateErr != nil {
						log.Printf("应用合并结果失败: %v", updateErr)
						syncFailed = true
					} else {
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": id, "server_version": newVer, "title": mergedTitle, "description": mergedDesc,
							"status": mergedStatus, "priority": priority, "updated_at": now.Format(time.RFC3339), "is_deleted": false,
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": id, "op": "update",
						})

						// 记录冲突
						recordConflict(c.LocalID, id, "intelligent_merge", userID)
						conflicts = append(conflicts, map[string]interface{}{
							"local_id":   c.LocalID,
							"server_id":  id,
							"reason":     "intelligent_merge",
							"resolution": "智能合并",
							"merged_data": map[string]interface{}{
								"title":       mergedTitle,
								"description": mergedDesc,
								"status":      mergedStatus,
							},
						})
					}
				} else {
					// 正常更新
					title := serverTitle
					description := serverDesc
					status := serverStatus
					priority := "medium"

					if v, ok := c.Payload["title"].(string); ok {
						title = v
					}
					if v, ok := c.Payload["description"].(string); ok {
						description = v
					}
					if v, ok := c.Payload["status"].(string); ok {
						status = v
					}
					if v, ok := c.Payload["priority"].(string); ok {
						priority = v
					}

					updateErr := db.UpdateTaskWithVersion(id, title, description, status, priority, newVer)
					if updateErr != nil {
						log.Printf("更新任务失败: %v", updateErr)
						syncFailed = true
					} else {
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": id, "server_version": newVer, "title": title, "updated_at": now.Format(time.RFC3339),
							"description": description, "status": status, "priority": priority, "is_deleted": false,
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": id, "op": "update",
						})
					}
				}
			}

		case "delete":
			if idVal, ok := c.Payload["id"].(float64); ok {
				id := int64(idVal)

				oldVer, _, _, _, isDeleted, err := db.GetTaskForSync(id)

				if err != nil {
					log.Printf("查询任务失败: %v", err)
					syncFailed = true
					continue
				}

				newVer := oldVer + 1

				// ✅ 版本检查和冲突检测
				if c.CV != oldVer {
					log.Printf("删除冲突检测: client_version=%d, server_version=%d", c.CV, oldVer)

					if isDeleted {
						// 服务器已删除，忽略客户端删除
						continue
					} else {
						// 冲突：客户端想删除但服务器有更新
						// 策略：软删除，记录冲突
						deleteErr := db.SoftDeleteTaskWithVersion(id, newVer)
						if deleteErr != nil {
							log.Printf("删除任务失败: %v", deleteErr)
							syncFailed = true
						} else {
							serverChanges = append(serverChanges, map[string]interface{}{
								"id": id, "server_version": newVer, "is_deleted": true, "updated_at": now.Format(time.RFC3339),
							})
							clientChanges = append(clientChanges, map[string]interface{}{
								"local_id": c.LocalID, "server_id": id, "op": "delete",
							})

							// 记录冲突：服务器被标记为删除
							recordConflict(c.LocalID, id, "delete_while_modified", userID)
							conflicts = append(conflicts, map[string]interface{}{
								"local_id":  c.LocalID,
								"server_id": id,
								"reason":    "delete_while_modified",
							})
						}
					}
				} else {
					// 正常删除
					deleteErr := db.SoftDeleteTaskWithVersion(id, newVer)
					if deleteErr != nil {
						log.Printf("删除任务失败: %v", deleteErr)
						syncFailed = true
					} else {
						serverChanges = append(serverChanges, map[string]interface{}{
							"id": id, "server_version": newVer, "is_deleted": true, "updated_at": now.Format(time.RFC3339),
						})
						clientChanges = append(clientChanges, map[string]interface{}{
							"local_id": c.LocalID, "server_id": id, "op": "delete",
						})
					}
				}
			}
		}
	}

	// 发送同步结果通知
	if syncFailed {
		sendNotificationToUser(userID, "sync_failed", "同步失败", "部分任务同步失败，请检查网络连接", "high", wsHub)
	} else if len(conflicts) > 0 {
		sendNotificationToUser(userID, "sync_conflict", "同步完成但存在冲突", fmt.Sprintf("成功同步 %d 个任务，但检测到 %d 个冲突", len(clientChanges), len(conflicts)), "high", wsHub)
	} else if len(clientChanges) > 0 {
		sendNotificationToUser(userID, "sync_success", "同步完成", fmt.Sprintf("成功同步 %d 个任务", len(clientChanges)), "normal", wsHub)
	}

	resp := map[string]interface{}{
		"server_changes": serverChanges,
		"client_changes": clientChanges,
		"last_sync_at":   now.Format(time.RFC3339),
		"conflicts":      conflicts, // ✅ 返回实际冲突
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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
	const charset = "0123456789abcdef"
	b := make([]byte, 64)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%16]
	}
	return string(b)
}

// handleBatchDeleteTasks 批量删除任务
func handleBatchDeleteTasks(w http.ResponseWriter, r *http.Request, wsHub *wsclient.Hub) {
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
		TaskIDs []int64 `json:"task_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, "无效的请�数据", http.StatusBadRequest)
		return
	}

	if len(req.TaskIDs) == 0 {
		response.ErrorResponse(w, "未选择要删除的任务", http.StatusBadRequest)
		return
	}

	// 批量软删除任务
	count, err := db.BatchDeleteTasks(userID, req.TaskIDs)
	if err != nil {
		log.Printf("批量删除失败: %v", err)
		response.ErrorResponse(w, "批量删除失败", http.StatusInternalServerError)
		return
	}

	// 发送通知
	sendNotificationToUser(userID, "tasks_deleted", "任务已删除", fmt.Sprintf("已删除 %d 个任务，30秒内可撤销", count), "normal", wsHub)

	response.SuccessResponse(w, map[string]interface{}{
		"status":              "deleted",
		"count":               count,
		"can_undo":            true,
		"undo_window_seconds": 30,
	}, http.StatusOK)
}

// handleRestoreTask 恢复删除的任务（撤销）
func handleRestoreTask(w http.ResponseWriter, r *http.Request) {
	userIDStr := getUserIDFromContext(r.Context())
	if userIDStr == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	taskID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		response.ErrorResponse(w, "无效的任务ID", http.StatusBadRequest)
		return
	}

	userID, _ := strconv.Atoi(userIDStr)

	// 恢复任务
	err = db.RestoreDeletedTask(taskID, userID)
	if err != nil {
		if _, ok := err.(*db.UndoExpiredError); ok {
			response.ErrorResponse(w, "已超过30秒撤销期限", http.StatusGone)
		} else {
			response.ErrorResponse(w, "恢复失败", http.StatusInternalServerError)
		}
		return
	}

	response.SuccessResponse(w, map[string]string{
		"status": "restored",
	}, http.StatusOK)
}

// handleExport 导出数据（任务）为 JSON 或 CSV 格式
func handleExport(w http.ResponseWriter, r *http.Request) {
	// 从上下文获取用户 ID
	userID := getUserIDFromContext(r.Context())
	if userID == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
		return
	}

	userIDInt, err := strconv.Atoi(userID)
	if err != nil {
		response.ErrorResponse(w, "无效的用户ID", http.StatusBadRequest)
		return
	}

	t := r.URL.Query().Get("type")
	if t != "tasks" {
		response.ErrorResponse(w, "不支持的导出类型", http.StatusBadRequest)
		return
	}

	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		format = "json"
	}

	// 获取当前用户的任务（修复：正确传递userID）
	tasks, _, err := db.GetTasksPaginated(userIDInt, 1, 10000)
	if err != nil {
		response.ErrorResponse(w, "导出错误", http.StatusInternalServerError)
		return
	}

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment;filename=tasks.json")

		// 记录导出审计日志
		_ = db.LogExportAction(userIDInt, "tasks", format, len(tasks))

		json.NewEncoder(w).Encode(tasks)
		return
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment;filename=tasks.csv")

		// 写入UTF-8 BOM（Excel兼容）
		w.Write([]byte{0xEF, 0xBB, 0xBF})

		// 使用CSVStreamer进行流式写入
		streamer := utils.NewCSVStreamer(w)
		defer streamer.Close()

		headers := []string{
			"id", "local_id", "server_version", "title", "description",
			"status", "priority", "due_at", "created_at", "updated_at",
			"completed_at", "is_deleted", "last_modified",
		}
		if err := streamer.WriteHeader(headers); err != nil {
			log.Printf("写入CSV表头失败: %v", err)
			return
		}

		for _, task := range tasks {
			if err := streamer.WriteRow(task); err != nil {
				log.Printf("写入CSV行失败: %v", err)
				continue
			}
		}

		// 记录导出审计日志
		_ = db.LogExportAction(userIDInt, "tasks", format, len(tasks))

		log.Printf("用户 %d 导出了 %d 个任务到 %s 格式", userIDInt, len(tasks), format)
		return
	}

	response.ErrorResponse(w, "未知的格式", http.StatusBadRequest)
}

// intelligentMerge 策略：字段级冲突智能合并
func intelligentMerge(serverTitle, serverDesc, serverStatus string, clientProps map[string]interface{}) (string, string, string) {
	mergedTitle := serverTitle
	mergedDesc := serverDesc
	mergedStatus := serverStatus

	// 标题：优先服务器
	if clientTitle, ok := clientProps["title"].(string); ok && clientTitle != "" {
		if clientTitle != serverTitle {
			mergedTitle = serverTitle
		} else {
			mergedTitle = clientTitle
		}
	}

	// 描述：合并两者的描述
	if clientDesc, ok := clientProps["description"].(string); ok {
		if clientDesc != serverDesc && serverDesc != "" && clientDesc != "" {
			mergedDesc = fmt.Sprintf("[服务器更新] %s\n\n[客户端更新] %s", serverDesc, clientDesc)
		} else if clientDesc != "" {
			mergedDesc = clientDesc
		}
	}

	// 状态：保留较新的状态
	if clientStatus, ok := clientProps["status"].(string); ok {
		if clientStatus != serverStatus {
			mergedStatus = types.PrioritizeStatus(clientStatus, serverStatus)
		}
	}

	return mergedTitle, mergedDesc, mergedStatus
}

// recordConflict 记录冲突到数据库
func recordConflict(localID string, serverID int64, reason string, userID int) {
	options := []types.ConflictResolution{types.ConflictKeepServer, types.ConflictKeepClient, types.ConflictMerge}
	optionsJSON, _ := json.Marshal(options)
	_ = db.RecordConflict(localID, serverID, reason, string(optionsJSON))
}

// randomString 生成随机字符串（用于local_id冲突处理）
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}

// ============ Admin Handlers ============

type adminUserResponse struct {
	ID                 int64  `json:"id"`
	Email              string `json:"email"`
	Role               string `json:"role"`
	FailedAttempts     int    `json:"failed_attempts"`
	LockedUntil        string `json:"locked_until,omitempty"`
	MustChangePassword bool   `json:"must_change_password"`
	CreatedAt          string `json:"created_at"`
	IsLocked           bool   `json:"is_locked"`
}

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

func handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := db.GetAllUsers()
	if err != nil {
		log.Printf("Failed to get users: %v", err)
		response.ErrorResponse(w, "获取用户列表失败", http.StatusInternalServerError)
		return
	}
	response.SuccessResponse(w, users, http.StatusOK)
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

	userID, err := db.CreateUser(req.Email, req.Password, req.Role)
	if err != nil {
		response.ErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	adminID := getUserIDFromContext(r.Context())
	ip := getClientIP(r)
	_ = db.LogAdminAction(toInt(adminID), adminEmail, "create_user", req.Email, userID, fmt.Sprintf("Created user with role: %s", req.Role), ip)

	// 发送通知给新用户
	roleName := map[string]string{"admin": "管理员", "user": "普通用户"}[req.Role]
	sendNotificationToUser(int(userID), "account_created", "账户已创建", fmt.Sprintf("您已被创建为%s", roleName), "normal", wsHub)

	log.Printf("Admin %s created user %s (ID: %d)", adminEmail, req.Email, userID)
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
		response.ErrorResponse(w, "更新用户失败", http.StatusInternalServerError)
		return
	}

	adminID := getUserIDFromContext(r.Context())
	adminEmail := getEmailFromContext(r.Context())
	ip := getClientIP(r)
	_ = db.LogAdminAction(toInt(adminID), adminEmail, "update_user", req.Email, userID,
		fmt.Sprintf("Updated email: %s -> %s, role: %s", oldEmail, req.Email, req.Role), ip)

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

	if err := db.ResetUserPassword(userID, req.NewPassword); err != nil {
		response.ErrorResponse(w, "重置密码失败", http.StatusInternalServerError)
		return
	}

	targetEmail, _ := db.GetUserEmail(userID)
	adminID := getUserIDFromContext(r.Context())
	adminEmail := getEmailFromContext(r.Context())
	ip := getClientIP(r)
	_ = db.LogAdminAction(toInt(adminID), adminEmail, "reset_password", targetEmail, userID, "Password reset by admin", ip)

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

	targetEmail, _ := db.GetUserEmail(userID)
	adminID := getUserIDFromContext(r.Context())
	adminEmail := getEmailFromContext(r.Context())
	ip := getClientIP(r)
	_ = db.LogAdminAction(toInt(adminID), adminEmail, "lock_user", targetEmail, userID,
		fmt.Sprintf("Locked for %d minutes", req.DurationMinutes), ip)

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

	targetEmail, _ := db.GetUserEmail(userID)
	adminID := getUserIDFromContext(r.Context())
	adminEmail := getEmailFromContext(r.Context())
	ip := getClientIP(r)
	_ = db.LogAdminAction(toInt(adminID), adminEmail, "unlock_user", targetEmail, userID, "Account unlocked by admin", ip)

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
	_ = db.LogAdminAction(toInt(adminID), adminEmail, "delete_user", targetEmail, userID, "User deleted by admin", ip)

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
	_ = db.LogAdminAction(toInt(adminID), adminEmail, "update_config", "", 0, fmt.Sprintf("Updated config: %s = %s", req.Key, req.Value), ip)

	response.SuccessResponse(w, map[string]string{"status": "config updated"}, http.StatusOK)
}

// Helper functions
func getEmailFromContext(ctx context.Context) string {
	if email, ok := ctx.Value("email").(string); ok {
		return email
	}
	return ""
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

type notificationUpdateRequest struct {
	IsRead *bool `json:"is_read"`
}

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
		olderThanDays, _ = strconv.Atoi(olderThanDaysStr)
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
func startCleanupTasks() {
	// 每小时清理一次过期通知和令牌
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			cleanupExpiredNotifications()
			cleanupExpiredTokens()
		}
	}()

	// 每天清理一次旧日志和删除记录
	dailyTicker := time.NewTicker(24 * time.Hour)
	go func() {
		for range dailyTicker.C {
			db.CleanupOldLoginLogs()
			count, _ := db.CleanupOldDeletedTasks()
			if count > 0 {
				log.Printf("Cleaned up %d old deleted_task records", count)
			}
		}
	}()
}

// cleanupExpiredNotifications 清理过期通知
func cleanupExpiredNotifications() {
	count, err := db.CleanupExpiredNotifications()
	if err != nil {
		log.Printf("Failed to cleanup expired notifications: %v", err)
	} else if count > 0 {
		log.Printf("Cleaned up %d expired notifications", count)
	}
}

// cleanupExpiredTokens 清理过期令牌
func cleanupExpiredTokens() {
	err := db.CleanupExpiredTokens()
	if err != nil {
		log.Printf("Failed to cleanup expired tokens: %v", err)
	}
}

// sendNotificationToUser 发送通知给用户（数据库 + WebSocket）
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
