package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"todoapp/internal/db"
)

// findUserIDByEmail 从 admin 用户列表中查目标用户 id。
func findUserIDByEmail(t *testing.T, email string) int64 {
	t.Helper()
	_, out := doJSON(t, http.MethodGet, "/api/v1/admin/users", adminTok, nil)
	users, _ := dataMap(out)["users"].([]interface{})
	for _, u := range users {
		m, _ := u.(map[string]interface{})
		if m["email"] == email {
			return int64(m["id"].(float64))
		}
	}
	t.Fatalf("未找到用户 %s: %v", email, out)
	return 0
}

// loginWithRefresh 登录并返回 access token 与 refresh cookie。
func loginWithRefresh(t *testing.T, email string) (string, *http.Cookie) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": testPassword})
	resp, err := http.Post(testTS.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "refresh_token" {
			cookie = c
		}
	}
	if out.Data.AccessToken == "" || cookie == nil {
		t.Fatalf("登录 %s 应返回 access_token 与 refresh cookie: %d", email, resp.StatusCode)
	}
	return out.Data.AccessToken, cookie
}

// refreshWithCookie 用给定 cookie 调用刷新接口（不依赖 *testing.T，goroutine 安全）。
func refreshWithCookie(cookie *http.Cookie) (int, error) {
	req, _ := http.NewRequest(http.MethodPost, testTS.URL+"/api/v1/auth/refresh", nil)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func TestAdminLockUnlockLoginFlow(t *testing.T) {
	cleanTables(t)
	// 使用独立用户，避免锁定/解锁影响共享账号的后续测试
	if _, err := db.CreateUser("lockflow@test.local", testPassword, "user"); err != nil {
		t.Fatalf("创建锁定流程用户失败: %v", err)
	}
	uid := findUserIDByEmail(t, "lockflow@test.local")

	resp, _ := doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/lock", uid), adminTok, map[string]int{"duration_minutes": 30})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("锁定用户应 200, got %d", resp.StatusCode)
	}

	// 锁定期间密码登录应 423
	body, _ := json.Marshal(map[string]string{"email": "lockflow@test.local", "password": testPassword})
	loginResp, err := http.Post(testTS.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusLocked {
		t.Errorf("锁定期间登录应 423, got %d", loginResp.StatusCode)
	}

	resp, _ = doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/unlock", uid), adminTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("解锁用户应 200, got %d", resp.StatusCode)
	}

	// 解锁后登录恢复
	loginResp, err = http.Post(testTS.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Errorf("解锁后登录应 200, got %d", loginResp.StatusCode)
	}
}

func TestAdminResetPasswordRevokesRefresh(t *testing.T) {
	cleanTables(t)
	// 使用独立用户，避免重置密码影响共享账号的后续测试
	if _, err := db.CreateUser("resetpw@test.local", testPassword, "user"); err != nil {
		t.Fatalf("创建重置密码用户失败: %v", err)
	}
	uid := findUserIDByEmail(t, "resetpw@test.local")
	_, cookie := loginWithRefresh(t, "resetpw@test.local")

	resp, _ := doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/password", uid), adminTok, map[string]string{"new_password": "NewReset@2026x"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("重置密码应 200, got %d", resp.StatusCode)
	}

	// 重置密码后旧 refresh token 应被撤销
	if code, err := refreshWithCookie(cookie); err != nil || code != http.StatusUnauthorized {
		t.Errorf("重置密码后旧 refresh 应失效(401), got %d err=%v", code, err)
	}

	// 新密码可登录
	body, _ := json.Marshal(map[string]string{"email": "resetpw@test.local", "password": "NewReset@2026x"})
	loginResp, err := http.Post(testTS.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Errorf("新密码登录应 200, got %d", loginResp.StatusCode)
	}
}

func TestAdminDeleteUserCascades(t *testing.T) {
	cleanTables(t)
	if _, err := db.CreateUser("cascade@test.local", testPassword, "user"); err != nil {
		t.Fatalf("创建级联测试用户失败: %v", err)
	}
	uid := findUserIDByEmail(t, "cascade@test.local")
	token, cookie := loginWithRefresh(t, "cascade@test.local")
	createTaskViaAPI(t, token, "级联任务")

	resp, _ := doJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", uid), adminTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("删除用户应 200, got %d", resp.StatusCode)
	}

	// 旧 refresh token 应被级联清理
	if code, err := refreshWithCookie(cookie); err != nil || code != http.StatusUnauthorized {
		t.Errorf("删除用户后旧 refresh 应失效(401), got %d err=%v", code, err)
	}
	// 该用户的任务数据应被级联清理
	var taskCount int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM tasks WHERE user_id = ?", uid).Scan(&taskCount); err != nil {
		t.Fatalf("查询级联任务失败: %v", err)
	}
	if taskCount != 0 {
		t.Errorf("删除用户后其任务应被级联清理, 仍剩 %d 条", taskCount)
	}
	var deviceCount int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM devices WHERE user_id = ?", uid).Scan(&deviceCount); err != nil {
		t.Fatalf("查询级联设备失败: %v", err)
	}
	if deviceCount != 0 {
		t.Errorf("删除用户后其设备应被级联清理, 仍剩 %d 台", deviceCount)
	}
	// 用户已删除：原密码登录失败
	body, _ := json.Marshal(map[string]string{"email": "cascade@test.local", "password": testPassword})
	loginResp, err := http.Post(testTS.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("已删除用户登录应 401, got %d", loginResp.StatusCode)
	}
	// 该用户任务应被清理（admin 全局任务视角不存在；用 tasks 表无用户归属校验代替：
	// 直接验证 admin 用户列表不再包含该用户）
	_, out := doJSON(t, http.MethodGet, "/api/v1/admin/users", adminTok, nil)
	for _, u := range dataMap(out)["users"].([]interface{}) {
		if m, _ := u.(map[string]interface{}); m["email"] == "cascade@test.local" {
			t.Errorf("删除后用户列表不应包含该用户: %v", out)
		}
	}
}

func TestAdminDeleteLastAdminRejected(t *testing.T) {
	cleanTables(t)
	adminUID := findUserIDByEmail(t, "admin@test.local")
	resp, _ := doJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", adminUID), adminTok, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("删除最后一个管理员应 400, got %d", resp.StatusCode)
	}
}

func TestAdminDeleteNonexistentUser(t *testing.T) {
	cleanTables(t)
	resp, _ := doJSON(t, http.MethodDelete, "/api/v1/admin/users/999999", adminTok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("删除不存在用户应 404, got %d", resp.StatusCode)
	}
}
