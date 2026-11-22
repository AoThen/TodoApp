package main

import (
	"net/http"
	"strings"
	"testing"
)

// pairDevice 走完 init → pair 流程，返回 pair 响应 data。
func pairDevice(t *testing.T, token, deviceID string) map[string]interface{} {
	t.Helper()
	_, out := doJSON(t, http.MethodPost, "/api/v1/devices/pair/init", token, map[string]string{
		"device_type": "android", "device_id": deviceID,
	})
	key, _ := dataMap(out)["key"].(string)
	if key == "" {
		t.Fatalf("init 配对应返回 key: %v", out)
	}
	resp, out := doJSON(t, http.MethodPost, "/api/v1/devices/pair", token, map[string]string{
		"key": key, "device_type": "android", "device_id": deviceID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pair 应 200, got %d %v", resp.StatusCode, out)
	}
	return dataMap(out)
}

func TestDevicePairInvalidInputs(t *testing.T) {
	cleanTables(t)
	// key 长度不是 64
	resp, _ := doJSON(t, http.MethodPost, "/api/v1/devices/pair", userTok, map[string]string{
		"key": "short-key", "device_type": "android", "device_id": "dev-invalid",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("非法 key 长度应 400, got %d", resp.StatusCode)
	}
	// 非法设备类型
	resp, _ = doJSON(t, http.MethodPost, "/api/v1/devices/pair", userTok, map[string]string{
		"key": strings.Repeat("a", 64), "device_type": "router", "device_id": "dev-invalid",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("非法设备类型应 400, got %d", resp.StatusCode)
	}
}

func TestDevicePairDuplicateKey(t *testing.T) {
	cleanTables(t)
	_, out := doJSON(t, http.MethodPost, "/api/v1/devices/pair/init", userTok, map[string]string{
		"device_type": "android", "device_id": "dev-dup",
	})
	key := dataMap(out)["key"].(string)
	body := map[string]string{"key": key, "device_type": "android", "device_id": "dev-dup"}
	resp, _ := doJSON(t, http.MethodPost, "/api/v1/devices/pair", userTok, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("首次配对应 200, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodPost, "/api/v1/devices/pair", userTok, body)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("重复配对同 key 应 409, got %d", resp.StatusCode)
	}
}

func TestDeviceRegenerateReturnsNewKey(t *testing.T) {
	cleanTables(t)
	pairDevice(t, userTok, "dev-regen")
	_, out := doJSON(t, http.MethodPost, "/api/v1/devices/pair/init", userTok, map[string]string{
		"device_type": "android", "device_id": "dev-regen-2",
	})
	oldKey := dataMap(out)["key"].(string)

	resp, out := doJSON(t, http.MethodPost, "/api/v1/devices/dev-regen/regenerate", userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("regenerate 应 200, got %d %v", resp.StatusCode, out)
	}
	newKey, _ := dataMap(out)["new_key"].(string)
	if len(newKey) != 64 {
		t.Errorf("新 key 应为 64 字符 hex, got %q", newKey)
	}
	if newKey == oldKey {
		t.Error("新 key 不应与旧 key 相同")
	}
}

func TestDeviceRevokeLifecycle(t *testing.T) {
	cleanTables(t)
	pairDevice(t, userTok, "dev-revoke")

	resp, _ := doJSON(t, http.MethodDelete, "/api/v1/devices/dev-revoke", userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("撤销设备应 200, got %d", resp.StatusCode)
	}

	// 已撤销设备：重复撤销 → 400
	resp, _ = doJSON(t, http.MethodDelete, "/api/v1/devices/dev-revoke", userTok, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("重复撤销应 400, got %d", resp.StatusCode)
	}
	// 已撤销设备：regenerate → 400
	resp, _ = doJSON(t, http.MethodPost, "/api/v1/devices/dev-revoke/regenerate", userTok, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("已撤销设备 regenerate 应 400, got %d", resp.StatusCode)
	}
	// 不存在设备 → 404
	resp, _ = doJSON(t, http.MethodDelete, "/api/v1/devices/no-such-device", userTok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("撤销不存在设备应 404, got %d", resp.StatusCode)
	}
}

func TestDeviceCrossUserIsolation(t *testing.T) {
	cleanTables(t)
	pairDevice(t, userTok, "dev-iso")

	// user2 操作 user 的设备 → 403
	resp, _ := doJSON(t, http.MethodPost, "/api/v1/devices/dev-iso/regenerate", user2Tok, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("跨用户 regenerate 应 403, got %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodDelete, "/api/v1/devices/dev-iso", user2Tok, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("跨用户撤销应 403, got %d", resp.StatusCode)
	}

	// 设备列表按用户隔离
	_, out := doJSON(t, http.MethodGet, "/api/v1/devices", user2Tok, nil)
	if count := dataMap(out)["count"].(float64); count != 0 {
		t.Errorf("user2 设备数应为 0, got %v", dataMap(out))
	}
	_, out = doJSON(t, http.MethodGet, "/api/v1/devices", userTok, nil)
	if count := dataMap(out)["count"].(float64); count != 1 {
		t.Errorf("user 设备数应为 1, got %v", dataMap(out))
	}
}
