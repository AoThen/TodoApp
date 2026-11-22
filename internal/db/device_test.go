package db

import (
	"testing"
	"time"
)

// TestPairingExpiryLifecycle 验证待配对密钥的存在性/过期校验与过期清理
func TestPairingExpiryLifecycle(t *testing.T) {
	setupTestDB(t)

	userID := int(createTestUser(t, "pair@test.com", "user"))

	key, expiresAt, err := CreatePendingPairing(userID, "android")
	if err != nil {
		t.Fatalf("创建待配对密钥失败: %v", err)
	}
	if len(key) != 64 {
		t.Errorf("配对密钥应为 64 字符 hex，got %d", len(key))
	}

	expTime, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		t.Fatalf("过期时间应为 RFC3339 格式: %v", err)
	}
	if diff := time.Until(expTime); diff < 4*time.Minute || diff > 6*time.Minute {
		t.Errorf("过期时间应在约 5 分钟后，got %v", diff)
	}

	// 新签发的密钥：存在且未过期
	exists, expired, err := ValidatePendingPairing(userID, key)
	if err != nil || !exists || expired {
		t.Fatalf("新签发的配对密钥应存在且未过期，got exists=%v expired=%v err=%v", exists, expired, err)
	}

	// 篡改过期时间为过去：应判定过期
	past := time.Now().UTC().Add(-time.Minute)
	if _, err := DB.Exec("UPDATE devices SET pairing_expires_at = ? WHERE pairing_key = ?", past, key); err != nil {
		t.Fatalf("更新过期时间失败: %v", err)
	}
	exists, expired, err = ValidatePendingPairing(userID, key)
	if err != nil || !exists || !expired {
		t.Fatalf("过期密钥应判定为已过期，got exists=%v expired=%v err=%v", exists, expired, err)
	}

	// 过期记录应被清理
	if err := DeleteExpiredPendingPairings(userID); err != nil {
		t.Fatalf("清理过期记录失败: %v", err)
	}
	exists, _, err = ValidatePendingPairing(userID, key)
	if err != nil || exists {
		t.Fatalf("清理后密钥应不存在，got exists=%v err=%v", exists, err)
	}

	// 伪造密钥：不存在，校验不通过
	if exists, _, _ = ValidatePendingPairing(userID, "forged-key"); exists {
		t.Error("伪造密钥不应通过校验")
	}

	// 存量行无过期时间（NULL）：按已过期处理，且应被清理
	if _, err := DB.Exec(
		"INSERT INTO devices (user_id, device_type, pairing_key, server_url, paired_at, last_seen, is_active) VALUES (?, 'web', 'legacy-key', '', ?, ?, 0)",
		userID, time.Now().UTC(), time.Now().UTC(),
	); err != nil {
		t.Fatalf("插入存量待配对行失败: %v", err)
	}
	exists, expired, err = ValidatePendingPairing(userID, "legacy-key")
	if err != nil || !exists || !expired {
		t.Fatalf("无过期时间的存量行应按已过期处理，got exists=%v expired=%v err=%v", exists, expired, err)
	}
	if err := DeleteExpiredPendingPairings(userID); err != nil {
		t.Fatalf("清理存量行失败: %v", err)
	}
	if exists, _, _ = ValidatePendingPairing(userID, "legacy-key"); exists {
		t.Error("无过期时间的存量行应被清理")
	}
}
