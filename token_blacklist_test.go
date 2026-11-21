package main

import (
	"net/http"
	"testing"
)

// TestLogoutBlacklistsAccessToken 登出后，当前访问令牌应立即失效（而非等待自然过期）
func TestLogoutBlacklistsAccessToken(t *testing.T) {
	cleanTables(t)

	// 独立登录获取新令牌
	token := mustLogin("user@test.local")

	// 登出前令牌可用
	resp, _ := doJSON(t, http.MethodGet, "/api/v1/users/me", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("登出前令牌应有效, got %d", resp.StatusCode)
	}

	// 登出
	resp, _ = doJSON(t, http.MethodPost, "/api/v1/auth/logout", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("登出应成功, got %d", resp.StatusCode)
	}

	// 登出后同一令牌立即失效
	resp, _ = doJSON(t, http.MethodGet, "/api/v1/users/me", token, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("登出后令牌应立即失效(401), got %d", resp.StatusCode)
	}

	// 其他用户的令牌不受影响（独立登录，避免依赖全局预登录令牌）
	otherToken := mustLogin("admin@test.local")
	resp, _ = doJSON(t, http.MethodGet, "/api/v1/users/me", otherToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("其他令牌不应受影响, got %d", resp.StatusCode)
	}
}
