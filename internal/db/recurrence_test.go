package db

import (
	"strings"
	"testing"
	"time"
)

func strPtr(s string) *string { return &s }

func f64Ptr(f float64) *float64 { return &f }

// createRecurringTask 创建一个带循环规则的任务，返回任务 ID
func createRecurringTask(t *testing.T, userID int, localID, title, rule, dueAt string) int64 {
	t.Helper()
	taskID, err := CreateTask(userID, localID, title)
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	extra := &TaskExtraFields{RecurrenceType: strPtr(rule)}
	if err := UpdateTaskFull(taskID, userID, "", "", "", "", dueAt, true, extra); err != nil {
		t.Fatalf("设置循环规则失败: %v", err)
	}
	return taskID
}

// completeTask 将任务标记为完成（模拟勾选完成）
func completeTask(t *testing.T, userID int, taskID int64) {
	t.Helper()
	if err := UpdateTaskFull(taskID, userID, "", "", "done", "", "", false, nil); err != nil {
		t.Fatalf("完成任务失败: %v", err)
	}
}

func countTasksByLocalIDPrefix(t *testing.T, userID int, prefix string) int {
	t.Helper()
	var count int
	if err := DB.QueryRow(
		"SELECT COUNT(*) FROM tasks WHERE user_id = ? AND local_id LIKE ?",
		userID, prefix+"%",
	).Scan(&count); err != nil {
		t.Fatalf("查询任务数失败: %v", err)
	}
	return count
}

func TestNextOccurrenceRules(t *testing.T) {
	base := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) // 周二

	cases := []struct {
		name string
		rule string
		want time.Time
		ok   bool
	}{
		{"每天", RecurrenceDaily, base.AddDate(0, 0, 1), true},
		{"每周", RecurrenceWeekly, base.AddDate(0, 0, 7), true},
		{"工作日-周三", RecurrenceWeekdays, base.AddDate(0, 0, 1), true},
		{"无效规则", "yearly", time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := NextOccurrence(c.rule, base)
		if ok != c.ok {
			t.Errorf("%s: ok = %v, 期望 %v", c.name, ok, c.ok)
			continue
		}
		if ok && !got.Equal(c.want) {
			t.Errorf("%s: got %v, 期望 %v", c.name, got, c.want)
		}
	}

	// 工作日：周五 → 下周一
	friday := time.Date(2030, 1, 4, 0, 0, 0, 0, time.UTC)
	got, ok := NextOccurrence(RecurrenceWeekdays, friday)
	if !ok || got.Weekday() != time.Monday || !got.Equal(time.Date(2030, 1, 7, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("工作日周五推进失败: got %v ok=%v", got, ok)
	}

	// 每月：1/31 → 2/28（月末钳制）
	jan31 := time.Date(2030, 1, 31, 0, 0, 0, 0, time.UTC)
	got, ok = NextOccurrence(RecurrenceMonthly, jan31)
	if !ok || !got.Equal(time.Date(2030, 2, 28, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("每月月末钳制失败: got %v ok=%v", got, ok)
	}
}

func TestMaterializeNextOnCompletion(t *testing.T) {
	setupTestDB(t)
	userID := int(createTestUser(t, "recur@test.com", "user"))

	taskID := createRecurringTask(t, userID, "local-recur-1", "每周报告", RecurrenceWeekly, "2030-01-01T00:00:00Z")
	completeTask(t, userID, taskID)

	if err := MaterializeNextIfRecurring(userID, taskID); err != nil {
		t.Fatalf("物化下一次失败: %v", err)
	}

	// 新任务生成：local_id = series:2030-01-08，状态 todo，due_at 为下周
	var newID int64
	var localID, status, dueAt, rule string
	err := DB.QueryRow(
		"SELECT id, local_id, status, COALESCE(due_at,''), recurrence_type FROM tasks WHERE user_id = ? AND local_id LIKE '%:2030-01-08'",
		userID,
	).Scan(&newID, &localID, &status, &dueAt, &rule)
	if err != nil {
		t.Fatalf("未找到生成的下一次任务: %v", err)
	}
	if status != "todo" {
		t.Errorf("新任务状态 = %s, 期望 todo", status)
	}
	if rule != RecurrenceWeekly {
		t.Errorf("新任务循环规则 = %s, 期望 weekly", rule)
	}
	if !strings.Contains(dueAt, "2030-01-08") {
		t.Errorf("新任务 due_at = %s, 期望包含 2030-01-08", dueAt)
	}

	// 原任务补齐了系列 ID，且新任务与原任务同系列
	var origSeries, newSeries string
	if err := DB.QueryRow("SELECT recurrence_series_id FROM tasks WHERE id = ?", taskID).Scan(&origSeries); err != nil {
		t.Fatalf("查询原任务系列失败: %v", err)
	}
	if err := DB.QueryRow("SELECT recurrence_series_id FROM tasks WHERE id = ?", newID).Scan(&newSeries); err != nil {
		t.Fatalf("查询新任务系列失败: %v", err)
	}
	if origSeries == "" || origSeries != newSeries {
		t.Errorf("系列 ID 不一致: 原=%s 新=%s", origSeries, newSeries)
	}
	if !strings.HasPrefix(localID, origSeries+":") {
		t.Errorf("新任务 local_id = %s, 应以系列 ID 为前缀", localID)
	}
}

func TestMaterializeIdempotent(t *testing.T) {
	setupTestDB(t)
	userID := int(createTestUser(t, "idem@test.com", "user"))

	taskID := createRecurringTask(t, userID, "local-idem-1", "每天打卡", RecurrenceDaily, "2030-01-01T00:00:00Z")
	completeTask(t, userID, taskID)

	if err := MaterializeNextIfRecurring(userID, taskID); err != nil {
		t.Fatalf("第一次物化失败: %v", err)
	}
	// 模拟"撤销后再完成"的重复物化
	if err := MaterializeNextIfRecurring(userID, taskID); err != nil {
		t.Fatalf("第二次物化失败: %v", err)
	}

	// 只应有 1 个 local_id 以 series:2030-01-02 结尾的任务 + 1 个原任务
	var count int
	if err := DB.QueryRow(
		"SELECT COUNT(*) FROM tasks WHERE user_id = ? AND local_id LIKE '%:2030-01-02'",
		userID,
	).Scan(&count); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != 1 {
		t.Errorf("重复物化产生了 %d 个下一次任务, 期望 1 个", count)
	}
}

func TestMaterializeSkipsNonRecurringAndNonDone(t *testing.T) {
	setupTestDB(t)
	userID := int(createTestUser(t, "skip@test.com", "user"))

	// 非循环任务完成：不生成
	plainID, _ := CreateTask(userID, "local-plain", "普通任务")
	completeTask(t, userID, plainID)
	if err := MaterializeNextIfRecurring(userID, plainID); err != nil {
		t.Fatalf("物化失败: %v", err)
	}
	if got := countTasksByLocalIDPrefix(t, userID, "local-plain"); got != 1 {
		t.Errorf("非循环任务产生了衍生任务, 任务数=%d", got)
	}

	// 循环任务未完成：不生成
	recurID := createRecurringTask(t, userID, "local-notdone", "未完成循环", RecurrenceDaily, "2030-01-01T00:00:00Z")
	if err := MaterializeNextIfRecurring(userID, recurID); err != nil {
		t.Fatalf("物化失败: %v", err)
	}
	if got := countTasksByLocalIDPrefix(t, userID, "local-notdone"); got != 1 {
		t.Errorf("未完成任务产生了衍生任务, 任务数=%d", got)
	}
}

func TestMaterializeOverdueAdvancesFromToday(t *testing.T) {
	setupTestDB(t)
	userID := int(createTestUser(t, "overdue@test.com", "user"))

	// 到期日远在过去：完成后下一次应从"今天"起算，而不是从过期日顺延
	taskID := createRecurringTask(t, userID, "local-overdue", "逾期每日任务", RecurrenceDaily, "2020-01-01T00:00:00Z")
	completeTask(t, userID, taskID)

	if err := MaterializeNextIfRecurring(userID, taskID); err != nil {
		t.Fatalf("物化失败: %v", err)
	}

	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	var dueAt string
	err := DB.QueryRow(
		"SELECT COALESCE(due_at,'') FROM tasks WHERE user_id = ? AND local_id LIKE '%:'||?",
		userID, tomorrow,
	).Scan(&dueAt)
	if err != nil {
		t.Fatalf("未找到以明天为到期日的新任务: %v", err)
	}
}

func TestManualSortFiltering(t *testing.T) {
	setupTestDB(t)
	userID := int(createTestUser(t, "sort@test.com", "user"))

	// 直接插入以控制 sort_order 与 created_at
	now := time.Now().UTC()
	rows := []struct {
		localID   string
		sortOrder float64
		createdAt time.Time
	}{
		{"sort-c", 10, now.Add(-2 * time.Hour)},
		{"sort-a", 5, now.Add(-1 * time.Hour)},
		{"sort-b", 0, now},
	}
	for _, r := range rows {
		if _, err := DB.Exec(
			"INSERT INTO tasks (user_id, local_id, server_version, title, status, created_at, updated_at, last_modified, sort_order) VALUES (?, ?, 1, ?, 'todo', ?, ?, ?, ?)",
			userID, r.localID, "任务-"+r.localID, r.createdAt, r.createdAt, r.createdAt, r.sortOrder,
		); err != nil {
			t.Fatalf("插入测试任务失败: %v", err)
		}
	}

	tasks, total, err := GetTasksFiltered(userID, 1, 20, "", "", "", "manual", "desc")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, 期望 3", total)
	}
	wantOrder := []string{"sort-b", "sort-a", "sort-c"}
	for i, want := range wantOrder {
		if tasks[i]["local_id"] != want {
			t.Errorf("manual 排序第 %d 位 = %v, 期望 %s", i, tasks[i]["local_id"], want)
		}
	}

	// 老数据 sort_order 全 0 时回退 created_at DESC（平滑过渡验证）
	if _, err := DB.Exec("UPDATE tasks SET sort_order = 0 WHERE user_id = ?", userID); err != nil {
		t.Fatalf("重置 sort_order 失败: %v", err)
	}
	tasks, _, err = GetTasksFiltered(userID, 1, 20, "", "", "", "manual", "desc")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	wantFallback := []string{"sort-b", "sort-a", "sort-c"} // created_at 倒序
	for i, want := range wantFallback {
		if tasks[i]["local_id"] != want {
			t.Errorf("回退排序第 %d 位 = %v, 期望 %s", i, tasks[i]["local_id"], want)
		}
	}
}

func TestUpdateTaskWithVersionExtraFields(t *testing.T) {
	setupTestDB(t)
	userID := int(createTestUser(t, "extra@test.com", "user"))

	taskID, _ := CreateTask(userID, "local-extra", "扩展字段任务")
	if err := UpdateTaskFull(taskID, userID, "", "", "", "", "2030-01-01T00:00:00Z", true,
		&TaskExtraFields{RecurrenceType: strPtr(RecurrenceMonthly), RecurrenceSeriesID: strPtr("series-x")}); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	// extra 携带 sort_order 与 recurrence_type：应更新
	if err := UpdateTaskWithVersion(DB, userID, taskID, "扩展字段任务", "", "todo", "high", nil, 2,
		&TaskExtraFields{SortOrder: f64Ptr(42.5), RecurrenceType: strPtr(RecurrenceWeekly)}); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	var sortOrder float64
	var rule, series string
	if err := DB.QueryRow(
		"SELECT sort_order, recurrence_type, COALESCE(recurrence_series_id,'') FROM tasks WHERE id = ?",
		taskID,
	).Scan(&sortOrder, &rule, &series); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if sortOrder != 42.5 || rule != RecurrenceWeekly || series != "series-x" {
		t.Errorf("extra 更新不生效: sort_order=%v rule=%s series=%s", sortOrder, rule, series)
	}

	// extra 为空指针字段：不更新（保持原值）
	if err := UpdateTaskWithVersion(DB, userID, taskID, "扩展字段任务", "", "todo", "high", nil, 3,
		&TaskExtraFields{}); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if err := DB.QueryRow(
		"SELECT sort_order, recurrence_type FROM tasks WHERE id = ?", taskID,
	).Scan(&sortOrder, &rule); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if sortOrder != 42.5 || rule != RecurrenceWeekly {
		t.Errorf("空 extra 指针误清了字段: sort_order=%v rule=%s", sortOrder, rule)
	}
}

func TestEnsureRecurrenceSeriesID(t *testing.T) {
	setupTestDB(t)
	userID := int(createTestUser(t, "series@test.com", "user"))

	taskID, _ := CreateTask(userID, "local-series", "系列任务")

	// 非循环任务：不生成系列
	if s, err := EnsureRecurrenceSeriesID(DB, userID, taskID); err != nil || s != "" {
		t.Errorf("非循环任务应返回空系列: s=%s err=%v", s, err)
	}

	// 设置循环后：生成并持久化；再次调用返回同一 ID
	if err := UpdateTaskFull(taskID, userID, "", "", "", "", "", false,
		&TaskExtraFields{RecurrenceType: strPtr(RecurrenceDaily)}); err != nil {
		t.Fatalf("设置循环失败: %v", err)
	}
	first, err := EnsureRecurrenceSeriesID(DB, userID, taskID)
	if err != nil || first == "" {
		t.Fatalf("生成系列失败: s=%s err=%v", first, err)
	}
	second, err := EnsureRecurrenceSeriesID(DB, userID, taskID)
	if err != nil || second != first {
		t.Errorf("系列 ID 应保持稳定: first=%s second=%s", first, second)
	}
}

func TestGetTasksChangedSinceIncludesNewFields(t *testing.T) {
	setupTestDB(t)
	userID := int(createTestUser(t, "changed@test.com", "user"))

	taskID := createRecurringTask(t, userID, "local-changed", "增量字段任务", RecurrenceWeekly, "2030-01-01T00:00:00Z")
	if _, err := DB.Exec("UPDATE tasks SET sort_order = 7.5 WHERE id = ?", taskID); err != nil {
		t.Fatalf("设置 sort_order 失败: %v", err)
	}

	changed, err := GetTasksChangedSince(userID, time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatalf("增量查询失败: %v", err)
	}
	if len(changed) != 1 {
		t.Fatalf("增量结果数 = %d, 期望 1", len(changed))
	}
	row := changed[0]
	if row["due_at"] == "" || row["due_at"] == nil {
		t.Errorf("增量结果缺少 due_at（R2 回归）")
	}
	if v, ok := row["sort_order"].(float64); !ok || v != 7.5 {
		t.Errorf("增量结果 sort_order = %v, 期望 7.5", row["sort_order"])
	}
	if row["recurrence_type"] != RecurrenceWeekly {
		t.Errorf("增量结果 recurrence_type = %v, 期望 weekly", row["recurrence_type"])
	}
	if row["completed_at"] == nil {
		t.Errorf("增量结果缺少 completed_at 键")
	}
}
