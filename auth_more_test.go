package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
)

// tryLoginFromIP 以指定 X-Forwarded-For 登录（空 IP 使用真实地址）。
func tryLoginFromIP(email, ip string) (string, error) {
	body, _ := json.Marshal(map[string]string{"email": email, "password": testPassword})
	req, err := http.NewRequest(http.MethodPost, testTS.URL+"/api/v1/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if ip != "" {
		req.Header.Set("X-Forwarded-For", ip)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("登录 %s 失败: %d", email, resp.StatusCode)
	}
	var out struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Data.AccessToken == "" {
		return "", fmt.Errorf("登录 %s 未返回 access_token", email)
	}
	return out.Data.AccessToken, nil
}

func TestRefreshReplayAfterLogout(t *testing.T) {
	cleanTables(t)
	token, cookie := loginWithRefresh(t, "user@test.local")

	// 登出（带 refresh cookie：服务端据此撤销刷新令牌；access token 进黑名单）
	logoutReq, _ := http.NewRequest(http.MethodPost, testTS.URL+"/api/v1/auth/logout", nil)
	logoutReq.Header.Set("Content-Type", "application/json")
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutReq.AddCookie(cookie)
	logoutResp, err := http.DefaultClient.Do(logoutReq)
	if err != nil {
		t.Fatal(err)
	}
	logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("登出应 200, got %d", logoutResp.StatusCode)
	}

	// 旧 refresh cookie 不能再刷新
	if code, err := refreshWithCookie(cookie); err != nil || code != http.StatusUnauthorized {
		t.Errorf("登出后旧 refresh 应失效(401), got %d err=%v", code, err)
	}
	// 旧 access token 也不能再用（黑名单）
	resp, _ := doJSON(t, http.MethodGet, "/api/v1/users/me", token, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("登出后旧 access token 应失效(401), got %d", resp.StatusCode)
	}
}

func TestConcurrentLoginRace(t *testing.T) {
	cleanTables(t)
	// 信任 X-Forwarded-For，使并发登录各用独立限流桶，避免登录限流误伤并发断言
	origTrusted := trustedProxy
	trustedProxy = true
	t.Cleanup(func() { trustedProxy = origTrusted })

	const n = 10
	var wg sync.WaitGroup
	var okCount int64
	statuses := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			token, err := tryLoginFromIP("user@test.local", fmt.Sprintf("10.0.%d.%d", idx/250+1, idx%250+1))
			if err == nil && token != "" {
				atomic.AddInt64(&okCount, 1)
			} else if err != nil {
				statuses[idx] = loginStatusForDebug("user@test.local")
			}
		}(i)
	}
	wg.Wait()
	if okCount != n {
		t.Errorf("并发登录应全部成功, 成功 %d/%d, 失败状态码 %v", okCount, n, statuses)
	}
}

// loginStatusForDebug 返回一次登录尝试的状态码（仅诊断用）。
func loginStatusForDebug(email string) int {
	body, _ := json.Marshal(map[string]string{"email": email, "password": testPassword})
	resp, err := http.Post(testTS.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return -1
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestConcurrentRefreshRotation(t *testing.T) {
	cleanTables(t)
	_, cookie := loginWithRefresh(t, "user@test.local")

	// 同一 refresh cookie 并发刷新：轮换应保证恰好一次成功
	const n = 8
	var wg sync.WaitGroup
	var okCount int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if code, err := refreshWithCookie(cookie); err == nil && code == http.StatusOK {
				atomic.AddInt64(&okCount, 1)
			}
		}()
	}
	wg.Wait()
	if okCount != 1 {
		t.Errorf("并发刷新同一 refresh 应恰好成功 1 次, 实际 %d", okCount)
	}
}
