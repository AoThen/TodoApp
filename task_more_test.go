package main

import (
	"fmt"
	"net/http"
	"testing"
)

func TestRestoreTaskFlow(t *testing.T) {
	cleanTables(t)
	task := createTaskViaAPI(t, userTok, "待恢复任务")
	taskID := task["id"].(float64)

	// 删除（软删除）
	resp, _ := doJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%v", taskID), userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("删除任务应 200, got %d", resp.StatusCode)
	}
	_, out := doJSON(t, http.MethodGet, "/api/v1/tasks", userTok, nil)
	if tasks, _ := dataMap(out)["tasks"].([]interface{}); len(tasks) != 0 {
		t.Fatalf("删除后列表应为空: %v", out)
	}

	// 恢复
	resp, out = doJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%v/restore", taskID), userTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("恢复任务应 200, got %d %v", resp.StatusCode, out)
	}
	_, out = doJSON(t, http.MethodGet, "/api/v1/tasks", userTok, nil)
	tasks, _ := dataMap(out)["tasks"].([]interface{})
	if len(tasks) != 1 {
		t.Fatalf("恢复后任务应可见: %v", out)
	}
	if got := tasks[0].(map[string]interface{})["title"]; got != "待恢复任务" {
		t.Errorf("恢复后标题应一致, got %v", got)
	}
}

func TestRestoreNonexistentTask(t *testing.T) {
	cleanTables(t)
	resp, _ := doJSON(t, http.MethodPost, "/api/v1/tasks/999999/restore", userTok, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("恢复不存在任务应 404, got %d", resp.StatusCode)
	}
}

func TestBatchDeletePartialMissing(t *testing.T) {
	cleanTables(t)
	t1 := createTaskViaAPI(t, userTok, "批量删除1")
	t2 := createTaskViaAPI(t, userTok, "批量删除2")
	id1 := t1["id"].(float64)
	id2 := t2["id"].(float64)

	// 一个存在 + 一个不存在：存在的应被删除，整体不报错
	resp, out := doJSON(t, http.MethodDelete, "/api/v1/tasks/batch", userTok, map[string]interface{}{
		"task_ids": []float64{id1, 999999},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("部分缺失的批量删除应 200, got %d %v", resp.StatusCode, out)
	}

	_, out = doJSON(t, http.MethodGet, "/api/v1/tasks", userTok, nil)
	tasks, _ := dataMap(out)["tasks"].([]interface{})
	if len(tasks) != 1 {
		t.Fatalf("应只剩 1 个任务: %v", out)
	}
	if got := tasks[0].(map[string]interface{})["id"].(float64); got != id2 {
		t.Errorf("剩余任务应为 id2, got %v", tasks[0])
	}
}

func TestTasksPaginationBoundaries(t *testing.T) {
	cleanTables(t)
	for i := 1; i <= 3; i++ {
		createTaskViaAPI(t, userTok, fmt.Sprintf("分页任务%d", i))
	}

	// 正常分页
	_, out := doJSON(t, http.MethodGet, "/api/v1/tasks?page=1&page_size=2", userTok, nil)
	data := dataMap(out)
	tasks, _ := data["tasks"].([]interface{})
	if len(tasks) != 2 {
		t.Errorf("page_size=2 应返回 2 条, got %d", len(tasks))
	}
	pg, _ := data["pagination"].(map[string]interface{})
	if pg["total"].(float64) != 3 || pg["pages"].(float64) != 2 {
		t.Errorf("分页元数据应为 total=3 pages=2, got %v", pg)
	}

	// 非法 page/page_size 应回退默认值而非报错
	_, out = doJSON(t, http.MethodGet, "/api/v1/tasks?page=-1&page_size=999", userTok, nil)
	if tasks, _ = dataMap(out)["tasks"].([]interface{}); len(tasks) != 3 {
		t.Errorf("非法分页参数应回退默认并返回全部 3 条, got %d", len(tasks))
	}

	// 越界页
	_, out = doJSON(t, http.MethodGet, "/api/v1/tasks?page=99&page_size=2", userTok, nil)
	if tasks, _ = dataMap(out)["tasks"].([]interface{}); len(tasks) != 0 {
		t.Errorf("越界页应返回空列表, got %d", len(tasks))
	}
}
