package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

// RegisterDevice 注册新设备
func RegisterDevice(userID int, deviceType, deviceID, pairingKey, serverURL string) error {
	now := time.Now().UTC()
	_, err := DB.Exec(
		"INSERT INTO devices (user_id, device_type, device_id, pairing_key, server_url, paired_at, last_seen) VALUES (?, ?, ?, ?, ?, ?, ?)",
		userID, deviceType, deviceID, pairingKey, serverURL, now, now,
	)
	return err
}

// ValidatePairingKey 验证配对密钥是否存在
func ValidatePairingKey(userID int, pairingKey string) (bool, error) {
	var count int
	err := DB.QueryRow(
		"SELECT COUNT(*) FROM devices WHERE user_id = ? AND pairing_key = ? AND is_active = 1",
		userID, pairingKey,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetUserDevices 获取用户的设备列表
func GetUserDevices(userID int) ([]map[string]interface{}, error) {
	query := `
		SELECT id, device_type, device_id, server_url, paired_at, last_seen, is_active
		FROM devices
		WHERE user_id = ?
		ORDER BY paired_at DESC
	`
	rows, err := DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int64
		// 待配对设备的 device_id/server_url/paired_at/last_seen 可为 NULL
		var deviceType, deviceID, serverURL sql.NullString
		var pairedAt, lastSeen sql.NullTime
		var isActive sql.NullBool

		err := rows.Scan(&id, &deviceType, &deviceID, &serverURL, &pairedAt, &lastSeen, &isActive)
		if err != nil {
			return nil, err
		}

		pairedAtStr := ""
		if pairedAt.Valid {
			pairedAtStr = pairedAt.Time.Format(time.RFC3339)
		}
		lastSeenStr := ""
		if lastSeen.Valid {
			lastSeenStr = lastSeen.Time.Format(time.RFC3339)
		}

		results = append(results, map[string]interface{}{
			"id":          id,
			"device_type": deviceType.String,
			"device_id":   deviceID.String,
			"server_url":  serverURL.String,
			"paired_at":   pairedAtStr,
			"last_seen":   lastSeenStr,
			"is_active":   isActive.Bool,
		})
	}
	return results, nil
}

// RegeneratePairingKey 重新生成设备配对密钥
func RegeneratePairingKey(deviceID string, newKey string) error {
	_, err := DB.Exec(
		"UPDATE devices SET pairing_key = ? WHERE device_id = ?",
		newKey, deviceID,
	)
	return err
}

// RevokeDevice 撤销设备
func RevokeDevice(deviceID string) error {
	_, err := DB.Exec(
		"UPDATE devices SET is_active = 0 WHERE device_id = ?",
		deviceID,
	)
	return err
}

// GetDeviceInfoByDeviceID 根据device_id获取设备信息
func GetDeviceInfoByDeviceID(deviceID string) (int, string, string, bool, error) {
	var userID int
	var serverURL string
	var lastSeen time.Time
	var isActive bool

	err := DB.QueryRow(
		"SELECT user_id, server_url, last_seen, is_active FROM devices WHERE device_id = ?",
		deviceID,
	).Scan(&userID, &serverURL, &lastSeen, &isActive)

	return userID, serverURL, lastSeen.Format(time.RFC3339), isActive, err
}

// ClearPendingPairing 清理已使用的待配对记录，防止 devices 表堆积 is_active=0 的垃圾行
func ClearPendingPairing(userID int, pairingKey string) error {
	_, err := DB.Exec(
		"DELETE FROM devices WHERE user_id = ? AND pairing_key = ? AND is_active = 0",
		userID, pairingKey,
	)
	return err
}

// GeneratePairingKey 生成64字符随机配对密钥
func GeneratePairingKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreatePendingPairing 创建待确认的配对密钥
func CreatePendingPairing(userID int, deviceType string) (string, string, error) {
	key, err := GeneratePairingKey()
	if err != nil {
		return "", "", err
	}

	expiresAt := time.Now().Add(5 * time.Minute).UTC()

	_, err = DB.Exec(
		"INSERT INTO devices (user_id, device_type, pairing_key, server_url, paired_at, last_seen, is_active) VALUES (?, ?, ?, ?, ?, ?, ?)",
		userID, deviceType, key, "", time.Now().UTC(), time.Now().UTC(), false,
	)
	if err != nil {
		return "", "", err
	}

	return key, expiresAt.Format(time.RFC3339), nil
}
