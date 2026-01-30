package db

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
	"log"
	"time"
)

var DB *sql.DB

// InitDB initializes the SQLite database and creates core tables if not existing.
func InitDB(dataSourceName string) error {
	var err error
	DB, err = sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return err
	}

	// Enable WAL to improve concurrency
	if _, err = DB.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		log.Println("warning: failed to set WAL mode:", err)
	}

	// Configure connection pool
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(5 * time.Minute)

	// Enable WAL mode
	if _, err = DB.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		log.Println("warning: failed to set WAL mode:", err)
	}

	// Set synchronous mode to NORMAL for better performance
	if _, err = DB.Exec("PRAGMA synchronous = NORMAL;"); err != nil {
		log.Println("warning: failed to set synchronous mode:", err)
	}

	// Create tables
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            email TEXT UNIQUE,
            password_hash TEXT,
            failed_attempts INTEGER DEFAULT 0,
            locked_until DATETIME,
            created_at DATETIME,
            updated_at DATETIME
        );`,
		`CREATE TABLE IF NOT EXISTS login_logs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            email TEXT,
            ip_address TEXT,
            success BOOLEAN,
            timestamp DATETIME,
            FOREIGN KEY(email) REFERENCES users(email)
        );`,
		`CREATE TABLE IF NOT EXISTS tokens (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER,
            access_token TEXT,
            refresh_token TEXT,
            expires_at DATETIME,
            revoked BOOLEAN,
            FOREIGN KEY(user_id) REFERENCES users(id)
        );`,
		`CREATE TABLE IF NOT EXISTS tasks (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER,
            local_id TEXT,
            server_version INTEGER,
            title TEXT,
            description TEXT,
            status TEXT,
            priority TEXT,
            due_at DATETIME,
            created_at DATETIME,
            updated_at DATETIME,
            completed_at DATETIME,
            is_deleted BOOLEAN,
            last_modified DATETIME,
            FOREIGN KEY(user_id) REFERENCES users(id)
        );`,
		`CREATE TABLE IF NOT EXISTS delta_queue (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            local_id TEXT,
            op TEXT,
            payload TEXT,
            client_version INTEGER,
            timestamp DATETIME
        );`,
		`CREATE TABLE IF NOT EXISTS sync_meta (
            user_id INTEGER PRIMARY KEY,
            last_sync_at DATETIME,
            last_server_version INTEGER,
            FOREIGN KEY(user_id) REFERENCES users(id)
        );`,
		`CREATE TABLE IF NOT EXISTS conflicts (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            local_id TEXT,
            server_id INTEGER,
            reason TEXT,
            options TEXT,
            created_at DATETIME
        );`,
		// Indexes for performance
		`CREATE INDEX IF NOT EXISTS idx_tasks_user_id ON tasks(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_server_version ON tasks(server_version);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_last_modified ON tasks(last_modified);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_is_deleted ON tasks(is_deleted);`,
		`CREATE INDEX IF NOT EXISTS idx_tokens_user_id ON tokens(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_tokens_expires_at ON tokens(expires_at);`,
		`CREATE INDEX IF NOT EXISTS idx_login_logs_email ON login_logs(email);`,
		`CREATE INDEX IF NOT EXISTS idx_login_logs_timestamp ON login_logs(timestamp);`,
	}

	for _, s := range stmts {
		if _, err := DB.Exec(s); err != nil {
			return err
		}
	}
	// Seed default data if empty
	seedIfEmpty()
	return nil
}

// seedIfEmpty creates a default user and a sample task if the tables are empty
func seedIfEmpty() {
	var count int
	row := DB.QueryRow("SELECT COUNT(*) FROM users")
	if err := row.Scan(&count); err != nil {
		log.Println("seed: failed to count users:", err)
		return
	}
	if count == 0 {
		password := []byte("password")
		hash, _ := bcrypt.GenerateFromPassword(password, 12)
		now := time.Now().UTC()
		res, err := DB.Exec("INSERT INTO users (email, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)", "test@example.com", string(hash), now, now)
		if err != nil {
			log.Println("seed: failed to insert user:", err)
			return
		}
		userID, _ := res.LastInsertId()
		// sample task
		DB.Exec("INSERT INTO tasks (user_id, local_id, server_version, title, status, created_at, updated_at, last_modified) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", userID, "sample-1", 0, "Sample Task", "todo", now, now, now)
	}
}

// GetAllTasks retrieves all tasks (simplified export)
func GetAllTasks() ([]map[string]interface{}, error) {
	rows, err := DB.Query("SELECT id, local_id, server_version, title, description, status, priority, due_at, created_at, updated_at, completed_at, is_deleted, last_modified FROM tasks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []map[string]interface{}
	for rows.Next() {
		var id int64
		var localID sql.NullString
		var serverVersion sql.NullInt64
		var title sql.NullString
		var description sql.NullString
		var status sql.NullString
		var priority sql.NullString
		var dueAt sql.NullString
		var createdAt sql.NullString
		var updatedAt sql.NullString
		var completedAt sql.NullString
		var isDeleted sql.NullBool
		var lastModified sql.NullString
		if err := rows.Scan(&id, &localID, &serverVersion, &title, &description, &status, &priority, &dueAt, &createdAt, &updatedAt, &completedAt, &isDeleted, &lastModified); err != nil {
			return nil, err
		}
		row := map[string]interface{}{
			"id":       id,
			"local_id": localID.String,
			"server_version": func() interface{} {
				if serverVersion.Valid {
					return serverVersion.Int64
				}
				return nil
			}(),
			"title":         title.String,
			"description":   description.String,
			"status":        status.String,
			"priority":      priority.String,
			"due_at":        dueAt.String,
			"created_at":    createdAt.String,
			"updated_at":    updatedAt.String,
			"completed_at":  completedAt.String,
			"is_deleted":    isDeleted.Bool,
			"last_modified": lastModified.String,
		}
		results = append(results, row)
	}
	return results, nil
}

// SaveRefreshToken persists a refresh token for a user for rotation support
func SaveRefreshToken(userID string, token string, expiresAt time.Time) error {
	_, err := DB.Exec("INSERT INTO tokens (user_id, access_token, refresh_token, expires_at, revoked) VALUES (?, NULL, ?, ?, 0)", userID, token, expiresAt)
	return err
}

// ValidateRefreshToken checks if the given refresh token is valid for the user
func ValidateRefreshToken(userID string, token string) (bool, error) {
	var expiresAt time.Time
	var revoked bool
	err := DB.QueryRow("SELECT expires_at, revoked FROM tokens WHERE user_id = ? AND refresh_token = ?", userID, token).Scan(&expiresAt, &revoked)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if revoked {
		return false, nil
	}
	if expiresAt.Before(time.Now()) {
		return false, nil
	}
	return true, nil
}

// RevokeRefreshToken marks a refresh token as revoked (rotation/logout)
func RevokeRefreshToken(userID string, token string) error {
	_, err := DB.Exec("UPDATE tokens SET revoked = 1 WHERE user_id = ? AND refresh_token = ?", userID, token)
	return err
}

// IsAccountLocked 检查账户是否被锁定
func IsAccountLocked(email string) (bool, time.Time, error) {
	var lockedUntil sql.NullTime
	err := DB.QueryRow("SELECT locked_until FROM users WHERE email = ?", email).Scan(&lockedUntil)
	if err != nil {
		return false, time.Time{}, err
	}
	if lockedUntil.Valid && lockedUntil.Time.After(time.Now()) {
		return true, lockedUntil.Time, nil
	}
	return false, time.Time{}, nil
}

// RecordFailedLogin 记录失败的登录尝试
func RecordFailedLogin(email string) error {
	_, err := DB.Exec(`
		UPDATE users
		SET failed_attempts = COALESCE(failed_attempts, 0) + 1,
		    locked_until = CASE
		        WHEN COALESCE(failed_attempts, 0) + 1 >= 5
		        THEN datetime('now', '+30 minutes')
		        ELSE locked_until
		    END
		WHERE email = ?
	`, email)
	return err
}

// ResetFailedLogin 在成功登录后重置失败次数
func ResetFailedLogin(email string) error {
	_, err := DB.Exec("UPDATE users SET failed_attempts = 0, locked_until = NULL WHERE email = ?", email)
	return err
}

// LogLoginAttempt 记录登录尝试
func LogLoginAttempt(email, ip string, success bool) error {
	_, err := DB.Exec("INSERT INTO login_logs (email, ip_address, success, timestamp) VALUES (?, ?, ?, ?)",
		email, ip, success, time.Now().UTC())
	return err
}

// GetFailedAttemptsCount 获取失败的登录尝试次数
func GetFailedAttemptsCount(email string) (int, error) {
	var count int
	err := DB.QueryRow("SELECT COALESCE(failed_attempts, 0) FROM users WHERE email = ?", email).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ValidateUserCredentials 验证用户邮箱和密码
// 使用 bcrypt 比较哈希密码，包含账户锁定检查
func ValidateUserCredentials(email, password string) (string, error) {
	var userID string
	var passwordHash string

	// 检查账户是否被锁定
	locked, lockedUntil, err := IsAccountLocked(email)
	if err == nil && locked {
		return "", fmt.Errorf("账户已被锁定，将在 %s 后解锁", lockedUntil.Format("2006-01-02 15:04:05"))
	}

	err = DB.QueryRow("SELECT id, password_hash FROM users WHERE email = ?", email).Scan(&userID, &passwordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("用户不存在")
		}
		return "", err
	}

	// 使用 bcrypt 比较哈希密码
	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	if err != nil {
		return "", fmt.Errorf("密码错误")
	}

	return userID, nil
}

// GetUserByID 根据 ID 获取用户信息
func GetUserByID(userID string) (map[string]interface{}, error) {
	var id int64
	var email string
	var createdAt, updatedAt string

	err := DB.QueryRow("SELECT id, email, created_at, updated_at FROM users WHERE id = ?", userID).Scan(&id, &email, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, err
	}

	return map[string]interface{}{
		"id":         id,
		"email":      email,
		"created_at": createdAt,
		"updated_at": updatedAt,
	}, nil
}

// GetTasksPaginated 分页获取任务
func GetTasksPaginated(userID int, page, pageSize int) ([]map[string]interface{}, int, error) {
	offset := (page - 1) * pageSize

	// 获取总数
	var total int
	err := DB.QueryRow("SELECT COUNT(*) FROM tasks WHERE user_id = ? AND is_deleted = 0", userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// 获取分页任务
	query := `
		SELECT id, local_id, server_version, title, description, status, priority,
		       due_at, created_at, updated_at, completed_at, is_deleted, last_modified
		FROM tasks
		WHERE user_id = ? AND is_deleted = 0
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := DB.Query(query, userID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int64
		var localID sql.NullString
		var serverVersion sql.NullInt64
		var title sql.NullString
		var description sql.NullString
		var status sql.NullString
		var priority sql.NullString
		var dueAt sql.NullString
		var createdAt sql.NullString
		var updatedAt sql.NullString
		var completedAt sql.NullString
		var isDeleted sql.NullBool
		var lastModified sql.NullString
		if err := rows.Scan(&id, &localID, &serverVersion, &title, &description, &status, &priority, &dueAt, &createdAt, &updatedAt, &completedAt, &isDeleted, &lastModified); err != nil {
			return nil, 0, err
		}
		row := map[string]interface{}{
			"id":       id,
			"local_id": localID.String,
			"server_version": func() interface{} {
				if serverVersion.Valid {
					return serverVersion.Int64
				}
				return nil
			}(),
			"title":         title.String,
			"description":   description.String,
			"status":        status.String,
			"priority":      priority.String,
			"due_at":        dueAt.String,
			"created_at":    createdAt.String,
			"updated_at":    updatedAt.String,
			"completed_at":  completedAt.String,
			"is_deleted":    isDeleted.Bool,
			"last_modified": lastModified.String,
		}
		results = append(results, row)
	}
	return results, total, nil
}

// CreateTask 创建新任务
func CreateTask(userID int, localID, title string) (int64, error) {
	now := time.Now().UTC()
	result, err := DB.Exec(`
		INSERT INTO tasks (user_id, local_id, server_version, title, status, created_at, updated_at, last_modified)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, localID, 1, title, "todo", now, now, now)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateTask 更新任务
func UpdateTask(taskID int64, title, status string) error {
	now := time.Now().UTC()
	_, err := DB.Exec(`
		UPDATE tasks
		SET title = ?, status = ?, updated_at = ?, last_modified = ?
		WHERE id = ?
	`, title, status, now, now, taskID)
	return err
}

// DeleteTask 删除任务（软删除）
func DeleteTask(taskID int64) error {
	now := time.Now().UTC()
	_, err := DB.Exec(`
		UPDATE tasks
		SET is_deleted = 1, updated_at = ?, last_modified = ?
		WHERE id = ?
	`, now, now, taskID)
	return err
}

// CleanupExpiredTokens 清理过期的令牌
func CleanupExpiredTokens() error {
	_, err := DB.Exec("DELETE FROM tokens WHERE expires_at < ?", time.Now().UTC())
	return err
}

// CleanupOldLoginLogs 清理旧的登录日志（保留 30 天）
func CleanupOldLoginLogs() error {
	thirtyDaysAgo := time.Now().UTC().AddDate(0, 0, -30)
	_, err := DB.Exec("DELETE FROM login_logs WHERE timestamp < ?", thirtyDaysAgo)
	return err
}
