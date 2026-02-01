package db

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
	"log"
	"os"
	"strconv"
	"strings"
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
            role TEXT DEFAULT 'user',
            failed_attempts INTEGER DEFAULT 0,
            locked_until DATETIME,
            must_change_password BOOLEAN DEFAULT 0,
            created_at DATETIME,
            updated_at DATETIME
        );`,
		`CREATE TABLE IF NOT EXISTS admin_logs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            admin_id INTEGER,
            admin_email TEXT,
            action TEXT,
            target_user_id INTEGER,
            target_email TEXT,
            details TEXT,
            ip_address TEXT,
            timestamp DATETIME,
            FOREIGN KEY(admin_id) REFERENCES users(id)
        );`,
		`CREATE TABLE IF NOT EXISTS system_config (
            key TEXT PRIMARY KEY,
            value TEXT,
            description TEXT,
            updated_at DATETIME,
            updated_by INTEGER
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
		`CREATE TABLE IF NOT EXISTS notifications (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            type TEXT NOT NULL,
            title TEXT NOT NULL,
            content TEXT NOT NULL,
            priority TEXT DEFAULT 'normal',
            is_read BOOLEAN DEFAULT 0,
            read_at DATETIME,
            expires_at DATETIME,
            created_at DATETIME DEFAULT (datetime('now')),
            FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
        );`,
		`CREATE TABLE IF NOT EXISTS notification_templates (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            type TEXT UNIQUE NOT NULL,
            title_template TEXT NOT NULL,
            content_template TEXT NOT NULL,
            priority TEXT DEFAULT 'normal',
            enabled BOOLEAN DEFAULT 1,
            created_at DATETIME DEFAULT (datetime('now')),
            updated_at DATETIME DEFAULT (datetime('now'))
        );`,
		`CREATE TABLE IF NOT EXISTS notification_settings (
            user_id INTEGER PRIMARY KEY,
            notification_type TEXT NOT NULL,
            enabled BOOLEAN DEFAULT 1,
            auto_clear_days INTEGER DEFAULT 30,
            created_at DATETIME DEFAULT (datetime('now')),
            updated_at DATETIME DEFAULT (datetime('now')),
            FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
        );`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_read ON notifications(is_read);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_read ON notifications(user_id, is_read);`,
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

// seedIfEmpty creates a default admin user and a sample task if the tables are empty
func seedIfEmpty() {
	var count int
	row := DB.QueryRow("SELECT COUNT(*) FROM users")
	if err := row.Scan(&count); err != nil {
		log.Println("seed: failed to count users:", err)
		return
	}
	if count == 0 {
		// Get admin credentials from environment variables or use defaults
		adminEmail := os.Getenv("INITIAL_ADMIN_EMAIL")
		if adminEmail == "" {
			adminEmail = "admin@example.com"
		}
		adminPassword := os.Getenv("INITIAL_ADMIN_PASSWORD")
		if adminPassword == "" {
			adminPassword = "Admin123!"
		}

		password := []byte(adminPassword)
		hash, _ := bcrypt.GenerateFromPassword(password, 12)
		now := time.Now().UTC()
		res, err := DB.Exec("INSERT INTO users (email, password_hash, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			adminEmail, string(hash), "admin", now, now)
		if err != nil {
			log.Println("seed: failed to insert admin user:", err)
			return
		}
		userID, _ := res.LastInsertId()
		// sample task
		DB.Exec("INSERT INTO tasks (user_id, local_id, server_version, title, status, created_at, updated_at, last_modified) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", userID, "sample-1", 0, "Sample Task", "todo", now, now, now)

		// Initialize default system config
		initSystemConfig(now)

		log.Printf("seed: created admin user %s (password: %s)", adminEmail, adminPassword)
	}
}

// initSystemConfig initializes default system configuration
func initSystemConfig(now time.Time) {
	configs := []struct {
		key         string
		value       string
		description string
	}{
		{"max_login_attempts", "5", "Maximum failed login attempts before lockout"},
		{"login_attempt_window_minutes", "15", "Time window in minutes for login attempt counting"},
		{"lockout_duration_minutes", "30", "Account lockout duration in minutes"},
		{"access_token_duration_minutes", "15", "Access token validity in minutes"},
		{"refresh_token_duration_days", "7", "Refresh token validity in days"},
		{"allow_public_registration", "false", "Allow public user registration"},
	}

	for _, cfg := range configs {
		DB.Exec("INSERT OR IGNORE INTO system_config (key, value, description, updated_at) VALUES (?, ?, ?, ?)",
			cfg.key, cfg.value, cfg.description, now)
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

// GetUserEmail 获取用户邮箱
func GetUserEmail(userID int64) (string, error) {
	var email string
	err := DB.QueryRow("SELECT email FROM users WHERE id = ?", userID).Scan(&email)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("用户不存在")
		}
		return "", err
	}
	return email, nil
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

// ============ Admin Management Functions ============

// GetUserRole 获取用户的角色
func GetUserRole(userID string) (string, error) {
	var role string
	err := DB.QueryRow("SELECT role FROM users WHERE id = ?", userID).Scan(&role)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("用户不存在")
		}
		return "", err
	}
	return role, nil
}

// GetAllUsers 获取所有用户列表
func GetAllUsers() ([]map[string]interface{}, error) {
	rows, err := DB.Query(`
		SELECT id, email, role, failed_attempts, locked_until, must_change_password, created_at, updated_at
		FROM users ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int64
		var email, role string
		var failedAttempts int
		var lockedUntil sql.NullTime
		var mustChangePassword bool
		var createdAt, updatedAt string

		if err := rows.Scan(&id, &email, &role, &failedAttempts, &lockedUntil, &mustChangePassword, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"id":              id,
			"email":           email,
			"role":            role,
			"failed_attempts": failedAttempts,
			"locked_until": func() interface{} {
				if lockedUntil.Valid {
					return lockedUntil.Time.Format(time.RFC3339)
				}
				return nil
			}(),
			"must_change_password": mustChangePassword,
			"created_at":           createdAt,
			"updated_at":           updatedAt,
			"is_locked":            lockedUntil.Valid && lockedUntil.Time.After(time.Now()),
		})
	}
	return results, nil
}

// CreateUser 创建新用户
func CreateUser(email, password, role string) (int64, error) {
	// 检查邮箱是否已存在
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", email).Scan(&count)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		return 0, fmt.Errorf("邮箱已被注册")
	}

	// 验证角色
	if role != "admin" && role != "user" {
		return 0, fmt.Errorf("无效的角色")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	result, err := DB.Exec(
		"INSERT INTO users (email, password_hash, role, must_change_password, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		email, string(hash), role, role == "admin", now, now,
	)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// UpdateUser 更新用户信息
func UpdateUser(userID int64, email, role string) error {
	now := time.Now().UTC()
	_, err := DB.Exec("UPDATE users SET email = ?, role = ?, updated_at = ? WHERE id = ?", email, role, now, userID)
	return err
}

// ResetUserPassword 重置用户密码
func ResetUserPassword(userID int64, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	_, err = DB.Exec("UPDATE users SET password_hash = ?, must_change_password = ?, updated_at = ? WHERE id = ?",
		string(hash), true, now, userID)
	return err
}

// LockUserAccount 锁定用户账户
func LockUserAccount(userID int64, durationMinutes int) error {
	_, err := DB.Exec(
		"UPDATE users SET locked_until = datetime('now', '+%d minutes'), updated_at = ? WHERE id = ?",
		durationMinutes, time.Now().UTC(), userID,
	)
	return err
}

// UnlockUserAccount 解锁用户账户
func UnlockUserAccount(userID int64) error {
	_, err := DB.Exec("UPDATE users SET locked_until = NULL, failed_attempts = 0, updated_at = ? WHERE id = ?",
		time.Now().UTC(), userID)
	return err
}

// DeleteUser 删除用户（软删除）
func DeleteUser(userID int64) error {
	// 检查是否为最后一个管理员
	var adminCount int
	err := DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&adminCount)
	if err != nil {
		return err
	}

	var isAdmin string
	err = DB.QueryRow("SELECT role FROM users WHERE id = ?", userID).Scan(&isAdmin)
	if err != nil {
		return err
	}

	if isAdmin == "admin" && adminCount <= 1 {
		return fmt.Errorf("无法删除最后一个管理员")
	}

	_, err = DB.Exec("DELETE FROM users WHERE id = ?", userID)
	return err
}

// LogAdminAction 记录管理员操作
func LogAdminAction(adminID int, adminEmail, action, targetEmail string, targetUserID int64, details, ipAddress string) error {
	_, err := DB.Exec(
		"INSERT INTO admin_logs (admin_id, admin_email, action, target_user_id, target_email, details, ip_address, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		adminID, adminEmail, action, targetUserID, targetEmail, details, ipAddress, time.Now().UTC(),
	)
	return err
}

// GetAdminLogs 获取管理员操作日志
func GetAdminLogs(filters map[string]string) ([]map[string]interface{}, error) {
	query := "SELECT * FROM admin_logs WHERE 1=1"
	args := []interface{}{}

	if email, ok := filters["email"]; ok && email != "" {
		query += " AND admin_email LIKE ?"
		args = append(args, "%"+email+"%")
	}

	if targetEmail, ok := filters["target_email"]; ok && targetEmail != "" {
		query += " AND target_email LIKE ?"
		args = append(args, "%"+targetEmail+"%")
	}

	if action, ok := filters["action"]; ok && action != "" {
		query += " AND action = ?"
		args = append(args, action)
	}

	if startTime, ok := filters["start_time"]; ok && startTime != "" {
		query += " AND timestamp >= ?"
		args = append(args, startTime)
	}

	if endTime, ok := filters["end_time"]; ok && endTime != "" {
		query += " AND timestamp <= ?"
		args = append(args, endTime)
	}

	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	pageSize, _ := strconv.Atoi(filters["page_size"])
	page, _ := strconv.Atoi(filters["page"])
	if pageSize <= 0 {
		pageSize = 50
	}
	if page <= 0 {
		page = 1
	}
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, adminID, targetUserID int
		var adminEmail, action, targetEmail, details, ipAddress, timestamp string
		if err := rows.Scan(&id, &adminID, &adminEmail, &action, &targetUserID, &targetEmail, &details, &ipAddress, &timestamp); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":             id,
			"admin_id":       adminID,
			"admin_email":    adminEmail,
			"action":         action,
			"target_user_id": targetUserID,
			"target_email":   targetEmail,
			"details":        details,
			"ip_address":     ipAddress,
			"timestamp":      timestamp,
		})
	}
	return results, nil
}

// GetLoginLogsWithFilters 获取登录日志（带过滤）
func GetLoginLogsWithFilters(filters map[string]string) ([]map[string]interface{}, error) {
	query := "SELECT * FROM login_logs WHERE 1=1"
	args := []interface{}{}

	if email, ok := filters["email"]; ok && email != "" {
		query += " AND email LIKE ?"
		args = append(args, "%"+email+"%")
	}

	if success, ok := filters["success"]; ok && success != "" {
		query += " AND success = ?"
		args = append(args, success == "true")
	}

	if startTime, ok := filters["start_time"]; ok && startTime != "" {
		query += " AND timestamp >= ?"
		args = append(args, startTime)
	}

	if endTime, ok := filters["end_time"]; ok && endTime != "" {
		query += " AND timestamp <= ?"
		args = append(args, endTime)
	}

	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	pageSize, _ := strconv.Atoi(filters["page_size"])
	page, _ := strconv.Atoi(filters["page"])
	if pageSize <= 0 {
		pageSize = 50
	}
	if page <= 0 {
		page = 1
	}
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var email, ipAddress, timestamp string
		var success bool
		if err := rows.Scan(&id, &email, &ipAddress, &success, &timestamp); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":         id,
			"email":      email,
			"ip_address": ipAddress,
			"success":    success,
			"timestamp":  timestamp,
		})
	}
	return results, nil
}

// GetSystemConfig 获取系统配置
func GetSystemConfig() (map[string]string, error) {
	rows, err := DB.Query("SELECT key, value, description FROM system_config")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	config := make(map[string]string)
	for rows.Next() {
		var key, value, description string
		if err := rows.Scan(&key, &value, &description); err != nil {
			return nil, err
		}
		config[key] = value
	}
	return config, nil
}

// SetSystemConfig 设置系统配置
func SetSystemConfig(key, value, description, updatedBy string) error {
	now := time.Now().UTC()
	_, err := DB.Exec(
		"INSERT OR REPLACE INTO system_config (key, value, description, updated_at, updated_by) VALUES (?, ?, ?, ?, ?)",
		key, value, description, now, updatedBy,
	)
	return err
}

// ============ Notification Management Functions ============

// CreateNotification 创建通知
func CreateNotification(userID int, ntype, title, content, priority string, expiresAt *time.Time) (int64, error) {
	now := time.Now().UTC()
	var expiresAtStr string
	if expiresAt != nil {
		expiresAtStr = expiresAt.Format("2006-01-02 15:04:05")
	}

	result, err := DB.Exec(
		`INSERT INTO notifications (user_id, type, title, content, priority, is_read, created_at, expires_at) 
		 VALUES (?, ?, ?, ?, ?, 0, ?, ?)`,
		userID, ntype, title, content, priority, now, expiresAtStr,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetNotificationsPaginated 分页获取通知
func GetNotificationsPaginated(userID int, page, pageSize int, filters map[string]string) ([]map[string]interface{}, int, error) {
	offset := (page - 1) * pageSize
	query := "SELECT count(*) FROM notifications WHERE user_id = ?"
	args := []interface{}{userID}

	// 应用过滤器
	if read, ok := filters["read"]; ok && read != "" {
		query += " AND is_read = ?"
		args = append(args, read == "true")
	}
	if ntype, ok := filters["type"]; ok && ntype != "" {
		query += " AND type = ?"
		args = append(args, ntype)
	}
	if priority, ok := filters["priority"]; ok && priority != "" {
		query += " AND priority = ?"
		args = append(args, priority)
	}

	var total int
	err := DB.QueryRow(query, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query = `SELECT id, user_id, type, title, content, priority, is_read, read_at, created_at, expires_at
	          FROM notifications WHERE user_id = ?`

	if read, ok := filters["read"]; ok && read != "" {
		query += " AND is_read = ?"
	}
	if ntype, ok := filters["type"]; ok && ntype != "" {
		query += " AND type = ?"
	}
	if priority, ok := filters["priority"]; ok && priority != "" {
		query += " AND priority = ?"
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int64
		var userIDInt int
		var ntype, title, content, priority string
		var isRead bool
		var readAt, createdAt sql.NullString
		var expiresAt sql.NullString

		if err := rows.Scan(&id, &userIDInt, &ntype, &title, &content, &priority, &isRead, &readAt, &createdAt, &expiresAt); err != nil {
			return nil, 0, err
		}

		results = append(results, map[string]interface{}{
			"id":       id,
			"user_id":  userIDInt,
			"type":     ntype,
			"title":    title,
			"content":  content,
			"priority": priority,
			"is_read":  isRead,
			"read_at": func() interface{} {
				if readAt.Valid {
					return readAt.String
				}
				return nil
			}(),
			"created_at": createdAt.String,
			"expires_at": func() interface{} {
				if expiresAt.Valid {
					return expiresAt.String
				}
				return nil
			}(),
		})
	}

	return results, total, nil
}

// MarkNotificationAsRead 标记通知为已读
func MarkNotificationAsRead(userID, notificationID int) error {
	now := time.Now().UTC()
	result, err := DB.Exec(
		"UPDATE notifications SET is_read = 1, read_at = ? WHERE id = ? AND user_id = ?",
		now, notificationID, userID,
	)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("通知不存在")
	}
	return nil
}

// MarkAllNotificationsAsRead 标记所有通知为已读
func MarkAllNotificationsAsRead(userID int) (int64, error) {
	now := time.Now().UTC()
	result, err := DB.Exec(
		"UPDATE notifications SET is_read = 1, read_at = ? WHERE user_id = ? AND is_read = 0",
		now, userID,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetUnreadNotificationsCount 获取未读通知数量
func GetUnreadNotificationsCount(userID int, ntypes []string) (int, error) {
	query := "SELECT COUNT(*) FROM notifications WHERE user_id = ? AND is_read = 0"
	args := []interface{}{userID}

	if len(ntypes) > 0 {
		placeholders := make([]string, len(ntypes))
		for i := range ntypes {
			placeholders[i] = "?"
			args = append(args, ntypes[i])
		}
		query += " AND type IN (" + strings.Join(placeholders, ",") + ")"
	}

	var count int
	err := DB.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// DeleteNotification 删除通知
func DeleteNotification(userID, notificationID int) error {
	result, err := DB.Exec("DELETE FROM notifications WHERE id = ? AND user_id = ?", notificationID, userID)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("通知不存在")
	}
	return nil
}

// ClearNotifications 清空通知
func ClearNotifications(userID int, olderThanDays int) (int64, error) {
	var condition string
	var args []interface{}

	if olderThanDays > 0 {
		cutoffDate := time.Now().UTC().AddDate(0, 0, -olderThanDays)
		condition = " AND created_at < ?"
		args = []interface{}{userID, cutoffDate.Format("2006-01-02 15:04:05")}
	} else {
		args = []interface{}{userID}
	}

	query := "DELETE FROM notifications WHERE user_id = ?" + condition
	result, err := DB.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CleanupExpiredNotifications 清理过期通知
func CleanupExpiredNotifications() (int64, error) {
	result, err := DB.Exec(
		"DELETE FROM notifications WHERE expires_at IS NOT NULL AND expires_at < ?",
		time.Now().UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetNotificationByID 根据ID获取通知
func GetNotificationByID(userID, notificationID int) (map[string]interface{}, error) {
	var id int64
	var userIDInt int
	var ntype, title, content, priority string
	var isRead bool
	var readAt, createdAt sql.NullString
	var expiresAt sql.NullString

	err := DB.QueryRow(
		`SELECT id, user_id, type, title, content, priority, is_read, read_at, created_at, expires_at
		 FROM notifications WHERE id = ? AND user_id = ?`,
		notificationID, userID,
	).Scan(&id, &userIDInt, &ntype, &title, &content, &priority, &isRead, &readAt, &createdAt, &expiresAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("通知不存在")
		}
		return nil, err
	}

	return map[string]interface{}{
		"id":       id,
		"user_id":  userIDInt,
		"type":     ntype,
		"title":    title,
		"content":  content,
		"priority": priority,
		"is_read":  isRead,
		"read_at": func() interface{} {
			if readAt.Valid {
				return readAt.String
			}
			return nil
		}(),
		"created_at": createdAt.String,
		"expires_at": func() interface{} {
			if expiresAt.Valid {
				return expiresAt.String
			}
			return nil
		}(),
	}, nil
}
