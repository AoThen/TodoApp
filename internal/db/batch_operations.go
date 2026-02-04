package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// BatchDeleteTasks 批量软删除任务
func BatchDeleteTasks(userID int, taskIDs []int64) (int, error) {
	tx, err := DB.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	count := 0
	now := time.Now().UTC()

	for _, taskID := range taskIDs {
		// 1. 获取任务数据用于恢复
		var localID string
		var serverVersion int
		var title, description, status, priority, dueAt string
		var createdAt string

		err := tx.QueryRow(
			"SELECT local_id, server_version, title, description, status, priority, due_at, created_at FROM tasks WHERE id = ? AND user_id = ? AND is_deleted = 0",
			taskID, userID,
		).Scan(&localID, &serverVersion, &title, &description, &status, &priority, &dueAt, &createdAt)

		if err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return 0, err
		}

		// 构建任务数据JSON
		taskMap := map[string]interface{}{
			"id":             taskID,
			"local_id":       localID,
			"server_version": serverVersion,
			"title":          title,
			"description":    description,
			"status":         status,
			"priority":       priority,
			"due_at":         dueAt,
			"created_at":     createdAt,
		}
		taskJSON, _ := json.Marshal(taskMap)

		// 2. 软删除任务
		_, err = tx.Exec("UPDATE tasks SET is_deleted=1, updated_at=?, last_modified=? WHERE id=?",
			now, now, taskID)
		if err != nil {
			return 0, err
		}

		// 3. 保存删除记录（用于撤销）
		_, err = tx.Exec("INSERT INTO deleted_tasks (task_id, user_id, task_data, deleted_at) VALUES (?, ?, ?, ?)",
			taskID, userID, string(taskJSON), now)
		if err != nil {
			return 0, err
		}

		count++
	}

	return count, nil
}

// RestoreDeletedTask 恢复删除的任务（撤销删除）
func RestoreDeletedTask(taskID int64, userID int) error {
	// 1. 查找删除记录
	var taskJSON string
	var deletedAt time.Time

	err := DB.QueryRow(
		"SELECT task_data, deleted_at FROM deleted_tasks WHERE task_id = ? AND user_id = ? AND is_restorable=1",
		taskID, userID,
	).Scan(&taskJSON, &deletedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		return err
	}

	// 2. 检查30秒期限
	if time.Since(deletedAt) > 30*time.Second {
		// 标记为不可恢复
		DB.Exec("UPDATE deleted_tasks SET is_restorable=0 WHERE task_id=? AND user_id=?", taskID, userID)
		return &UndoExpiredError{}
	}

	// 3. 解析任务数据
	var taskData map[string]interface{}
	err = json.Unmarshal([]byte(taskJSON), &taskData)
	if err != nil {
		return err
	}

	// 4. 恢复任务
	now := time.Now().UTC()
	title := taskJSONValue(taskData["title"])
	description := taskJSONValue(taskData["description"])
	status := taskJSONValue(taskData["status"])
	priority := taskJSONValue(taskData["priority"])

	_, err = DB.Exec(
		"UPDATE tasks SET is_deleted=0, title=?, description=?, status=?, priority=?, updated_at=?, last_modified=? WHERE id=?",
		title, description, status, priority, now, now, taskID,
	)
	if err != nil {
		return err
	}

	// 5. 标记为已恢复
	_, err = DB.Exec("UPDATE deleted_tasks SET is_restorable=0 WHERE task_id=? AND user_id=?", taskID, userID)
	return err
}

// GetRestorableTask 获取可恢复的任务
func GetRestorableTask(taskID int64, userID int) (map[string]interface{}, error) {
	var taskData string
	var deletedAt time.Time

	err := DB.QueryRow(
		"SELECT task_data, deleted_at FROM deleted_tasks WHERE task_id = ? AND user_id = ? AND is_restorable=1",
		taskID, userID,
	).Scan(&taskData, &deletedAt)

	if err != nil {
		return nil, err
	}

	// 检查30秒期限
	if time.Since(deletedAt) > 30*time.Second {
		return nil, &UndoExpiredError{}
	}

	// 解析任务数据
	var taskMap map[string]interface{}
	err = json.Unmarshal([]byte(taskData), &taskMap)
	if err != nil {
		return nil, err
	}

	taskMap["deleted_at"] = deletedAt.Format(time.RFC3339)
	taskMap["seconds_until_expiry"] = 30 - int(time.Since(deletedAt).Seconds())

	return taskMap, nil
}

// CleanupOldDeletedTasks 清理旧删除记录（超过30天）
func CleanupOldDeletedTasks() (int, error) {
	threshold := time.Now().AddDate(0, 0, -30).UTC()

	result, err := DB.Exec(
		"DELETE FROM deleted_tasks WHERE deleted_at < ?",
		threshold,
	)
	if err != nil {
		return 0, err
	}

	count, _ := result.RowsAffected()
	return int(count), nil
}

// taskJSONValue 从interface安全地转换为字符串
func taskJSONValue(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%f", val)
	case int64:
		return fmt.Sprintf("%d", val)
	default:
		return ""
	}
}

// UndoExpiredError 30秒撤销期限错误
type UndoExpiredError struct{}

func (e *UndoExpiredError) Error() string {
	return "已超过30秒撤销期限"
}
