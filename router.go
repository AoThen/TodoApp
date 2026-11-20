package main

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"todoapp/internal/crypto"
	"todoapp/internal/websocket"
)

// buildRouter 装配全部路由与中间件（main 与集成测试共享）。
// 从 main() 原样抽出，行为不变。
func buildRouter(wsHub *websocket.Hub, enforceWSEncryption bool, serverPort string) http.Handler {
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

	// Protected routes
	protected := router.PathPrefix("/api/v1").Subrouter()
	protected.Use(authMiddleware)

	em := crypto.GetEncryptionMiddleware()
	protected.Use(em.DecryptRequest)
	protected.Use(em.EncryptResponse)

	protected.HandleFunc("/users/me", handleMe).Methods("GET")
	// logout 需要登录态（撤销当前用户的 refresh token），必须挂在受保护段
	protected.HandleFunc("/auth/logout", handleLogout).Methods("POST")
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

	// 前端静态资源与 SPA fallback：注册在所有 API 路由之后兜底。
	// 静态目录可通过 STATIC_DIR 覆盖（Docker 镜像内为 web/build）；目录不存在（如无前端的开发场景）时不启用。
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "web/build"
	}
	if _, err := os.Stat(filepath.Join(staticDir, "index.html")); err == nil {
		router.PathPrefix("/").Handler(serveStaticHandler(staticDir))
	}

	// CORS handling for development
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"http://localhost:3000", "http://localhost:" + serverPort}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "X-User-ID", "X-Request-ID"}),
		handlers.AllowCredentials(),
	)(router)

	return corsHandler
}

// serveStaticHandler 提供前端构建产物（web/build）的静态文件服务与 SPA fallback：
// 请求命中真实文件时直接返回；否则回退到 index.html，交由前端路由处理。
func serveStaticHandler(dir string) http.Handler {
	fs := http.Dir(dir)
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// http.Dir.Open 内置路径穿越防护（拒绝包含 .. 的路径）
		if f, err := fs.Open(r.URL.Path); err == nil {
			stat, statErr := f.Stat()
			_ = f.Close()
			if statErr == nil && !stat.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}
