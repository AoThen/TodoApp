package db

import (
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
		var deviceType, deviceID, serverURL string
		var pairedAt, lastSeen time.Time
		var isActive bool

		err := rows.Scan(&id, &deviceType, &deviceID, &serverURL, &pairedAt, &lastSeen, &isActive)
		if err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"id":          id,
			"device_type": deviceType,
			"device_id":   deviceID,
			"server_url":  serverURL,
			"paired_at":   pairedAt.Format(time.RFC3339),
			"last_seen":   lastSeen.Format(time.RFC3339),
			"is_active":   isActive,
		})
	}
	return results, nil
}

// UpdateDeviceLastSeen 更新设备最后活跃时间
func UpdateDeviceLastSeen(deviceID string) error {
	now := time.Now().UTC()
	_, err := DB.Exec(
		"UPDATE devices SET last_seen = ? WHERE device_id = ?",
		now, deviceID,
	)
	return err
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

// GetDeviceCount 获取用户设备数量
func GetDeviceCount(userID int) (int, error) {
	var count int
	err := DB.QueryRow(
		"SELECT COUNT(*) FROM devices WHERE user_id = ? AND is_active = 1",
		userID,
	).Scan(&count)
	return count, err
}
