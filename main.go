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

	// 读取服务器端口配置（支持 SERVER_PORT 环境变量，默认 8080）
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}
	if _, err := strconv.Atoi(serverPort); err != nil {
		log.Printf("SERVER_PORT 环境变量无效（%q），使用默认端口 8080", serverPort)
		serverPort = "8080"
	}

	// 路由与中间件装配（与集成测试共享）
	handler := buildRouter(wsHub, enforceWSEncryption, serverPort)

	// Configure server with timeouts
	serverAddr := ":" + serverPort
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      handler,
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
