package db

import (
	"fmt"
	"time"
)

// BatchInsertTasks 批量插入任务（用于导入）
func BatchInsertTasks(userID int, tasks []map[string]interface{}) ([]int64, error) {
	tx, err := DB.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	var insertedIDs []int64
	now := time.Now().UTC()

	for _, taskData := range tasks {
		localID := taskValue(taskData["local_id"])
		title := taskValue(taskData["title"])
		description := taskValue(taskData["description"])
		status := taskValue(taskData["status"])
		priority := taskValue(taskData["priority"])

		if localID == "" {
			return nil, fmt.Errorf("title不能为空")
		}

		if title == "" {
			continue
		}

		if status == "" {
			status = "todo"
		}

		if priority == "" {
			priority = "medium"
		}

		result, err := tx.Exec(
			"INSERT INTO tasks (user_id, local_id, server_version, title, description, status, priority, created_at, updated_at, last_modified) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			userID, localID, 1, title, description, status, priority, now, now, now,
		)

		if err != nil {
			continue
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, err
		}

		insertedIDs = append(insertedIDs, id)
	}

	return insertedIDs, nil
}

// CheckConflictByLocalID 检查local_id是否冲突
func CheckConflictByLocalID(userID int, localID string) (int64, string, error) {
	var existingID int64
	var existingTitle string

	err := DB.QueryRow(
		"SELECT id, title FROM tasks WHERE local_id = ? AND user_id = ?",
		localID, userID,
	).Scan(&existingID, &existingTitle)

	if err != nil {
		return 0, "", err
	}

	return existingID, existingTitle, nil
}

// taskValue 从interface安全地转换为字符串
func taskValue(v interface{}) string {
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
	case int:
		return fmt.Sprintf("%d", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}
