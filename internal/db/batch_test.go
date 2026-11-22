package db

import "testing"

func createTaskForBatch(t *testing.T, userID int, localID, title string) int64 {
	t.Helper()
	id, err := CreateTask(userID, localID, title)
	if err != nil {
		t.Fatalf("创建任务 %s 失败: %v", title, err)
	}
	return id
}

func countRows(t *testing.T, query string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := DB.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("统计查询失败 %q: %v", query, err)
	}
	return n
}

func TestBatchDeleteTasksOptimizedBasic(t *testing.T) {
	setupTestDB(t)
	uid := int(createTestUser(t, "batch@test.com", "user"))
	id1 := createTaskForBatch(t, uid, "b-1", "批量任务1")
	id2 := createTaskForBatch(t, uid, "b-2", "批量任务2")
	id3 := createTaskForBatch(t, uid, "b-3", "批量任务3")

	deleted, err := BatchDeleteTasksOptimized(uid, []int64{id1, id2})
	if err != nil {
		t.Fatalf("批量删除失败: %v", err)
	}
	if deleted != 2 {
		t.Errorf("应删除 2 条, got %d", deleted)
	}

	if remain := countRows(t, "SELECT COUNT(*) FROM tasks WHERE user_id = ? AND is_deleted = 0", uid); remain != 1 {
		t.Errorf("剩余未删除任务应为 1, got %d", remain)
	}
	if records := countRows(t, "SELECT COUNT(*) FROM deleted_tasks WHERE user_id = ?", uid); records != 2 {
		t.Errorf("deleted_tasks 回收站记录应为 2, got %d", records)
	}

	// 恢复 id3 未被误删
	var title string
	if err := DB.QueryRow("SELECT title FROM tasks WHERE id = ?", id3).Scan(&title); err != nil {
		t.Fatalf("未删除任务应仍可查询: %v", err)
	}
	if title != "批量任务3" {
		t.Errorf("未删除任务标题不符: %q", title)
	}
}

func TestBatchDeleteTasksOptimizedOwnership(t *testing.T) {
	setupTestDB(t)
	userA := int(createTestUser(t, "ba@test.com", "user"))
	userB := int(createTestUser(t, "bb@test.com", "user"))
	idB := createTaskForBatch(t, userB, "own-1", "B的任务")

	// A 不能删除 B 的任务
	deleted, err := BatchDeleteTasksOptimized(userA, []int64{idB})
	if err != nil {
		t.Fatalf("跨用户批量删除不应报错, got %v", err)
	}
	if deleted != 0 {
		t.Errorf("跨用户删除应跳过, got %d", deleted)
	}
	if remain := countRows(t, "SELECT COUNT(*) FROM tasks WHERE id = ? AND is_deleted = 0", idB); remain != 1 {
		t.Errorf("B 的任务不应被 A 删除")
	}
}

func TestBatchDeleteTasksOptimizedEmpty(t *testing.T) {
	setupTestDB(t)
	uid := int(createTestUser(t, "be@test.com", "user"))
	deleted, err := BatchDeleteTasksOptimized(uid, nil)
	if err != nil {
		t.Fatalf("空列表批量删除不应报错, got %v", err)
	}
	if deleted != 0 {
		t.Errorf("空列表应删除 0 条, got %d", deleted)
	}
}

func TestBatchDeleteTasksAliasMatches(t *testing.T) {
	setupTestDB(t)
	uid := int(createTestUser(t, "bf@test.com", "user"))
	id1 := createTaskForBatch(t, uid, "alias-1", "别名任务1")

	deleted, err := BatchDeleteTasks(uid, []int64{id1})
	if err != nil {
		t.Fatalf("BatchDeleteTasks 别名调用失败: %v", err)
	}
	if deleted != 1 {
		t.Errorf("别名调用应删除 1 条, got %d", deleted)
	}
}
