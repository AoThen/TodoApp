package db

import (
	"database/sql"
	"time"
)

// DBTX 事务与连接的公共接口：*sql.DB 与 *sql.Tx 均满足。
// 同步路径通过它把操作绑定到调用方事务，避免"事务内走全局连接"的假事务问题。
type DBTX interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// GetTaskForSync 获取任务用于同步检查（按 user_id 校验归属，防止越权访问他人任务）
func GetTaskForSync(q DBTX, userID int, taskID int64) (int, string, string, string, bool, error) {
	var serverVersion int
	var title, description, status string
	var isDeleted bool

	err := q.QueryRow(
		"SELECT server_version, title, description, status, is_deleted FROM tasks WHERE id = ? AND user_id = ?",
		taskID, userID,
	).Scan(&serverVersion, &title, &description, &status, &isDeleted)

	return serverVersion, title, description, status, isDeleted, err
}

// TaskExistsByLocalID 检查local_id是否存在（限定当前用户）
func TaskExistsByLocalID(q DBTX, userID int, localID string) (int64, string, error) {
	var taskID int64
	var title string

	err := q.QueryRow(
		"SELECT id, title FROM tasks WHERE local_id = ? AND user_id = ?",
		localID, userID,
	).Scan(&taskID, &title)

	return taskID, title, err
}

// RecordConflict 记录冲突到数据库（带用户归属；事务提交后调用，走全局连接）
func RecordConflict(userID int, localID string, serverID int64, reason string, optionsJSON string) error {
	_, err := DB.Exec(
		"INSERT INTO conflicts (user_id, local_id, server_id, reason, options, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		userID, localID, serverID, reason, optionsJSON, time.Now().UTC(),
	)
	return err
}

// TaskExtraFields 同步更新中"显式携带才更新"的扩展字段；
// 指针为 nil 表示本次不更新该字段（保持服务端原值）
type TaskExtraFields struct {
	SortOrder          *float64
	RecurrenceType     *string
	RecurrenceSeriesID *string
}

// UpdateTaskWithVersion 更新任务并增加版本号（校验归属；q 传事务以纳入原子提交）
// dueAt 非 nil 时一并更新 due_at（空串表示清除）；nil 表示保持原值
// extra 非 nil 时按指针是否为 nil 决定是否更新手动排序/循环任务字段
func UpdateTaskWithVersion(q DBTX, userID int, taskID int64, title, description, status, priority string, dueAt *string, newVer int, extra *TaskExtraFields) error {
	now := time.Now().UTC()
	query := "UPDATE tasks SET title=?, description=?, status=?, priority=?, server_version=?, updated_at=?, last_modified=?"
	args := []interface{}{title, description, status, priority, newVer, now, now}
	if dueAt != nil {
		query += ", due_at=?"
		if *dueAt == "" {
			args = append(args, nil)
		} else {
			args = append(args, *dueAt)
		}
	}
	if extra != nil {
		if extra.SortOrder != nil {
			query += ", sort_order=?"
			args = append(args, *extra.SortOrder)
		}
		if extra.RecurrenceType != nil {
			query += ", recurrence_type=?"
			args = append(args, *extra.RecurrenceType)
		}
		if extra.RecurrenceSeriesID != nil {
			query += ", recurrence_series_id=?"
			args = append(args, *extra.RecurrenceSeriesID)
		}
	}
	query += " WHERE id=? AND user_id=?"
	args = append(args, taskID, userID)

	_, err := q.Exec(query, args...)
	return err
}

// SoftDeleteTaskWithVersion 软删除任务并增加版本号（校验归属；q 传事务以纳入原子提交）
func SoftDeleteTaskWithVersion(q DBTX, userID int, taskID int64, newVer int) error {
	now := time.Now().UTC()
	_, err := q.Exec(
		"UPDATE tasks SET is_deleted=1, server_version=?, updated_at=?, last_modified=? WHERE id=? AND user_id=?",
		newVer, now, now, taskID, userID,
	)
	return err
}
