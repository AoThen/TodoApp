package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

import (
	"todoapp/internal/config"
	"todoapp/internal/crypto"
	"todoapp/internal/db"
	"todoapp/internal/response"
	"todoapp/internal/websocket"
)

const (
	maxRequestBodySize = 10 * 1024 * 1024 // 10MB
	cookieSameSite     = http.SameSiteStrictMode
)

// 安全与令牌相关配置：支持通过环境变量覆盖（详见 README 环境变量说明）
var (
	accessTokenDuration  = config.EnvDuration("ACCESS_TOKEN_DURATION", 15*time.Minute)
	refreshTokenDuration = config.EnvDuration("REFRESH_TOKEN_DURATION", 7*24*time.Hour)
	maxLoginAttempts     = config.EnvInt("MAX_LOGIN_ATTEMPTS", 5)
	loginAttemptWindow   = config.EnvDuration("LOGIN_ATTEMPT_WINDOW", 15*time.Minute)
	lockoutDuration      = config.EnvDuration("LOCKOUT_DURATION", 30*time.Minute)
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

	// WebSocket Origin 白名单（生产环境必须配置，防止跨站 WebSocket 劫持）
	if origins := os.Getenv("WS_ALLOWED_ORIGINS"); origins != "" {
		var originList []string
		for _, o := range strings.Split(origins, ",") {
			if o = strings.TrimSpace(o); o != "" {
				originList = append(originList, o)
			}
		}
		websocket.SetAllowedOrigins(originList)
		log.Printf("WebSocket Origin 白名单已启用: %v", originList)
	}

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

	// 生产环境可选 HTTP→HTTPS 重定向（依赖反向代理设置 X-Forwarded-Proto）
	if environment == "production" && config.EnvBool("AUTO_REDIRECT_HTTPS", false) {
		handler = httpsRedirectMiddleware(handler)
		log.Println("AUTO_REDIRECT_HTTPS 已启用：HTTP 请求将重定向到 HTTPS")
	}

	// Configure server with timeouts
	serverAddr := ":" + serverPort
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 优雅停机：监听 SIGINT/SIGTERM，收到信号后等待进行中的请求完成（最长 10s）再退出
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("Starting server on %s...", serverAddr)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	case <-ctx.Done():
		log.Println("收到退出信号，开始优雅停机...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("优雅停机未在期限内完成: %v", err)
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server failed: %v", err)
		}
		if err := db.CloseDB(); err != nil {
			log.Printf("关闭数据库失败: %v", err)
		}
		log.Println("服务已退出")
	}
}

// httpsRedirectMiddleware 将非 HTTPS 的 GET/HEAD 请求（依据 X-Forwarded-Proto）301 重定向到 HTTPS。
// 非 GET/HEAD 请求不重定向（301 会丢弃请求体，破坏 API 写操作）。
func httpsRedirectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-Proto") == "http" {
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
				return
			}
			http.Error(w, "请使用 HTTPS 访问", http.StatusUpgradeRequired)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func startCleanupTasks() {
	// 每小时清理一次过期通知和令牌
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			cleanupExpiredNotifications()
			cleanupExpiredTokens()
		}
	}()

	// 每天清理一次旧日志、删除记录与超过保留期的通知
	dailyTicker := time.NewTicker(24 * time.Hour)
	go func() {
		for range dailyTicker.C {
			if err := db.CleanupOldLoginLogs(); err != nil {
				log.Printf("Failed to cleanup old login logs: %v", err)
			}
			count, _ := db.CleanupOldDeletedTasks()
			if count > 0 {
				log.Printf("Cleaned up %d old deleted_task records", count)
			}
			retentionDays := config.EnvInt("NOTIFICATION_RETENTION_DAYS", 30)
			if n, err := db.CleanupNotificationsOlderThan(retentionDays); err != nil {
				log.Printf("Failed to cleanup notifications older than %d days: %v", retentionDays, err)
			} else if n > 0 {
				log.Printf("Cleaned up %d notifications older than %d days", n, retentionDays)
			}
			// 冲突记录与删除记录同保留期（30 天），防止 conflicts 表无限增长
			if n, err := db.CleanupOldConflicts(30); err != nil {
				log.Printf("Failed to cleanup old conflicts: %v", err)
			} else if n > 0 {
				log.Printf("Cleaned up %d old conflict records", n)
			}
		}
	}()
}

func cleanupExpiredTokens() {
	err := db.CleanupExpiredTokens()
	if err != nil {
		log.Printf("Failed to cleanup expired tokens: %v", err)
	}
}

// sendNotificationToUser 发送通知给用户（数据库 + WebSocket）
