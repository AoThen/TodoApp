package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

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
		_, _ = DB.Exec("UPDATE deleted_tasks SET is_restorable=0 WHERE task_id=? AND user_id=?", taskID, userID)
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
