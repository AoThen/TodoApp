package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"todoapp/internal/db"
	wsclient "todoapp/internal/websocket"
)

var (
	testTS    *httptest.Server
	testWSHub *wsclient.Hub
	adminTok  string
	userTok   string
	user2Tok  string
)

const (
	testPassword = "TestP@ssword123"
)

func TestMain(m *testing.M) {
	// init() 在测试二进制启动时已校验 JWT_SECRET / ENCRYPTION_KEY，必须由外部环境提供
	if os.Getenv("JWT_SECRET") == "" || os.Getenv("ENCRYPTION_KEY") == "" {
		fmt.Println("集成测试需要设置 JWT_SECRET 与 ENCRYPTION_KEY 环境变量")
		os.Exit(1)
	}

	// 临时文件数据库：:memory: 模式下 25 连接池的每个连接都是独立库，会造成数据漂移；
	// 文件库 + WAL 才能与生产连接池行为一致
	tmpFile, err := os.CreateTemp("", "todoapp-it-*.db")
	if err != nil {
		panic(err)
	}
	dsn := tmpFile.Name()
	tmpFile.Close()
	cleanupDBFiles := func() {
		os.Remove(dsn)
		os.Remove(dsn + "-wal")
		os.Remove(dsn + "-shm")
	}
	// 两次初始化绕过 seed：首次建表+seed 后删除文件重开
	for i := 0; i < 2; i++ {
		if err := db.InitDB(dsn); err != nil {
			panic(err)
		}
		db.DB.Close()
		cleanupDBFiles()
	}
	if err := db.InitDB(dsn); err != nil {
		panic(err)
	}

	if _, err := db.CreateUser("admin@test.local", testPassword, "admin"); err != nil {
		panic(err)
	}
	if _, err := db.CreateUser("user@test.local", testPassword, "user"); err != nil {
		panic(err)
	}
	if _, err := db.CreateUser("user2@test.local", testPassword, "user"); err != nil {
		panic(err)
	}

	wsHub := wsclient.NewHub()
	go wsHub.Run()
	testWSHub = wsHub

	testTS = httptest.NewServer(buildRouter(wsHub, false, "8080"))

	resetRateLimitState()
	adminTok = mustLogin("admin@test.local")
	userTok = mustLogin("user@test.local")
	user2Tok = mustLogin("user2@test.local")

	code := m.Run()
	testTS.Close()
	db.DB.Close()
	cleanupDBFiles()
	os.Exit(code)
}

func mustLogin(email string) string {
	body, _ := json.Marshal(map[string]string{"email": email, "password": testPassword})
	resp, err := http.Post(testTS.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var out struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.Data.AccessToken == "" {
		panic(fmt.Sprintf("预登录 %s 失败: %v", email, err))
	}
	return out.Data.AccessToken
}

// cleanTables 清空业务表并重置限流状态，保证测试互不影响（全局单例，测试须串行）
func cleanTables(t *testing.T) {
	t.Helper()
	for _, q := range []string{
		"DELETE FROM tasks", "DELETE FROM notifications", "DELETE FROM conflicts",
		"DELETE FROM delta_queue", "DELETE FROM tokens", "DELETE FROM devices",
		"DELETE FROM login_logs", "DELETE FROM admin_logs", "DELETE FROM deleted_tasks",
		"DELETE FROM system_config", "DELETE FROM sync_meta",
		// 重置测试账户的失败计数与锁定，避免限流/锁定测试污染后续用例
		"UPDATE users SET failed_attempts = 0, locked_until = NULL",
	} {
		if _, err := db.DB.Exec(q); err != nil {
			t.Fatalf("清表失败 %q: %v", q, err)
		}
	}
	resetRateLimitState()
	t.Cleanup(func() {
		for _, q := range []string{
			"DELETE FROM tasks", "DELETE FROM notifications", "DELETE FROM conflicts",
			"DELETE FROM delta_queue", "DELETE FROM tokens", "DELETE FROM devices",
			"DELETE FROM login_logs", "DELETE FROM admin_logs", "DELETE FROM deleted_tasks",
			"DELETE FROM system_config", "DELETE FROM sync_meta",
		} {
			_, _ = db.DB.Exec(q)
		}
		resetRateLimitState()
	})
}

func doJSON(t *testing.T, method, path, token string, body interface{}) (*http.Response, map[string]interface{}) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, testTS.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func dataMap(out map[string]interface{}) map[string]interface{} {
	if out == nil {
		return nil
	}
	d, _ := out["data"].(map[string]interface{})
	return d
}

func createTaskViaAPI(t *testing.T, token, title string) map[string]interface{} {
	t.Helper()
	resp, out := doJSON(t, http.MethodPost, "/api/v1/tasks", token, map[string]string{"title": title})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("创建任务失败: %d %v", resp.StatusCode, out)
	}
	return dataMap(out)
}

// ---------------- 基础与认证 ----------------

func TestHealthPublic(t *testing.T) {
	cleanTables(t)
	resp, out := doJSON(t, http.MethodGet, "/api/v1/health", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("健康检查应 200, got %d", resp.StatusCode)
	}
	if dataMap(out)["status"] != "healthy" {
		t.Errorf("健康检查状态异常: %v", out)
	}
}

func TestProtectedRequiresToken(t *testing.T) {
	cleanTables(t)
	resp, _ := doJSON(t, http.MethodGet, "/api/v1/tasks", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("无 token 应 401, got %d", resp.StatusCode)
	}
}

func TestProtectedRejectsBadToken(t *testing.T) {
	cleanTables(t)
	resp, _ := doJSON(t, http.MethodGet, "/api/v1/tasks", "not.a.token", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("非法 token 应 401, got %d", resp.StatusCode)
	}
}

func TestLoginSuccess(t *testing.T) {
	cleanTables(t)
	body, _ := json.Marshal(map[string]string{"email": "user@test.local", "password": testPassword})
	resp, err := http.Post(testTS.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Success bool `json:"success"`
		Data    struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || !out.Success || out.Data.AccessToken == "" {
		t.Errorf("登录应成功: %d %v", resp.StatusCode, out)
	}
	if out.Data.ExpiresIn != 900 {
		t.Errorf("expires_in 应为 900s, got %d", out.Data.ExpiresIn)
	}
	hasRefreshCookie := false
	for _, c := range resp.Cookies() {
		if c.Name == "refresh_token" && c.Value != "" {
			hasRefreshCookie = true
		}
	}
	if !hasRefreshCookie {
		t.Error("登录响应应携带 refresh_token cookie")
	}
}

func TestLoginFailures(t *testing.T) {
	cleanTables(t)
	cases := []struct {
		name  string
		email string
		pass  string
		want  int
	}{
		{"密码错误", "user@test.local", "WrongP@ss1", http.StatusUnauthorized},
		{"邮箱不存在", "ghost@test.local", testPassword, http.StatusUnauthorized},
		{"邮箱为空", "", testPassword, http.StatusBadRequest},
		{"密码为空", "user@test.local", "", http.StatusBadRequest},
		{"邮箱格式非法", "not-an-email", testPassword, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resetRateLimitState()
			body, _ := json.Marshal(map[string]string{"email": c.email, "password": c.pass})
			resp, err := http.Post(testTS.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != c.want {
				t.Errorf("%s: 状态码 = %d, want %d", c.name, resp.StatusCode, c.want)
			}
		})
	}
}

func TestLoginRateLimitPerIP(t *testing.T) {
	cleanTables(t)
	resetRateLimitState()
	var lastCode int
	for i := 0; i < maxLoginAttempts+2; i++ {
		body, _ := json.Marshal(map[string]string{"email": "user@test.local", "password": "WrongP@ss1"})
		resp, err := http.Post(testTS.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		lastCode = resp.StatusCode
		resp.Body.Close()
	}
	if lastCode != http.StatusTooManyRequests {
		t.Errorf("连续失败 %d 次后应触发 IP 限流 429, got %d", maxLoginAttempts, lastCode)
	}
}

func TestAccountLockout(t *testing.T) {
	cleanTables(t)
	resetRateLimitState()
	if _, err := db.CreateUser("lock@test.local", testPassword, "user"); err != nil {
		t.Fatalf("创建锁定测试用户失败: %v", err)
	}
	for i := 0; i < 5; i++ {
		_ = db.RecordFailedLogin("lock@test.local", 5, 30*time.Minute)
	}
	body, _ := json.Marshal(map[string]string{"email": "lock@test.local", "password": testPassword})
	resp, err := http.Post(testTS.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusLocked {
		t.Errorf("锁定账户登录应 423, got %d", resp.StatusCode)
	}
}

func TestRefreshTokenRotation(t *testing.T) {
	cleanTables(t)
	resetRateLimitState()
	// 登录拿 refresh cookie
	body, _ := json.Marshal(map[string]string{"email": "user@test.local", "password": testPassword})
	resp, err := http.Post(testTS.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var refreshCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "refresh_token" {
			refreshCookie = c
		}
	}
	resp.Body.Close()
	if refreshCookie == nil {
		t.Fatal("登录应返回 refresh_token cookie")
	}

	// 无 cookie → 401
	req, _ := http.NewRequest(http.MethodPost, testTS.URL+"/api/v1/auth/refresh", nil)
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("无 cookie 刷新应 401, got %d", resp2.StatusCode)
	}

	// 有效 cookie → 200 并轮换出新 cookie
	req3, _ := http.NewRequest(http.MethodPost, testTS.URL+"/api/v1/auth/refresh", nil)
	req3.Header.Set("Content-Type", "application/json")
	req3.AddCookie(refreshCookie)
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("有效 refresh 应 200, got %d", resp3.StatusCode)
	}
	var out struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp3.Body).Decode(&out)
	if out.Data.AccessToken == "" {
		t.Error("刷新应返回新 access_token")
	}
	rotated := false
	for _, c := range resp3.Cookies() {
		if c.Name == "refresh_token" && c.Value != "" && c.Value != refreshCookie.Value {
			rotated = true
		}
	}
	if !rotated {
		t.Error("刷新应轮换出新 refresh_token")
	}

	// 旧 refresh 已被吊销 → 再次使用应 401
	req4, _ := http.NewRequest(http.MethodPost, testTS.URL+"/api/v1/auth/refresh", nil)
	req4.Header.Set("Content-Type", "application/json")
	req4.AddCookie(refreshCookie)
	resp4, err := http.DefaultClient.Do(req4)
	if err != nil {
		t.Fatal(err)
	}
	resp4.Body.Close()
	if resp4.StatusCode != http.StatusUnauthorized {
		t.Errorf("已吊销的 refresh 应 401, got %d", resp4.StatusCode)
	}
}

func TestLogout(t *testing.T) {
	cleanTables(t)
	resp, out := doJSON(t, http.MethodPost, "/api/v1/auth/logout", userTok, nil)
	if resp.StatusCode != http.StatusOK || dataMap(out)["status"] != "已登出" {
		t.Errorf("登出应 200 已登出: %d %v", resp.StatusCode, out)
	}
}

func TestMe(t *testing.T) {
	cleanTables(t)
	resp, out := doJSON(t, http.MethodGet, "/api/v1/users/me", userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("me 应 200, got %d", resp.StatusCode)
	}
	if dataMap(out)["email"] != "user@test.local" {
		t.Errorf("me 返回邮箱不符: %v", out)
	}
}

// ---------------- 任务 CRUD 与所有权 ----------------

func TestTaskCRUD(t *testing.T) {
	cleanTables(t)
	// 创建
	created := createTaskViaAPI(t, userTok, "集成测试任务")
	idF, ok := created["id"].(float64)
	if !ok || idF < 1 {
		t.Fatalf("创建任务应返回正数 id: %v", created)
	}
	id := int64(idF)

	// 查询单个
	resp, out := doJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", id), userTok, nil)
	if resp.StatusCode != http.StatusOK || dataMap(out)["title"] != "集成测试任务" {
		t.Errorf("查询任务失败: %d %v", resp.StatusCode, out)
	}

	// 更新
	resp, out = doJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/tasks/%d", id), userTok,
		map[string]interface{}{"title": "更新后标题", "status": "done"})
	if resp.StatusCode != http.StatusOK || dataMap(out)["title"] != "更新后标题" || dataMap(out)["status"] != "done" {
		t.Errorf("更新任务失败: %d %v", resp.StatusCode, out)
	}

	// 列表分页
	for _, title := range []string{"任务B", "任务C"} {
		createTaskViaAPI(t, userTok, title)
	}
	resp, out = doJSON(t, http.MethodGet, "/api/v1/tasks?page=1&page_size=2", userTok, nil)
	data := dataMap(out)
	tasks, _ := data["tasks"].([]interface{})
	pagination, _ := data["pagination"].(map[string]interface{})
	if resp.StatusCode != http.StatusOK || len(tasks) != 2 {
		t.Errorf("分页应返回 2 条: %d %v", resp.StatusCode, data)
	}
	if pagination["total"].(float64) != 3 || pagination["pages"].(float64) != 2 {
		t.Errorf("分页元数据错误: %v", pagination)
	}

	// 删除（软删除：单查仍可见但 is_deleted=true，列表不再返回）
	resp, _ = doJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", id), userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("删除应 200, got %d", resp.StatusCode)
	}
	resp, out = doJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", id), userTok, nil)
	if resp.StatusCode != http.StatusOK || dataMap(out)["is_deleted"] != true {
		t.Errorf("软删除后单查应 200 且 is_deleted=true, got %d %v", resp.StatusCode, dataMap(out))
	}
	_, out = doJSON(t, http.MethodGet, "/api/v1/tasks", userTok, nil)
	if tasks, _ := dataMap(out)["tasks"].([]interface{}); len(tasks) != 2 {
		t.Errorf("软删除后列表应只剩 2 条, got %v", dataMap(out))
	}
}

func TestTaskValidation(t *testing.T) {
	cleanTables(t)
	// 空标题
	resp, _ := doJSON(t, http.MethodPost, "/api/v1/tasks", userTok, map[string]string{"title": ""})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("空标题应 400, got %d", resp.StatusCode)
	}
	// 超长标题（>200 字符）
	long := make([]byte, 201)
	for i := range long {
		long[i] = 'a'
	}
	resp, _ = doJSON(t, http.MethodPost, "/api/v1/tasks", userTok, map[string]string{"title": string(long)})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("超长标题应 400, got %d", resp.StatusCode)
	}
	// 无效 due_at
	resp, _ = doJSON(t, http.MethodPatch, "/api/v1/tasks/1", userTok, map[string]string{"due_at": "明天"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("无效 due_at 应 400, got %d", resp.StatusCode)
	}
	// PATCH 无有效字段
	resp, _ = doJSON(t, http.MethodPatch, "/api/v1/tasks/1", userTok, map[string]string{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("空更新应 400, got %d", resp.StatusCode)
	}
}

func TestTaskOwnershipIsolation(t *testing.T) {
	cleanTables(t)
	created := createTaskViaAPI(t, userTok, "A 的私有任务")
	id := int64(created["id"].(float64))

	// 用户 B 访问 A 的任务 → 404（不泄露存在性）
	resp, _ := doJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", id), user2Tok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("越权读取应 404, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/tasks/%d", id), user2Tok,
		map[string]string{"title": "篡改"})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("越权更新应 404, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", id), user2Tok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("越权删除应 404, got %d", resp.StatusCode)
	}
	// A 自己仍可读
	resp, _ = doJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", id), userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("所有者读取应 200, got %d", resp.StatusCode)
	}
}

func TestTaskNotFound(t *testing.T) {
	cleanTables(t)
	resp, _ := doJSON(t, http.MethodGet, "/api/v1/tasks/999999", userTok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("不存在任务应 404, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodGet, "/api/v1/tasks/abc", userTok, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("非法任务 ID 应 400, got %d", resp.StatusCode)
	}
}

func TestBatchDeleteTasks(t *testing.T) {
	cleanTables(t)
	t1 := createTaskViaAPI(t, userTok, "批量1")
	t2 := createTaskViaAPI(t, userTok, "批量2")
	t3 := createTaskViaAPI(t, userTok, "保留")
	ids := []int64{int64(t1["id"].(float64)), int64(t2["id"].(float64))}
	resp, out := doJSON(t, http.MethodDelete, "/api/v1/tasks/batch", userTok, map[string]interface{}{"task_ids": ids})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("批量删除应 200, got %d %v", resp.StatusCode, out)
	}
	// 已删的单查可见但 is_deleted=true；未删的正常
	resp, out = doJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%v", t1["id"]), userTok, nil)
	if resp.StatusCode != http.StatusOK || dataMap(out)["is_deleted"] != true {
		t.Errorf("批量软删除后单查应 200 且 is_deleted=true, got %d %v", resp.StatusCode, dataMap(out))
	}
	resp, _ = doJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%v", t3["id"]), userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("未删除任务应仍可见, got %d", resp.StatusCode)
	}
}

// ---------------- Delta 同步 ----------------

func TestSyncInsertDelta(t *testing.T) {
	cleanTables(t)
	syncBody := map[string]interface{}{
		"last_sync_at": "",
		"changes": []map[string]interface{}{
			{
				"local_id": "local-abc",
				"op":       "insert",
				"payload": map[string]interface{}{
					"title": "同步新建", "description": "来自 delta", "status": "todo", "priority": "high",
				},
				"client_version": 0,
			},
		},
	}
	resp, out := doJSON(t, http.MethodPost, "/api/v1/sync", userTok, syncBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sync 应 200, got %d %v", resp.StatusCode, out)
	}
	// sync 响应为顶层结构（server_changes/client_changes/last_sync_at/conflicts），非 {success,data} 包裹
	serverChanges, _ := out["server_changes"].([]interface{})
	clientChanges, _ := out["client_changes"].([]interface{})
	if len(serverChanges) != 1 || len(clientChanges) != 1 {
		t.Fatalf("insert delta 应产生 1 条 server_change 与 1 条 client_change: %v", out)
	}
	// 落库可见
	_, out = doJSON(t, http.MethodGet, "/api/v1/tasks", userTok, nil)
	tasks, _ := dataMap(out)["tasks"].([]interface{})
	if len(tasks) != 1 {
		t.Fatalf("同步后应能查到任务, got %v", tasks)
	}
	task := tasks[0].(map[string]interface{})
	if task["title"] != "同步新建" || task["priority"] != "high" {
		t.Errorf("同步任务字段不符: %v", task)
	}
}

func TestSyncDuplicateInsertConflict(t *testing.T) {
	cleanTables(t)
	change := map[string]interface{}{
		"local_id": "local-dup", "op": "insert",
		"payload":        map[string]interface{}{"title": "重复", "status": "todo", "priority": "medium"},
		"client_version": 0,
	}
	doJSON(t, http.MethodPost, "/api/v1/sync", userTok,
		map[string]interface{}{"last_sync_at": "", "changes": []map[string]interface{}{change}})
	resp, out := doJSON(t, http.MethodPost, "/api/v1/sync", userTok,
		map[string]interface{}{"last_sync_at": "", "changes": []map[string]interface{}{change}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("重复插入 sync 应 200, got %d", resp.StatusCode)
	}
	conflicts, _ := out["conflicts"].([]interface{})
	if len(conflicts) != 1 {
		t.Errorf("重复 local_id 应记录 duplicate_insert 冲突: %v", out)
	}
}

func TestSyncUpdateConflictIntelligentMerge(t *testing.T) {
	cleanTables(t)
	// 先通过 sync 建任务（server_version=1）
	doJSON(t, http.MethodPost, "/api/v1/sync", userTok,
		map[string]interface{}{"last_sync_at": "", "changes": []map[string]interface{}{
			{"local_id": "local-merge", "op": "insert",
				"payload":        map[string]interface{}{"title": "服务器标题", "description": "服务器描述", "status": "in_progress", "priority": "medium"},
				"client_version": 0},
		}})
	_, out := doJSON(t, http.MethodGet, "/api/v1/tasks", userTok, nil)
	tasks, _ := dataMap(out)["tasks"].([]interface{})
	if len(tasks) != 1 {
		t.Fatalf("sync insert 后应能查到 1 个任务, got %v", dataMap(out))
	}
	task := tasks[0].(map[string]interface{})
	serverID := task["id"].(float64)
	serverVersion := task["server_version"].(float64)

	// 用过期的 client_version 推送更新 → 冲突合并
	resp, out := doJSON(t, http.MethodPost, "/api/v1/sync", userTok,
		map[string]interface{}{"last_sync_at": "", "changes": []map[string]interface{}{
			{"local_id": "local-merge", "op": "update",
				"payload":        map[string]interface{}{"id": serverID, "title": "客户端标题", "description": "客户端描述", "status": "done"},
				"client_version": int(serverVersion - 1)},
		}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("冲突 sync 应 200, got %d %v", resp.StatusCode, out)
	}
	conflicts, _ := out["conflicts"].([]interface{})
	if len(conflicts) != 1 {
		t.Errorf("版本不匹配应产生冲突记录: %v", out)
	}

	// 合并结果：标题服务器优先、状态取优先级高者（done）
	_, out = doJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%v", serverID), userTok, nil)
	merged := dataMap(out)
	if merged["title"] != "服务器标题" {
		t.Errorf("合并后标题应服务器优先: %v", merged)
	}
	if merged["status"] != "done" {
		t.Errorf("合并后状态应为 done: %v", merged)
	}
}

func TestSyncUpdateNoConflict(t *testing.T) {
	cleanTables(t)
	doJSON(t, http.MethodPost, "/api/v1/sync", userTok,
		map[string]interface{}{"last_sync_at": "", "changes": []map[string]interface{}{
			{"local_id": "local-ok", "op": "insert",
				"payload":        map[string]interface{}{"title": "原标题", "status": "todo", "priority": "low"},
				"client_version": 0},
		}})
	_, out := doJSON(t, http.MethodGet, "/api/v1/tasks", userTok, nil)
	tasks, _ := dataMap(out)["tasks"].([]interface{})
	if len(tasks) != 1 {
		t.Fatalf("sync insert 后应能查到 1 个任务, got %v", dataMap(out))
	}
	task := tasks[0].(map[string]interface{})
	serverID := task["id"].(float64)
	serverVersion := int(task["server_version"].(float64))

	// 带正确 client_version 的更新 → 无冲突
	resp, out := doJSON(t, http.MethodPost, "/api/v1/sync", userTok,
		map[string]interface{}{"last_sync_at": "", "changes": []map[string]interface{}{
			{"local_id": "local-ok", "op": "update",
				"payload":        map[string]interface{}{"id": serverID, "title": "新标题", "status": "done"},
				"client_version": serverVersion},
		}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("正常更新 sync 应 200, got %d", resp.StatusCode)
	}
	if conflicts, _ := out["conflicts"].([]interface{}); len(conflicts) != 0 {
		t.Errorf("版本匹配不应有冲突: %v", out)
	}
	_, out = doJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%v", serverID), userTok, nil)
	if dataMap(out)["title"] != "新标题" || dataMap(out)["status"] != "done" {
		t.Errorf("正常更新未生效: %v", out)
	}
}

func TestSyncInvalidBody(t *testing.T) {
	cleanTables(t)
	resp, _ := doJSON(t, http.MethodPost, "/api/v1/sync", userTok, "not-an-object")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("非法 sync 请求体应 400, got %d", resp.StatusCode)
	}
}

// ---------------- 同步安全（越权防护）与事务一致性 ----------------

func TestSyncIDORProtection(t *testing.T) {
	cleanTables(t)
	// 用户 A（userTok）通过 sync 创建任务
	insertChange := map[string]interface{}{
		"local_id": "idor-insert", "op": "insert",
		"payload":        map[string]interface{}{"title": "A的私有任务", "status": "todo", "priority": "high"},
		"client_version": 0,
	}
	resp, out := doJSON(t, http.MethodPost, "/api/v1/sync", userTok,
		map[string]interface{}{"last_sync_at": "", "changes": []map[string]interface{}{insertChange}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("用户A sync 应 200, got %d %v", resp.StatusCode, out)
	}
	serverChanges, _ := out["server_changes"].([]interface{})
	if len(serverChanges) != 1 {
		t.Fatalf("应返回 1 条 server_change: %v", out)
	}
	taskID := int64(serverChanges[0].(map[string]interface{})["id"].(float64))

	// 用户 B（adminTok，不同 user_id）尝试通过 sync 更新/删除 A 的任务 → 应失败
	updateByB := map[string]interface{}{
		"local_id": "idor-update", "op": "update",
		"payload":        map[string]interface{}{"id": float64(taskID), "title": "被篡改", "status": "done", "priority": "low"},
		"client_version": 1,
	}
	resp, _ = doJSON(t, http.MethodPost, "/api/v1/sync", adminTok,
		map[string]interface{}{"last_sync_at": "", "changes": []map[string]interface{}{updateByB}})
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("B 更新 A 的任务应 500（归属校验拦截）, got %d", resp.StatusCode)
	}

	deleteByB := map[string]interface{}{
		"local_id": "idor-delete", "op": "delete",
		"payload":        map[string]interface{}{"id": float64(taskID)},
		"client_version": 1,
	}
	resp, _ = doJSON(t, http.MethodPost, "/api/v1/sync", adminTok,
		map[string]interface{}{"last_sync_at": "", "changes": []map[string]interface{}{deleteByB}})
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("B 删除 A 的任务应 500（归属校验拦截）, got %d", resp.StatusCode)
	}

	// A 的任务应保持原样
	_, out = doJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", taskID), userTok, nil)
	task := dataMap(out)
	if task["title"] != "A的私有任务" || task["status"] != "todo" {
		t.Errorf("A 的任务被越权修改: %v", task)
	}
}

func TestSyncRollbackOnFailure(t *testing.T) {
	cleanTables(t)
	// 先创建一个任务（server_version=1）
	insertChange := map[string]interface{}{
		"local_id": "rb-insert", "op": "insert",
		"payload":        map[string]interface{}{"title": "回滚验证", "status": "todo", "priority": "medium"},
		"client_version": 0,
	}
	_, out := doJSON(t, http.MethodPost, "/api/v1/sync", userTok,
		map[string]interface{}{"last_sync_at": "", "changes": []map[string]interface{}{insertChange}})
	serverChanges, _ := out["server_changes"].([]interface{})
	if len(serverChanges) != 1 {
		t.Fatalf("应返回 1 条 server_change: %v", out)
	}
	taskID := int64(serverChanges[0].(map[string]interface{})["id"].(float64))

	// 同一批次：一个合法更新 + 一个更新不存在的任务 → 任一失败应整体回滚
	batch := map[string]interface{}{
		"last_sync_at": "",
		"changes": []map[string]interface{}{
			{
				"local_id": "rb-good", "op": "update",
				"payload":        map[string]interface{}{"id": float64(taskID), "title": "已被修改", "status": "done", "priority": "high"},
				"client_version": 1,
			},
			{
				"local_id": "rb-bad", "op": "update",
				"payload":        map[string]interface{}{"id": float64(999999), "title": "不存在", "status": "todo", "priority": "low"},
				"client_version": 1,
			},
		},
	}
	resp, _ := doJSON(t, http.MethodPost, "/api/v1/sync", userTok, batch)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("批次含失败项应整体 500, got %d", resp.StatusCode)
	}

	// 合法更新也必须被回滚：任务保持原状
	_, out = doJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", taskID), userTok, nil)
	task := dataMap(out)
	if task["title"] != "回滚验证" || task["status"] != "todo" {
		t.Errorf("失败批次未回滚，任务被部分修改: %v", task)
	}
}

func TestAPIUnknownPathReturns404(t *testing.T) {
	cleanTables(t)
	resp, _ := doJSON(t, http.MethodGet, "/api/v1/definitely-not-exist", userTok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("未知 API 路径应 404 而非落入 SPA fallback, got %d", resp.StatusCode)
	}
}
