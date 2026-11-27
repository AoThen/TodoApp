package main

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"todoapp/internal/db"
)

func userIDByEmail(t *testing.T, email string) int {
	t.Helper()
	var uid int
	if err := db.DB.QueryRow("SELECT id FROM users WHERE email = ?", email).Scan(&uid); err != nil {
		t.Fatalf("查询用户 %s 失败: %v", email, err)
	}
	return uid
}

// ---------------- 设备配对 ----------------

func pairInit(t *testing.T, token, deviceType string) map[string]interface{} {
	t.Helper()
	body := map[string]string{}
	if deviceType != "" {
		body["device_type"] = deviceType
	}
	resp, out := doJSON(t, http.MethodPost, "/api/v1/devices/pair/init", token, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pair init 应 200, got %d: %v", resp.StatusCode, out)
	}
	return dataMap(out)
}

func TestDevicePairingLifecycle(t *testing.T) {
	cleanTables(t)

	// init：生成 64 字符 key，device_type 缺省为 web
	initOut := pairInit(t, userTok, "")
	key, _ := initOut["key"].(string)
	if len(key) != 64 {
		t.Fatalf("配对密钥应为 64 字符, got %q", key)
	}
	if initOut["device_id"] == "" || initOut["server_url"] == "" || initOut["expires_at"] == "" {
		t.Fatalf("init 应返回 device_id/server_url/expires_at: %v", initOut)
	}

	// list：pending 行可见且 is_active=false
	resp, out := doJSON(t, http.MethodGet, "/api/v1/devices", userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("设备列表应 200, got %d: %v", resp.StatusCode, out)
	}
	devices, _ := dataMap(out)["devices"].([]interface{})
	if len(devices) != 1 {
		t.Fatalf("init 后应有 1 条设备记录, got %d", len(devices))
	}
	if dev, _ := devices[0].(map[string]interface{}); dev["is_active"] == true {
		t.Fatal("待配对设备应为 is_active=false")
	}

	// 参数校验：缺 device_id / key 长度不足 / 非法设备类型
	resp, _ = doJSON(t, http.MethodPost, "/api/v1/devices/pair", userTok, map[string]string{"key": key, "device_type": "android"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("缺 device_id 应 400, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodPost, "/api/v1/devices/pair", userTok, map[string]string{"key": "short", "device_type": "android", "device_id": "d1"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("key 长度非法应 400, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodPost, "/api/v1/devices/pair", userTok, map[string]string{"key": key, "device_type": "linux", "device_id": "d1"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("非法设备类型应 400, got %d", resp.StatusCode)
	}

	// 正式配对
	resp, out = doJSON(t, http.MethodPost, "/api/v1/devices/pair", userTok, map[string]string{"key": key, "device_type": "android", "device_id": "android-1", "client_public_key": mustClientPub(t)})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pair 应 200, got %d: %v", resp.StatusCode, out)
	}
	if dataMap(out)["status"] != "paired" {
		t.Fatalf("pair 应返回 paired, got %v", out)
	}

	// 已消费的 pending 行被清理，仅剩 1 条 is_active=1 的设备
	var pending, active int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM devices WHERE pairing_key = ? AND is_active = 0", key).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM devices WHERE pairing_key = ? AND is_active = 1", key).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if pending != 0 || active != 1 {
		t.Fatalf("配对后应剩 1 条激活设备且无 pending 行, got pending=%d active=%d", pending, active)
	}

	// 同一 key 再次配对 → 409
	resp, _ = doJSON(t, http.MethodPost, "/api/v1/devices/pair", userTok, map[string]string{"key": key, "device_type": "android", "device_id": "android-2", "client_public_key": mustClientPub(t)})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("重复 key 配对应 409, got %d", resp.StatusCode)
	}

	// regenerate：设备不存在 404 / 非所有者 403 / 成功返回新 key
	resp, _ = doJSON(t, http.MethodPost, "/api/v1/devices/no-such-device/regenerate", userTok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("regenerate 不存在设备应 404, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodPost, "/api/v1/devices/android-1/regenerate", user2Tok, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("非所有者 regenerate 应 403, got %d", resp.StatusCode)
	}
	resp, out = doJSON(t, http.MethodPost, "/api/v1/devices/android-1/regenerate", userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("regenerate 应 200, got %d: %v", resp.StatusCode, out)
	}
	newKey, _ := dataMap(out)["new_key"].(string)
	if len(newKey) != 64 {
		t.Fatalf("新配对密钥应为 64 字符, got %q", newKey)
	}

	// revoke：非所有者 403 / 成功 / 重复撤销 400
	resp, _ = doJSON(t, http.MethodDelete, "/api/v1/devices/android-1", user2Tok, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("非所有者撤销应 403, got %d", resp.StatusCode)
	}
	resp, out = doJSON(t, http.MethodDelete, "/api/v1/devices/android-1", userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("撤销应 200, got %d: %v", resp.StatusCode, out)
	}
	resp, _ = doJSON(t, http.MethodDelete, "/api/v1/devices/android-1", userTok, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("重复撤销应 400, got %d", resp.StatusCode)
	}
}

// ---------------- 任务撤销恢复 ----------------

func TestRestoreDeletedTaskFlow(t *testing.T) {
	cleanTables(t)

	task := createTaskViaAPI(t, userTok, "待恢复任务")
	id := int64(task["id"].(float64))

	// 批量删除（写入 deleted_tasks，30 秒内可撤销）
	resp, out := doJSON(t, http.MethodDelete, "/api/v1/tasks/batch", userTok, map[string]interface{}{"task_ids": []int64{id}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("batch delete 应 200, got %d: %v", resp.StatusCode, out)
	}

	// 他人恢复 → 非 2xx，任务仍处于删除态
	resp, _ = doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/restore", id), user2Tok, nil)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		t.Fatalf("他人 restore 不应成功, got %d", resp.StatusCode)
	}

	// 恢复成功且任务重新可见
	resp, out = doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/restore", id), userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("restore 应 200, got %d: %v", resp.StatusCode, out)
	}
	if dataMap(out)["status"] != "restored" {
		t.Fatalf("restore 应返回 restored, got %v", out)
	}
	resp, _ = doJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", id), userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("恢复后任务应可见, got %d", resp.StatusCode)
	}

	// 重复恢复（is_restorable 已置 0）→ 非 2xx
	resp, _ = doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/restore", id), userTok, nil)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		t.Fatalf("重复 restore 不应成功, got %d", resp.StatusCode)
	}

	// 超过 30 秒撤销期限 → 410
	expired := createTaskViaAPI(t, userTok, "过期撤销任务")
	expiredID := int64(expired["id"].(float64))
	resp, _ = doJSON(t, http.MethodDelete, "/api/v1/tasks/batch", userTok, map[string]interface{}{"task_ids": []int64{expiredID}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("batch delete 应 200, got %d", resp.StatusCode)
	}
	if _, err := db.DB.Exec("UPDATE deleted_tasks SET deleted_at = ?", time.Now().UTC().Add(-31*time.Second)); err != nil {
		t.Fatal(err)
	}
	resp, _ = doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/restore", expiredID), userTok, nil)
	if resp.StatusCode != http.StatusGone {
		t.Fatalf("超期 restore 应 410, got %d", resp.StatusCode)
	}
}

// ---------------- 通知清空 ----------------

func TestClearNotifications(t *testing.T) {
	cleanTables(t)
	uid := userIDByEmail(t, "user@test.local")

	// 2 条 40 天前的旧通知
	for i := 0; i < 2; i++ {
		if _, err := db.CreateNotification(uid, "task_created", "旧通知", "内容", "normal", nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.DB.Exec("UPDATE notifications SET created_at = ?", time.Now().UTC().Add(-40*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	// 1 条新通知
	if _, err := db.CreateNotification(uid, "task_created", "新通知", "内容", "normal", nil); err != nil {
		t.Fatal(err)
	}

	// 按 older_than_days 清理：只清 35 天前的旧通知
	resp, out := doJSON(t, http.MethodDelete, "/api/v1/notifications/clear?older_than_days=35", userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("clear 应 200, got %d: %v", resp.StatusCode, out)
	}
	if n := int(dataMap(out)["cleared_count"].(float64)); n != 2 {
		t.Fatalf("应清理 2 条旧通知, got %d", n)
	}
	var remain int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = ?", uid).Scan(&remain); err != nil {
		t.Fatal(err)
	}
	if remain != 1 {
		t.Fatalf("应剩 1 条新通知, got %d", remain)
	}

	// 显式 older_than_days=0 全量清空（端点默认只清 30 天前的通知）
	resp, out = doJSON(t, http.MethodDelete, "/api/v1/notifications/clear?older_than_days=0", userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("clear 应 200, got %d: %v", resp.StatusCode, out)
	}
	if n := int(dataMap(out)["cleared_count"].(float64)); n != 1 {
		t.Fatalf("应清理 1 条通知, got %d", n)
	}
}

// ---------------- 管理员更新用户 ----------------

func TestAdminUpdateUser(t *testing.T) {
	cleanTables(t)
	uid := userIDByEmail(t, "user2@test.local")
	// users 表不在 cleanTables 范围内，恢复被本用例修改的 email/role，避免污染后续用例
	t.Cleanup(func() {
		_, _ = db.DB.Exec("UPDATE users SET email = 'user2@test.local', role = 'user' WHERE id = ?", uid)
	})
	path := fmt.Sprintf("/api/v1/admin/users/%d", uid)

	// 非 admin → 403
	resp, _ := doJSON(t, http.MethodPatch, path, userTok, map[string]string{"role": "admin"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("非 admin 更新用户应 403, got %d", resp.StatusCode)
	}

	// 非法邮箱 / 非法角色 → 400
	resp, _ = doJSON(t, http.MethodPatch, path, adminTok, map[string]string{"email": "not-an-email"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("非法邮箱应 400, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodPatch, path, adminTok, map[string]string{"role": "superadmin"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("非法角色应 400, got %d", resp.StatusCode)
	}

	// 成功更新邮箱与角色
	resp, out := doJSON(t, http.MethodPatch, path, adminTok, map[string]string{"email": "user2.new@test.local", "role": "admin"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("更新用户应 200, got %d: %v", resp.StatusCode, out)
	}
	var email, role string
	if err := db.DB.QueryRow("SELECT email, role FROM users WHERE id = ?", uid).Scan(&email, &role); err != nil {
		t.Fatal(err)
	}
	if email != "user2.new@test.local" || role != "admin" {
		t.Fatalf("更新未生效: email=%s role=%s", email, role)
	}

	// 仅更新角色，邮箱保持不变
	resp, _ = doJSON(t, http.MethodPatch, path, adminTok, map[string]string{"role": "user"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("更新角色应 200, got %d", resp.StatusCode)
	}
	if err := db.DB.QueryRow("SELECT email, role FROM users WHERE id = ?", uid).Scan(&email, &role); err != nil {
		t.Fatal(err)
	}
	if email != "user2.new@test.local" || role != "user" {
		t.Fatalf("部分更新行为异常: email=%s role=%s", email, role)
	}
}

// ---------------- 冲突记录清理 ----------------

func TestCleanupOldConflicts(t *testing.T) {
	cleanTables(t)

	// 1 条 31 天前的旧冲突
	if err := db.RecordConflict(1, "old-local", 11, "version_conflict", "[]"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("UPDATE conflicts SET created_at = ?", time.Now().UTC().Add(-31*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	// 1 条新冲突
	if err := db.RecordConflict(1, "new-local", 12, "version_conflict", "[]"); err != nil {
		t.Fatal(err)
	}

	n, err := db.CleanupOldConflicts(30)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("应清理 1 条过期冲突, got %d", n)
	}
	var cnt int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM conflicts").Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 1 {
		t.Fatalf("应剩 1 条未过期冲突, got %d", cnt)
	}
}
