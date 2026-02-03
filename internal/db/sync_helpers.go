package db

import (
	"time"
)

// GetTaskForSync 获取任务用于同步检查
func GetTaskForSync(taskID int64) (int, string, string, string, bool, error) {
	var serverVersion int
	var title, description, status string
	var isDeleted bool

	err := DB.QueryRow(
		"SELECT server_version, title, description, status, is_deleted FROM tasks WHERE id = ?",
		taskID,
	).Scan(&serverVersion, &title, &description, &status, &isDeleted)

	return serverVersion, title, description, status, isDeleted, err
}

// TaskExistsByLocalID 检查local_id是否存在
func TaskExistsByLocalID(userID int, localID string) (int64, string, error) {
	var taskID int64
	var title string

	err := DB.QueryRow(
		"SELECT id, title FROM tasks WHERE local_id = ? AND user_id = ?",
		localID, userID,
	).Scan(&taskID, &title)

	return taskID, title, err
}

// RecordConflict 记录冲突到数据库
func RecordConflict(localID string, serverID int64, reason string, optionsJSON string) error {
	_, err := DB.Exec(
		"INSERT INTO conflicts (local_id, server_id, reason, options, created_at) VALUES (?, ?, ?, ?, ?)",
		localID, serverID, reason, optionsJSON, time.Now().UTC(),
	)
	return err
}

// UpdateTaskWithVersion 更新任务并增加版本号
func UpdateTaskWithVersion(taskID int64, title, description, status, priority string, newVer int) error {
	now := time.Now().UTC()
	_, err := DB.Exec(
		"UPDATE tasks SET title=?, description=?, status=?, priority=?, server_version=?, updated_at=?, last_modified=? WHERE id=?",
		title, description, status, priority, newVer, now, now, taskID,
	)
	return err
}

// SoftDeleteTaskWithVersion 软删除任务并增加版本号
func SoftDeleteTaskWithVersion(taskID int64, newVer int) error {
	now := time.Now().UTC()
	_, err := DB.Exec(
		"UPDATE tasks SET is_deleted=1, server_version=?, updated_at=?, last_modified=? WHERE id=?",
		newVer, now, now, taskID,
	)
	return err
}
