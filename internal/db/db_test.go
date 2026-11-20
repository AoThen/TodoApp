package db

import (
	"strings"
	"testing"
)

// 注意：db 测试使用内存 SQLite（:memory:），不依赖外部环境变量。

// setupTestDB 使用临时内存数据库初始化
func setupTestDB(t *testing.T) {
	t.Helper()
	if err := InitDB(":memory:"); err != nil {
		t.Fatalf("初始化测试数据库失败: %v", err)
	}
	// 关闭并清空，避免 seed 影响
	DB.Close()
	if err := InitDB(":memory:"); err != nil {
		t.Fatalf("重新初始化测试数据库失败: %v", err)
	}
}

func createTestUser(t *testing.T, email, role string) int64 {
	t.Helper()
	id, err := CreateUser(email, "StrongP@ss123", role)
	if err != nil {
		t.Fatalf("创建测试用户 %s 失败: %v", email, err)
	}
	return id
}

// TestUpdateTaskFullOwnership 验证用户不能更新其他用户的任务（越权防护）
func TestUpdateTaskFullOwnership(t *testing.T) {
	setupTestDB(t)

	userA := createTestUser(t, "a@test.com", "user")
	userB := createTestUser(t, "b@test.com", "user")

	// 用户A创建任务
	taskID, err := CreateTask(int(userA), "local-a-1", "任务A")
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}

	// 用户A更新自己的任务：应成功
	if err := UpdateTaskFull(taskID, int(userA), "任务A-更新", "", "todo", "high", "", false); err != nil {
		t.Errorf("用户A更新自己的任务应成功，got: %v", err)
	}

	// 用户B尝试更新用户A的任务：应失败（越权防护）
	if err := UpdateTaskFull(taskID, int(userB), "恶意修改", "", "done", "low", "", false); err == nil {
		t.Error("用户B更新用户A的任务应被拒绝，但未报错")
	}

	// 验证任务仍归用户A所有且未被修改
	task, err := GetTaskByID(taskID, int(userA))
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}
	if task["title"] != "任务A-更新" {
		t.Errorf("任务标题被非法修改，got %v", task["title"])
	}
}

// TestDeleteTaskOwnership 验证用户不能删除其他用户的任务（越权防护）
func TestDeleteTaskOwnership(t *testing.T) {
	setupTestDB(t)

	userA := createTestUser(t, "a@test.com", "user")
	userB := createTestUser(t, "b@test.com", "user")

	taskID, err := CreateTask(int(userA), "local-a-2", "任务A2")
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}

	// 用户B尝试删除用户A的任务：应失败（越权防护）
	if err := DeleteTask(taskID, int(userB)); err == nil {
		t.Error("用户B删除用户A的任务应被拒绝，但未报错")
	}

	// 用户A删除自己的任务：应成功
	if err := DeleteTask(taskID, int(userA)); err != nil {
		t.Errorf("用户A删除自己的任务应成功，got: %v", err)
	}

	// 验证任务已软删除
	task, err := GetTaskByID(taskID, int(userA))
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}
	if task["is_deleted"] != true {
		t.Errorf("任务应被软删除，got %v", task["is_deleted"])
	}
}

// TestDeleteTaskNonexistent 验证删除不存在或不属于用户的任务返回错误
func TestDeleteTaskNonexistent(t *testing.T) {
	setupTestDB(t)

	userA := createTestUser(t, "a@test.com", "user")

	// 删除不存在的任务ID：应返回错误
	if err := DeleteTask(99999, int(userA)); err == nil {
		t.Error("删除不存在的任务应返回错误")
	}

	// 删除不存在用户的任务：应返回错误
	if err := DeleteTask(1, 99999); err == nil {
		t.Error("删除不存在用户的任务应返回错误")
	}
}

// TestUpdateTaskFullDueAt 验证 due_at 的更新与清除语义
func TestUpdateTaskFullDueAt(t *testing.T) {
	setupTestDB(t)

	userID := createTestUser(t, "due@test.com", "user")
	taskID, err := CreateTask(int(userID), "local-due-1", "带截止任务")
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}

	// 不传 due_at（hasDueAt=false）：保持为空
	if err := UpdateTaskFull(taskID, int(userID), "改标题", "", "todo", "high", "", false); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	task, err := GetTaskByID(taskID, int(userID))
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}
	if task["due_at"] != "" {
		t.Errorf("未传 due_at 时应保持为空, got %v", task["due_at"])
	}

	// 传 due_at：设置截止日期
	if err := UpdateTaskFull(taskID, int(userID), "", "", "", "", "2026-12-01", true); err != nil {
		t.Fatalf("设置 due_at 失败: %v", err)
	}
	task, err = GetTaskByID(taskID, int(userID))
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}
	// due_at 列声明为 DATETIME，go-sqlite3 读出时会把纯日期规范化为 RFC3339（2026-12-01T00:00:00Z），语义等价
	dueAtStr, _ := task["due_at"].(string)
	if !strings.HasPrefix(dueAtStr, "2026-12-01") {
		t.Errorf("due_at 应为 2026-12-01, got %v", task["due_at"])
	}

	// 传空串（hasDueAt=true）：清除截止日期
	if err := UpdateTaskFull(taskID, int(userID), "", "", "", "", "", true); err != nil {
		t.Fatalf("清除 due_at 失败: %v", err)
	}
	task, err = GetTaskByID(taskID, int(userID))
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}
	if task["due_at"] != "" {
		t.Errorf("空串应清除 due_at, got %v", task["due_at"])
	}
}
