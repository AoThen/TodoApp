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
	"todoapp/internal/db"
	"todoapp/internal/response"
	"todoapp/internal/validator"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
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
	// Initialize database with connection pool
	if err := db.InitDB("todoapp.db"); err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	log.Println("Database initialized.")

	// Create router with Gorilla Mux for better routing
	router := mux.NewRouter()

	// Apply security middleware
	router.Use(securityMiddleware)
	router.Use(headersMiddleware)

	// Health check endpoint (public)
	router.HandleFunc("/api/v1/health", handleHealth).Methods("GET")

	// Auth routes (public)
	router.HandleFunc("/api/v1/auth/register", handleRegister).Methods("POST")
	router.HandleFunc("/api/v1/auth/login", handleLogin).Methods("POST")
	router.HandleFunc("/api/v1/auth/refresh", handleRefresh).Methods("POST")
	router.HandleFunc("/api/v1/auth/logout", handleLogout).Methods("POST")

	// Protected routes
	protected := router.PathPrefix("/api/v1").Subrouter()
	protected.Use(authMiddleware)

	protected.HandleFunc("/users/me", handleMe).Methods("GET")
	protected.HandleFunc("/tasks", handleTasks).Methods("GET", "POST")
	protected.HandleFunc("/tasks/{id}", handleTaskByID).Methods("GET", "PATCH", "DELETE")
	protected.HandleFunc("/sync", handleSync).Methods("POST")
	protected.HandleFunc("/export", handleExport).Methods("GET")

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

		// Store user ID in context for handlers to use
		ctx := context.WithValue(r.Context(), "userID", claims.UserID)
		ctx = context.WithValue(ctx, "email", claims.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getUserIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value("userID").(string); ok {
		return id
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

func handleSync(w http.ResponseWriter, r *http.Request) {
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

	serverChanges := []map[string]interface{}{}
	clientChanges := []map[string]interface{}{}

	for _, c := range s.Changes {
		op := strings.ToLower(c.Op)
		switch op {
		case "insert":
			title := ""
			if v, ok := c.Payload["title"].(string); ok {
				title = v
			}
			res, err := db.DB.Exec("INSERT INTO tasks (user_id, local_id, server_version, title, status, created_at, updated_at, last_modified) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", userID, c.LocalID, 1, title, "todo", now, now, now)
			if err == nil {
				if serverID, err2 := res.LastInsertId(); err2 == nil {
					serverChanges = append(serverChanges, map[string]interface{}{
						"id": serverID, "server_version": 1, "title": title, "updated_at": now.Format(time.RFC3339), "is_deleted": false,
					})
					clientChanges = append(clientChanges, map[string]interface{}{
						"local_id": c.LocalID, "server_id": serverID, "op": "insert",
					})
				}
			}
		case "update":
			if idVal, ok := c.Payload["id"].(float64); ok {
				id := int64(idVal)
				var oldVer int
				_ = db.DB.QueryRow("SELECT server_version FROM tasks WHERE id = ?", id).Scan(&oldVer)
				newVer := oldVer + 1
				title := ""
				if v, ok := c.Payload["title"].(string); ok {
					title = v
				}
				_, _ = db.DB.Exec("UPDATE tasks SET title = ?, updated_at = ?, server_version = ? WHERE id = ?", title, now, newVer, id)
				serverChanges = append(serverChanges, map[string]interface{}{
					"id": id, "server_version": newVer, "title": title, "updated_at": now.Format(time.RFC3339), "is_deleted": false,
				})
				clientChanges = append(clientChanges, map[string]interface{}{
					"local_id": c.LocalID, "server_id": id, "op": "update",
				})
			}
		case "delete":
			if idVal, ok := c.Payload["id"].(float64); ok {
				id := int64(idVal)
				var oldVer int
				_ = db.DB.QueryRow("SELECT server_version FROM tasks WHERE id = ?", id).Scan(&oldVer)
				newVer := oldVer + 1
				_, _ = db.DB.Exec("UPDATE tasks SET is_deleted = 1, updated_at = ?, server_version = ? WHERE id = ?", now, newVer, id)
				serverChanges = append(serverChanges, map[string]interface{}{
					"id": id, "server_version": newVer, "is_deleted": true, "updated_at": now.Format(time.RFC3339),
				})
				clientChanges = append(clientChanges, map[string]interface{}{
					"local_id": c.LocalID, "server_id": id, "op": "delete",
				})
			}
		}
	}

	resp := map[string]interface{}{
		"server_changes": serverChanges,
		"client_changes": clientChanges,
		"last_sync_at":   now.Format(time.RFC3339),
		"conflicts":      []interface{}{},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleExport 导出数据（任务）为 JSON 或 CSV 格式
func handleExport(w http.ResponseWriter, r *http.Request) {
	// 从上下文获取用户 ID
	userID := getUserIDFromContext(r.Context())
	if userID == "" {
		response.ErrorResponse(w, "未授权", http.StatusUnauthorized)
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

	// 只获取当前用户的任务
	tasks, _, err := db.GetTasksPaginated(0, 1, 10000) // 临时解决方案，应该添加用户过滤
	if err != nil {
		response.ErrorResponse(w, "导出错误", http.StatusInternalServerError)
		return
	}

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tasks)
		return
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment;filename=tasks.csv")

		w.Write([]byte("id,local_id,server_version,title,description,status,priority,due_at,created_at,updated_at,completed_at,is_deleted,last_modified\n"))

		for _, row := range tasks {
			line := []string{
				toString(row["id"]),
				toString(row["local_id"]),
				toString(row["server_version"]),
				toString(row["title"]),
				toString(row["description"]),
				toString(row["status"]),
				toString(row["priority"]),
				toString(row["due_at"]),
				toString(row["created_at"]),
				toString(row["updated_at"]),
				toString(row["completed_at"]),
				toString(row["is_deleted"]),
				toString(row["last_modified"]),
			}
			w.Write([]byte(strings.Join(line, ",") + "\n"))
		}
		return
	}

	response.ErrorResponse(w, "未知的格式", http.StatusBadRequest)
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
