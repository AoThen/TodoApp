package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"testing"

	"todoapp/internal/db"
)

// ---------------- 通知 ----------------

func createNotificationViaAPI(t *testing.T, token, title string) map[string]interface{} {
	t.Helper()
	resp, out := doJSON(t, http.MethodPost, "/api/v1/notifications", token, map[string]string{
		"type": "system", "title": title, "content": "通知内容 " + title, "priority": "normal",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("创建通知失败: %d %v", resp.StatusCode, out)
	}
	return dataMap(out)
}

func TestNotificationLifecycle(t *testing.T) {
	cleanTables(t)
	n1 := createNotificationViaAPI(t, userTok, "通知A")
	n2 := createNotificationViaAPI(t, userTok, "通知B")
	n1ID := int(n1["id"].(float64))

	// 列表
	resp, out := doJSON(t, http.MethodGet, "/api/v1/notifications", userTok, nil)
	data := dataMap(out)
	list, _ := data["notifications"].([]interface{})
	if resp.StatusCode != http.StatusOK || len(list) != 2 {
		t.Errorf("通知列表应 2 条: %d %v", resp.StatusCode, data)
	}

	// 未读数
	_, out = doJSON(t, http.MethodGet, "/api/v1/notifications/unread-count", userTok, nil)
	if resp.StatusCode != http.StatusOK || dataMap(out)["unread_count"].(float64) != 2 {
		t.Errorf("未读数应为 2: %d %v", resp.StatusCode, out)
	}

	// 标记单条已读
	resp, out = doJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/notifications/%d/read", n1ID), userTok, nil)
	if resp.StatusCode != http.StatusOK || dataMap(out)["status"] != "已标记为已读" {
		t.Errorf("标记已读失败: %d %v", resp.StatusCode, out)
	}
	_, out = doJSON(t, http.MethodGet, "/api/v1/notifications/unread-count", userTok, nil)
	if dataMap(out)["unread_count"].(float64) != 1 {
		t.Errorf("标记后未读数应为 1: %v", out)
	}

	// 全部已读
	resp, _ = doJSON(t, http.MethodPatch, "/api/v1/notifications/read-all", userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("全部已读应 200, got %d", resp.StatusCode)
	}
	_, out = doJSON(t, http.MethodGet, "/api/v1/notifications/unread-count", userTok, nil)
	if dataMap(out)["unread_count"].(float64) != 0 {
		t.Errorf("全部已读后未读数应为 0: %v", out)
	}

	// 删除
	resp, out = doJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/notifications/%d", int(n2["id"].(float64))), userTok, nil)
	if resp.StatusCode != http.StatusOK || dataMap(out)["status"] != "已删除" {
		t.Errorf("删除通知失败: %d %v", resp.StatusCode, out)
	}

	// 用户隔离：B 看不到 A 的通知
	resp, out = doJSON(t, http.MethodGet, "/api/v1/notifications", user2Tok, nil)
	if list, _ := dataMap(out)["notifications"].([]interface{}); resp.StatusCode != http.StatusOK || len(list) != 0 {
		t.Errorf("用户 B 不应看到 A 的通知: %v", out)
	}
}

func TestCreateNotificationValidation(t *testing.T) {
	cleanTables(t)
	resp, _ := doJSON(t, http.MethodPost, "/api/v1/notifications", userTok, map[string]string{"type": "system"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("缺标题/内容应 400, got %d", resp.StatusCode)
	}
}

// ---------------- 数据导出与导入 ----------------

func TestExportJSON(t *testing.T) {
	cleanTables(t)
	createTaskViaAPI(t, userTok, "导出任务1")
	createTaskViaAPI(t, userTok, "导出任务2")

	req, _ := http.NewRequest(http.MethodGet, testTS.URL+"/api/v1/export?type=tasks&format=json", nil)
	req.Header.Set("Authorization", "Bearer "+userTok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("导出应 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("JSON 导出 Content-Type 错误: %q", ct)
	}
	var tasks []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		t.Fatalf("导出 JSON 解析失败: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("导出应含 2 个任务, got %d", len(tasks))
	}
}

func TestExportCSV(t *testing.T) {
	cleanTables(t)
	createTaskViaAPI(t, userTok, "CSV任务")

	req, _ := http.NewRequest(http.MethodGet, testTS.URL+"/api/v1/export?type=tasks&format=csv", nil)
	req.Header.Set("Authorization", "Bearer "+userTok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CSV 导出应 200, got %d", resp.StatusCode)
	}
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	if !bytes.Contains(buf.Bytes(), []byte("id,local_id,server_version,title")) {
		t.Errorf("CSV 导出缺少表头: %q", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte("CSV任务")) {
		t.Errorf("CSV 导出缺少任务数据: %q", buf.String())
	}
}

func TestExportInvalidType(t *testing.T) {
	cleanTables(t)
	resp, _ := doJSON(t, http.MethodGet, "/api/v1/export?type=users&format=json", userTok, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("非法导出类型应 400, got %d", resp.StatusCode)
	}
}

func importFile(t *testing.T, token, filename, content, format string) (*http.Response, map[string]interface{}) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write([]byte(content))
	_ = mw.WriteField("format", format)
	mw.Close()
	req, _ := http.NewRequest(http.MethodPost, testTS.URL+"/api/v1/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func TestImportJSON(t *testing.T) {
	cleanTables(t)
	content := `[{"title":"导入任务1","description":"d1","status":"todo","priority":"high"},{"title":"导入任务2","status":"done"}]`
	resp, out := importFile(t, userTok, "tasks.json", content, "json")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("JSON 导入应 200, got %d %v", resp.StatusCode, out)
	}
	// 导入的任务应可见
	_, list := doJSON(t, http.MethodGet, "/api/v1/tasks", userTok, nil)
	if tasks, _ := dataMap(list)["tasks"].([]interface{}); len(tasks) != 2 {
		t.Errorf("导入后应有 2 个任务: %v", list)
	}
}

func TestImportCSV(t *testing.T) {
	cleanTables(t)
	content := "title,status,priority\nCSV导入任务,todo,low\n"
	resp, out := importFile(t, userTok, "tasks.csv", content, "csv")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CSV 导入应 200, got %d %v", resp.StatusCode, out)
	}
	_, list := doJSON(t, http.MethodGet, "/api/v1/tasks", userTok, nil)
	if tasks, _ := dataMap(list)["tasks"].([]interface{}); len(tasks) != 1 {
		t.Errorf("CSV 导入后应有 1 个任务: %v", list)
	}
}

func TestImportInvalidFormat(t *testing.T) {
	cleanTables(t)
	resp, _ := importFile(t, userTok, "tasks.json", `[{"title":"x"}]`, "xml")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("非法导入格式应 400, got %d", resp.StatusCode)
	}
}

// ---------------- 管理接口 ----------------

func TestAdminForbiddenForNormalUser(t *testing.T) {
	cleanTables(t)
	resp, _ := doJSON(t, http.MethodGet, "/api/v1/admin/users", userTok, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("普通用户访问 admin 应 403, got %d", resp.StatusCode)
	}
}

func TestAdminCreateAndListUsers(t *testing.T) {
	cleanTables(t)
	resp, out := doJSON(t, http.MethodPost, "/api/v1/admin/users", adminTok, map[string]string{
		"email": "newuser@test.local", "password": "NewUser@2026", "role": "user",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("admin 建用户应 201, got %d %v", resp.StatusCode, out)
	}
	newID := int(dataMap(out)["id"].(float64))

	resp, out = doJSON(t, http.MethodGet, "/api/v1/admin/users", adminTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("admin 列用户应 200, got %d", resp.StatusCode)
	}
	if users, _ := dataMap(out)["users"].([]interface{}); len(users) < 4 {
		t.Errorf("用户列表应含全部用户: %v", out)
	}

	// 新用户可登录
	body := []byte(`{"email":"newuser@test.local","password":"NewUser@2026"}`)
	httpReq, _ := http.NewRequest(http.MethodPost, testTS.URL+"/api/v1/auth/login", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	loginResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatal(err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Errorf("新建用户应能登录, got %d", loginResp.StatusCode)
	}

	// 锁定/解锁
	resp, _ = doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/lock", newID), adminTok, map[string]int{"duration_minutes": 1})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("锁定用户应 200, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/unlock", newID), adminTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("解锁用户应 200, got %d", resp.StatusCode)
	}

	// 重置密码
	resp, _ = doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/password", newID), adminTok, map[string]string{"new_password": "Reset@2026pass"})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("重置密码应 200, got %d", resp.StatusCode)
	}

	// 删除用户
	resp, _ = doJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", newID), adminTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("删除用户应 200, got %d", resp.StatusCode)
	}
}

func TestAdminLogsAndConfig(t *testing.T) {
	cleanTables(t)
	// 先产生一条 admin 动作日志
	doJSON(t, http.MethodPost, "/api/v1/admin/users", adminTok, map[string]string{
		"email": "loguser@test.local", "password": "LogUser@2026", "role": "user",
	})

	resp, _ := doJSON(t, http.MethodGet, "/api/v1/admin/logs/actions", adminTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("动作日志应 200, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodGet, "/api/v1/admin/logs/login", adminTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("登录日志应 200, got %d", resp.StatusCode)
	}

	// 配置读写
	resp, _ = doJSON(t, http.MethodGet, "/api/v1/admin/config", adminTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("读取配置应 200, got %d", resp.StatusCode)
	}
	resp, out := doJSON(t, http.MethodPut, "/api/v1/admin/config", adminTok, map[string]string{
		"key": "test_key", "value": "test_value", "description": "测试配置",
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("写入配置应 200, got %d %v", resp.StatusCode, out)
	}
}

// ---------------- 设备配对 ----------------

func TestDevicePairingFlow(t *testing.T) {
	cleanTables(t)
	resp, out := doJSON(t, http.MethodPost, "/api/v1/devices/pair/init", userTok, map[string]string{
		"device_type": "android", "device_id": "test-device-001",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("发起配对应 200, got %d %v", resp.StatusCode, out)
	}
	data := dataMap(out)
	if data["key"] == "" || data["server_url"] == "" {
		t.Errorf("配对响应应含 key 与 server_url: %v", data)
	}

	resp, _ = doJSON(t, http.MethodGet, "/api/v1/devices", userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("设备列表应 200, got %d", resp.StatusCode)
	}
}

// ---------------- 响应安全头（经完整中间件链验证） ----------------

func TestSecurityHeadersViaServer(t *testing.T) {
	cleanTables(t)
	req, _ := http.NewRequest(http.MethodGet, testTS.URL+"/api/v1/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	checks := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Content-Security-Policy": "default-src 'self'",
	}
	for k, want := range checks {
		if got := resp.Header.Get(k); got != want {
			t.Errorf("安全头 %s = %q, want %q", k, got, want)
		}
	}
}

// 确保编译引用（db 包在清理辅助中使用）
var _ = db.DB
