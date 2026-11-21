package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"
)

// hashToken 对令牌做 SHA-256 摘要后再落库，避免明文存储令牌 Material
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// BlacklistToken 将访问令牌加入黑名单（登出后立即失效）。
// expiresAt 取令牌自身的过期时间，到期后由清理任务移除。
func BlacklistToken(token string, expiresAt time.Time) error {
	_, err := DB.Exec(
		"INSERT OR REPLACE INTO token_blacklist (token_hash, expires_at) VALUES (?, ?)",
		hashToken(token), expiresAt.UTC(),
	)
	return err
}

// IsTokenBlacklisted 判断访问令牌是否已被拉黑（登出即失效）
func IsTokenBlacklisted(token string) (bool, error) {
	var one int
	err := DB.QueryRow(
		"SELECT 1 FROM token_blacklist WHERE token_hash = ? AND expires_at > ?",
		hashToken(token), time.Now().UTC(),
	).Scan(&one)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// cleanupExpiredBlacklistedTokens 清理已过期的黑名单记录，防止表无限增长
func cleanupExpiredBlacklistedTokens() error {
	_, err := DB.Exec("DELETE FROM token_blacklist WHERE expires_at <= ?", time.Now().UTC())
	return err
}
