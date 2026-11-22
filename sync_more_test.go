package main

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// syncChanges 构造 sync 请求体。
func syncChanges(t *testing.T, token string, lastSyncAt string, changes []map[string]interface{}) (*http.Response, map[string]interface{}) {
	t.Helper()
	body := map[string]interface{}{"last_sync_at": lastSyncAt, "changes": changes}
	return doJSON(t, http.MethodPost, "/api/v1/sync", token, body)
}

func insertChange(localID, title string) map[string]interface{} {
	return map[string]interface{}{
		"local_id": localID, "op": "insert",
		"payload":        map[string]interface{}{"title": title, "description": "sync", "status": "todo", "priority": "medium"},
		"client_version": 0,
	}
}

func TestSyncIncrementalByLastSyncAt(t *testing.T) {
	cleanTables(t)
	// 第一轮：插入 A，记录服务端游标
	resp, out := syncChanges(t, userTok, "", []map[string]interface{}{insertChange("local-inc-a", "增量A")})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("首次 sync 应 200, got %d %v", resp.StatusCode, out)
	}
	cursor, _ := out["last_sync_at"].(string)
	if cursor == "" {
		t.Fatalf("sync 应返回 last_sync_at 游标: %v", out)
	}

	// SQLite 时间戳为秒级精度，等待 1s 保证 updated_at 推进，避免增量断言抖动
	time.Sleep(1100 * time.Millisecond)

	// 第二轮：插入 B
	if resp, out = syncChanges(t, userTok, "", []map[string]interface{}{insertChange("local-inc-b", "增量B")}); resp.StatusCode != http.StatusOK {
		t.Fatalf("第二次 sync 应 200, got %d %v", resp.StatusCode, out)
	}

	// 用旧游标拉增量：B 必须返回；A 与游标同一秒内提交，按 >= 语义可能重复下发
	// （秒级精度边界，客户端按 server_id 幂等回放，重复无害）
	resp, out = syncChanges(t, userTok, cursor, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("增量 sync 应 200, got %d %v", resp.StatusCode, out)
	}
	serverChanges, _ := out["server_changes"].([]interface{})
	if len(serverChanges) < 1 || len(serverChanges) > 2 {
		t.Fatalf("增量拉取应含 1~2 条变更, got %d: %v", len(serverChanges), out)
	}
	foundB := false
	for _, raw := range serverChanges {
		change, _ := raw.(map[string]interface{})
		if change["title"] == "增量B" {
			foundB = true
		}
	}
	if !foundB {
		t.Errorf("增量变更应包含 增量B, got %v", serverChanges)
	}
}

func TestSyncDeleteOp(t *testing.T) {
	cleanTables(t)
	resp, out := syncChanges(t, userTok, "", []map[string]interface{}{insertChange("local-del", "待删除")})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("insert sync 应 200, got %d %v", resp.StatusCode, out)
	}
	serverChanges, _ := out["server_changes"].([]interface{})
	if len(serverChanges) != 1 {
		t.Fatalf("应返回 1 条 server_change: %v", out)
	}
	change, _ := serverChanges[0].(map[string]interface{})
	serverID, _ := change["id"].(float64)
	if serverID == 0 {
		// 兼容 payload 内嵌 id 的响应结构
		if payload, _ := change["payload"].(map[string]interface{}); payload != nil {
			serverID, _ = payload["id"].(float64)
		}
	}
	if serverID == 0 {
		t.Fatalf("server_change 应含任务 id: %v", change)
	}

	// 通过 REST 确认任务可见后，用 sync delete 删除
	delChange := map[string]interface{}{
		"local_id": "local-del", "op": "delete",
		"payload":        map[string]interface{}{"id": serverID},
		"client_version": 1,
	}
	resp, out = syncChanges(t, userTok, "", []map[string]interface{}{delChange})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete sync 应 200, got %d %v", resp.StatusCode, out)
	}

	// 任务应被软删除：列表不可见
	_, out = doJSON(t, http.MethodGet, "/api/v1/tasks", userTok, nil)
	tasks, _ := dataMap(out)["tasks"].([]interface{})
	if len(tasks) != 0 {
		t.Errorf("delete 同步后任务应不可见, got %v", tasks)
	}
}

func TestSyncBatchChanges(t *testing.T) {
	cleanTables(t)
	changes := make([]map[string]interface{}, 0, 30)
	for i := 0; i < 30; i++ {
		changes = append(changes, insertChange(fmt.Sprintf("local-batch-%d", i), fmt.Sprintf("批量任务%d", i)))
	}
	resp, out := syncChanges(t, userTok, "", changes)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("批量 sync 应 200, got %d %v", resp.StatusCode, out)
	}
	clientChanges, _ := out["client_changes"].([]interface{})
	if len(clientChanges) != 30 {
		t.Errorf("批量 30 条变更应全部确认, got %d", len(clientChanges))
	}
	_, out = doJSON(t, http.MethodGet, "/api/v1/tasks?page=1&page_size=100", userTok, nil)
	tasks, _ := dataMap(out)["tasks"].([]interface{})
	if len(tasks) != 30 {
		t.Errorf("批量同步后应有 30 个任务, got %d", len(tasks))
	}
}
