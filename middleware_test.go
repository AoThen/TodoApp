package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// resetRateLimitState 清空包级限流状态，保证测试互不影响
func resetRateLimitState() {
	attemptsMutex.Lock()
	loginAttempts = make(map[string][]time.Time)
	attemptsMutex.Unlock()

	ipMutex.Lock()
	ipRequests = make(map[string][]time.Time)
	ipMutex.Unlock()
}

func TestCheckRateLimitWithinWindow(t *testing.T) {
	resetRateLimitState()
	ip := "1.2.3.4"
	for i := 0; i < maxLoginAttempts; i++ {
		if !checkRateLimit(ip) {
			t.Fatalf("第 %d 次尝试不应被限流", i+1)
		}
	}
	if checkRateLimit(ip) {
		t.Error("超过 maxLoginAttempts 后应当被限流")
	}
}

func TestCheckRateLimitWindowExpiry(t *testing.T) {
	resetRateLimitState()
	ip := "5.6.7.8"
	// 注入窗口外的旧尝试记录
	attemptsMutex.Lock()
	loginAttempts[ip] = []time.Time{
		time.Now().Add(-2 * loginAttemptWindow),
		time.Now().Add(-2 * loginAttemptWindow),
	}
	attemptsMutex.Unlock()

	if !checkRateLimit(ip) {
		t.Error("窗口外的旧记录不应导致限流")
	}
}

func TestCheckRateLimitIsolationByIP(t *testing.T) {
	resetRateLimitState()
	for i := 0; i < maxLoginAttempts; i++ {
		checkRateLimit("9.9.9.9")
	}
	if !checkRateLimit("8.8.8.8") {
		t.Error("不同 IP 不应被连带限流")
	}
}

func TestGetClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	if got := getClientIP(r); got != "10.0.0.1" {
		t.Errorf("RemoteAddr 回退解析错误: %q", got)
	}

	// 默认不信任 X-Forwarded-For（TRUSTED_PROXY=false）：防止客户端伪造绕过限流
	r.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.2")
	if got := getClientIP(r); got != "10.0.0.1" {
		t.Errorf("默认应忽略可伪造的 X-Forwarded-For: %q", got)
	}

	// 部署于可信反向代理之后（TRUSTED_PROXY=true）时取 X-Forwarded-For 第一个 IP
	origTrusted := trustedProxy
	trustedProxy = true
	defer func() { trustedProxy = origTrusted }()
	if got := getClientIP(r); got != "203.0.113.5" {
		t.Errorf("X-Forwarded-For 应取第一个 IP: %q", got)
	}

	r.Header.Set("X-Forwarded-For", " 203.0.113.7 ")
	if got := getClientIP(r); got != "203.0.113.7" {
		t.Errorf("X-Forwarded-For 应去除空白: %q", got)
	}
}

func TestGlobalRateLimitMiddleware(t *testing.T) {
	resetRateLimitState()
	// 缩小上限便于测试
	origLimit := globalRateLimit
	globalRateLimit = 3
	defer func() { globalRateLimit = origLimit }()

	handler := globalRateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip := "7.7.7.7"
	var last *httptest.ResponseRecorder
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		req.RemoteAddr = ip + ":1000" + string(rune('0'+i))
		last = httptest.NewRecorder()
		handler.ServeHTTP(last, req)
	}
	if last.Code != http.StatusTooManyRequests {
		t.Errorf("超限后应返回 429, got %d", last.Code)
	}
	if last.Header().Get("Retry-After") != "60" {
		t.Error("429 响应应携带 Retry-After: 60")
	}
	var body map[string]string
	if err := json.Unmarshal(last.Body.Bytes(), &body); err != nil || body["error"] == "" {
		t.Errorf("429 响应体格式错误: %q", last.Body.String())
	}
}

func TestHeadersMiddleware(t *testing.T) {
	handler := headersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	expected := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"X-XSS-Protection":          "1; mode=block",
		"Content-Security-Policy":   "default-src 'self'",
		"Cache-Control":             "no-store, no-cache, must-revalidate",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	}
	for k, v := range expected {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("安全头 %s = %q, want %q", k, got, v)
		}
	}

	// 非 /api/v1/ 路径不应带 Cache-Control: no-store
	req2 := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if cc := rec2.Header().Get("Cache-Control"); cc != "" {
		t.Errorf("非 API 路径不应设置 Cache-Control, got %q", cc)
	}
}

func TestSecurityMiddlewareContentType(t *testing.T) {
	handler := securityMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// POST 缺少 JSON Content-Type → 415
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader("x"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("非 JSON Content-Type 应返回 415, got %d", rec.Code)
	}

	// POST JSON → 放行
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader("{}"))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("JSON 请求应放行, got %d", rec2.Code)
	}

	// GET 不检查 Content-Type
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Errorf("GET 不应检查 Content-Type, got %d", rec3.Code)
	}
}

func TestAuthMiddlewareRejects(t *testing.T) {
	handler := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 无 Authorization 头
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("缺少 Authorization 应返回 401, got %d", rec.Code)
	}

	// 格式错误（非 Bearer）
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Authorization", "Basic abc")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("非 Bearer 格式应返回 401, got %d", rec2.Code)
	}

	// 无效 token（不触数据库即失败）
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.Header.Set("Authorization", "Bearer invalid.token.here")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusUnauthorized {
		t.Errorf("无效 token 应返回 401, got %d", rec3.Code)
	}
}

func TestAdminMiddleware(t *testing.T) {
	tests := []struct {
		name string
		role string
		want int
	}{
		{"admin 放行", "admin", http.StatusOK},
		{"普通用户 403", "user", http.StatusForbidden},
		{"无角色 403", "", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := adminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.role != "" {
				req = req.WithContext(context.WithValue(req.Context(), ctxKeyRole, tt.role))
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("role=%q 状态码 = %d, want %d", tt.role, rec.Code, tt.want)
			}
		})
	}
}

func TestWebSocketEncryptionMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ws?encryption=true", nil)
	rec := httptest.NewRecorder()
	webSocketEncryptionMiddleware(true)(inner).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("强制加密下带 encryption=true 应放行, got %d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec2 := httptest.NewRecorder()
	webSocketEncryptionMiddleware(true)(inner).ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusForbidden {
		t.Errorf("强制加密下未加密应 403, got %d", rec2.Code)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec3 := httptest.NewRecorder()
	webSocketEncryptionMiddleware(false)(inner).ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Errorf("非强制下未加密应放行, got %d", rec3.Code)
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()
	if got := getUserIDFromContext(ctx); got != "" {
		t.Errorf("空上下文 userID 应为空串, got %q", got)
	}
	ctx = context.WithValue(ctx, ctxKeyUserID, "42")
	ctx = context.WithValue(ctx, ctxKeyEmail, "a@b.c")
	ctx = context.WithValue(ctx, ctxKeyRole, "admin")
	if getUserIDFromContext(ctx) != "42" || getEmailFromContext(ctx) != "a@b.c" || getRoleFromContext(ctx) != "admin" {
		t.Error("上下文取值与写入不一致")
	}
}
