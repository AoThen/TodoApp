package db

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// generateImportLocalID 为导入任务生成唯一 local_id
func generateImportLocalID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("import-%d", time.Now().UnixNano())
	}
	return "import-" + hex.EncodeToString(b)
}

// BatchInsertTasks 批量插入任务（用于导入）
func BatchInsertTasks(userID int, tasks []map[string]interface{}) ([]int64, error) {
	tx, err := DB.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
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

		// 导入数据可能不带 local_id，自动生成以保证幂等同步语义
		if localID == "" {
			localID = generateImportLocalID()
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
