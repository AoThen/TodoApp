package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

import (
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

import (
	"todoapp/internal/crypto"
	"todoapp/internal/db"
	"todoapp/internal/response"
	"todoapp/internal/websocket"
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

func init() {
	go cleanupIPRequests()
}

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
	dbPath := os.Getenv("DATABASE_URL")
	if dbPath == "" {
		dbPath = "todoapp.db"
	}
	if err := db.InitDB(dbPath); err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	log.Printf("Database initialized (%s).", dbPath)

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
	router.Use(globalRateLimitMiddleware)

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
	// 具体路径必须注册在 {id} 之前，否则会被 {id} 捕获（如 /tasks/batch）
	protected.HandleFunc("/tasks/batch", func(w http.ResponseWriter, r *http.Request) {
		handleBatchDeleteTasks(w, r, wsHub)
	}).Methods("DELETE")
	protected.HandleFunc("/tasks/{id}", handleTaskByID).Methods("GET", "PATCH", "DELETE")
	protected.HandleFunc("/tasks/{id}/restore", handleRestoreTask).Methods("POST")
	protected.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) { handleSync(w, r, wsHub) }).Methods("POST")
	protected.HandleFunc("/export", handleExport).Methods("GET")
	protected.HandleFunc("/import", handleImport).Methods("POST")
	protected.HandleFunc("/notifications", handleGetNotifications).Methods("GET")
	protected.HandleFunc("/notifications", handleCreateNotification).Methods("POST")
	// 具体路径必须注册在 {id} 之前，否则会被 {id} 捕获
	protected.HandleFunc("/notifications/read-all", handleMarkAllAsRead).Methods("PATCH")
	protected.HandleFunc("/notifications/unread-count", handleGetUnreadCount).Methods("GET")
	protected.HandleFunc("/notifications/clear", handleClearNotifications).Methods("DELETE")
	protected.HandleFunc("/notifications/{id}/read", handleMarkAsRead).Methods("PATCH")
	protected.HandleFunc("/notifications/{id}", handleDeleteNotification).Methods("DELETE")

	// Device pairing routes
	protected.HandleFunc("/devices/pair/init", handleInitPairing).Methods("POST")
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

	// 读取服务器端口配置（支持 SERVER_PORT 环境变量，默认 8080）
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}
	if _, err := strconv.Atoi(serverPort); err != nil {
		log.Printf("SERVER_PORT 环境变量无效（%q），使用默认端口 8080", serverPort)
		serverPort = "8080"
	}
	serverAddr := ":" + serverPort

	// CORS handling for development
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"http://localhost:3000", "http://localhost:" + serverPort}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "X-User-ID"}),
		handlers.AllowCredentials(),
	)(router)

	// Configure server with timeouts
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      corsHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting server on %s...", serverAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// securityMiddleware adds security headers and request validation

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

func cleanupExpiredTokens() {
	err := db.CleanupExpiredTokens()
	if err != nil {
		log.Printf("Failed to cleanup expired tokens: %v", err)
	}
}

// sendNotificationToUser 发送通知给用户（数据库 + WebSocket）
