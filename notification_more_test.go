package main

import (
	"fmt"
	"net/http"
	"testing"
)

func TestNotificationClearAndUnread(t *testing.T) {
	cleanTables(t)
	createNotificationViaAPI(t, userTok, "清理通知A")
	createNotificationViaAPI(t, userTok, "清理通知B")

	// 全部已读后再清空
	resp, out := doJSON(t, http.MethodPatch, "/api/v1/notifications/read-all", userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("read-all 应 200, got %d %v", resp.StatusCode, out)
	}
	if marked, _ := dataMap(out)["marked_count"].(float64); marked != 2 {
		t.Errorf("read-all 应标记 2 条, got %v", out)
	}

	// older_than_days=0 清空全部通知（默认仅清理 30 天前的旧通知）
	resp, _ = doJSON(t, http.MethodDelete, "/api/v1/notifications/clear?older_than_days=0", userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("clear 应 200, got %d", resp.StatusCode)
	}

	_, out = doJSON(t, http.MethodGet, "/api/v1/notifications", userTok, nil)
	if list, _ := dataMap(out)["notifications"].([]interface{}); len(list) != 0 {
		t.Errorf("clear 后列表应为空: %v", out)
	}
	_, out = doJSON(t, http.MethodGet, "/api/v1/notifications/unread-count", userTok, nil)
	if unread, _ := dataMap(out)["unread_count"].(float64); unread != 0 {
		t.Errorf("clear 后未读数应为 0: %v", out)
	}
}

func TestNotificationCrossUserMarkReadRejected(t *testing.T) {
	cleanTables(t)
	n := createNotificationViaAPI(t, userTok, "他人通知")
	nID := int(n["id"].(float64))

	// user2 标记 user 的通知已读 → 404（IDOR 防护）
	resp, _ := doJSON(t, http.MethodPatch, fmt.Sprintf("/api/v1/notifications/%d/read", nID), user2Tok, nil)
	if resp.StatusCode < 400 {
		t.Errorf("跨用户标记已读应被拒绝, got %d", resp.StatusCode)
	}
	// user2 删除 user 的通知 → 404
	resp, _ = doJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/notifications/%d", nID), user2Tok, nil)
	if resp.StatusCode < 400 {
		t.Errorf("跨用户删除通知应被拒绝, got %d", resp.StatusCode)
	}
	// 原通知未被破坏
	_, out := doJSON(t, http.MethodGet, "/api/v1/notifications", userTok, nil)
	if list, _ := dataMap(out)["notifications"].([]interface{}); len(list) != 1 {
		t.Errorf("原通知应仍存在: %v", out)
	}
}

func TestNotificationMarkNonexistent(t *testing.T) {
	cleanTables(t)
	resp, _ := doJSON(t, http.MethodPatch, "/api/v1/notifications/999999/read", userTok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("标记不存在通知应 404, got %d", resp.StatusCode)
	}
}
