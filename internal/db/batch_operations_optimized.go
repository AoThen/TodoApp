package db

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// BatchDeleteTasksOptimized 批量软删除任务（优化版本）
func BatchDeleteTasksOptimized(userID int, taskIDs []int64) (int, error) {
	if len(taskIDs) == 0 {
		return 0, nil
	}

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

	now := time.Now().UTC()

	// 使用 IN 子句批量获取任务数据
	placeholders := make([]string, len(taskIDs))
	args := make([]interface{}, 0, len(taskIDs)+1) // +1 for userID
	args = append(args, userID)

	for i, taskID := range taskIDs {
		placeholders[i] = "?"
		args = append(args, taskID)
	}

	query := fmt.Sprintf(
		"SELECT id, local_id, server_version, title, description, status, priority, due_at, created_at FROM tasks WHERE user_id = ? AND id IN (%s) AND is_deleted = 0",
		strings.Join(placeholders, ","),
	)

	rows, err := tx.Query(query, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	// 收集需要删除的任务
	type taskData struct {
		id            int64
		localID       string
		serverVersion int
		title         string
		description   string
		status        string
		priority      string
		dueAt         string
		createdAt     string
	}
	var tasksToProcess []taskData
	for rows.Next() {
		var t taskData
		err := rows.Scan(&t.id, &t.localID, &t.serverVersion, &t.title, &t.description, &t.status, &t.priority, &t.dueAt, &t.createdAt)
		if err != nil {
			return 0, err
		}
		tasksToProcess = append(tasksToProcess, t)
	}

	if len(tasksToProcess) == 0 {
		return 0, nil
	}

	// 批量软删除任务
	deletePlaceholders := make([]string, len(tasksToProcess))
	deleteArgs := make([]interface{}, 0, len(tasksToProcess))

	for i, t := range tasksToProcess {
		deletePlaceholders[i] = "?"
		deleteArgs = append(deleteArgs, t.id)
	}

	deleteQuery := fmt.Sprintf("UPDATE tasks SET is_deleted=1, updated_at=?, last_modified=? WHERE id IN (%s)",
		strings.Join(deletePlaceholders, ","))
	updateArgs := make([]interface{}, 0, len(deleteArgs)+2)
	updateArgs = append(updateArgs, now, now)
	updateArgs = append(updateArgs, deleteArgs...)

	_, err = tx.Exec(deleteQuery, updateArgs...)
	if err != nil {
		return 0, err
	}

	// 批量插入删除记录（单条 SQL）
	insertPlaceholders := make([]string, len(tasksToProcess))
	insertArgs := make([]interface{}, 0, len(tasksToProcess)*4)
	for i, t := range tasksToProcess {
		insertPlaceholders[i] = "(?, ?, ?, ?)"
		taskMap := map[string]interface{}{
			"id":             t.id,
			"local_id":       t.localID,
			"server_version": t.serverVersion,
			"title":          t.title,
			"description":    t.description,
			"status":         t.status,
			"priority":       t.priority,
			"due_at":         t.dueAt,
			"created_at":     t.createdAt,
		}
		taskJSON, _ := json.Marshal(taskMap)
		insertArgs = append(insertArgs, t.id, userID, string(taskJSON), now)
	}

	insertQuery := fmt.Sprintf("INSERT INTO deleted_tasks (task_id, user_id, task_data, deleted_at) VALUES %s",
		strings.Join(insertPlaceholders, ","))
	_, err = tx.Exec(insertQuery, insertArgs...)
	if err != nil {
		return 0, err
	}

	return len(tasksToProcess), nil
}

// BatchDeleteTasks 批量软删除任务（使用优化版本）
func BatchDeleteTasks(userID int, taskIDs []int64) (int, error) {
	return BatchDeleteTasksOptimized(userID, taskIDs)
}
