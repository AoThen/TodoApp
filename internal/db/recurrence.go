package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// 循环任务规则：与三端（Go/Web/Android）约定的基础四档。
// 规则推进一律基于 UTC 日期计算，避免时区歧义。
const (
	RecurrenceDaily    = "daily"
	RecurrenceWeekly   = "weekly"
	RecurrenceMonthly  = "monthly"
	RecurrenceWeekdays = "weekdays"
)

// IsValidRecurrenceRule 校验规则是否受支持
func IsValidRecurrenceRule(rule string) bool {
	switch rule {
	case RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly, RecurrenceWeekdays:
		return true
	}
	return false
}

// newUUID 生成 32 位 hex 随机串（项目未引入 uuid 库，crypto/rand 足够）
func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 失败极为罕见，退回时间戳保证可用性
		return fmt.Sprintf("s%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// NextOccurrence 按规则计算 base（含）之后的下一次日期（UTC 零点）。
// monthly 做月末钳制（1/31 → 2/28）；weekdays 跳过周六日。
func NextOccurrence(rule string, base time.Time) (time.Time, bool) {
	base = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, time.UTC)
	switch rule {
	case RecurrenceDaily:
		return base.AddDate(0, 0, 1), true
	case RecurrenceWeekly:
		return base.AddDate(0, 0, 7), true
	case RecurrenceMonthly:
		return addMonthClamped(base), true
	case RecurrenceWeekdays:
		next := base.AddDate(0, 0, 1)
		for next.Weekday() == time.Saturday || next.Weekday() == time.Sunday {
			next = next.AddDate(0, 0, 1)
		}
		return next, true
	}
	return time.Time{}, false
}

// addMonthClamped 加一个月并把日号钳制到目标月最后一天
func addMonthClamped(base time.Time) time.Time {
	y, m, d := base.Date()
	firstOfTarget := time.Date(y, m+1, 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstOfTarget.AddDate(0, 1, 0).AddDate(0, 0, -1).Day()
	if d > lastDay {
		d = lastDay
	}
	return time.Date(y, m+1, d, 0, 0, 0, 0, time.UTC)
}

// parseTaskTime 宽松解析任务时间列（客户端可能写入 RFC3339 / 日期 / 空格分隔等格式）
func parseTaskTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// EnsureRecurrenceSeriesID 任务设置了循环规则但尚无系列 ID 时补齐（返回最终系列 ID）。
// q 可传事务（sync 路径）或全局连接（REST 路径）。
func EnsureRecurrenceSeriesID(q DBTX, userID int, taskID int64) (string, error) {
	var rule, series string
	err := q.QueryRow(
		"SELECT recurrence_type, COALESCE(recurrence_series_id,'') FROM tasks WHERE id = ? AND user_id = ?",
		taskID, userID,
	).Scan(&rule, &series)
	if err != nil {
		return "", err
	}
	if !IsValidRecurrenceRule(rule) || series != "" {
		return series, nil
	}
	series = newUUID()
	if _, err := q.Exec(
		"UPDATE tasks SET recurrence_series_id = ? WHERE id = ? AND user_id = ?",
		series, taskID, userID,
	); err != nil {
		return "", err
	}
	return series, nil
}

// NewSeriesID 生成新的循环系列 ID（供插入路径在写入前预生成）
func NewSeriesID() string { return newUUID() }

// MaterializeNextIfRecurring 独立事务版物化入口（REST PATCH 等非同步事务路径使用）
func MaterializeNextIfRecurring(userID int, taskID int64) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	if err := MaterializeNextTx(tx, userID, taskID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// MaterializeNextTx 在调用方事务内，为刚完成（status=done）的循环任务生成下一次 occurrence。
// 幂等保证：新任务 local_id = series_id:下一到期日，已存在则跳过（重复完成/撤销再完成不会重复生成）。
// 逾期补生成规则：base = max(原到期日, 今天)，保证不生成已过期的新任务。
func MaterializeNextTx(tx *sql.Tx, userID int, taskID int64) error {
	var status, dueAt, completedAt, rule, series, title, description, priority string
	err := tx.QueryRow(`
		SELECT status, COALESCE(due_at,''), COALESCE(completed_at,''),
		       COALESCE(recurrence_type,''), COALESCE(recurrence_series_id,''),
		       COALESCE(title,''), COALESCE(description,''), COALESCE(priority,'medium')
		FROM tasks WHERE id = ? AND user_id = ? AND is_deleted = 0
	`, taskID, userID).Scan(&status, &dueAt, &completedAt, &rule, &series, &title, &description, &priority)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	// 仅在"已完成"且规则有效时物化；调用方负责判断状态迁移，这里二次防御
	if status != "done" || !IsValidRecurrenceRule(rule) {
		return nil
	}

	// base：优先到期日，其次完成时间，最后当前时间
	base, ok := parseTaskTime(dueAt)
	if !ok {
		base, ok = parseTaskTime(completedAt)
	}
	if !ok {
		base = time.Now().UTC()
	}
	// 不生成已过期的新任务：从 max(base, 今天) 起算
	today := time.Now().UTC()
	if base.Before(time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)) {
		base = today
	}

	next, ok := NextOccurrence(rule, base)
	if !ok {
		return nil
	}

	// 系列缺失（如历史数据/导入数据）时补齐并回写当前任务，保证后续链条一致
	if series == "" {
		series = newUUID()
		if _, err := tx.Exec(
			"UPDATE tasks SET recurrence_series_id = ? WHERE id = ? AND user_id = ?",
			series, taskID, userID,
		); err != nil {
			return err
		}
	}

	nextLocalID := fmt.Sprintf("%s:%s", series, next.Format("2006-01-02"))

	// 幂等：该系列该日期的任务已存在则跳过
	var exists int
	if err := tx.QueryRow(
		"SELECT 1 FROM tasks WHERE user_id = ? AND local_id = ? AND is_deleted = 0 LIMIT 1",
		userID, nextLocalID,
	).Scan(&exists); err == nil {
		return nil
	} else if err != sql.ErrNoRows {
		return err
	}

	now := time.Now().UTC()
	nextDue := next.Format("2006-01-02T00:00:00Z")
	// 新任务 sort_order=0 置顶（manual 模式下与"新任务在最前"的既有习惯一致）
	if _, err := tx.Exec(`
		INSERT INTO tasks (user_id, local_id, server_version, title, description, status, priority,
			due_at, created_at, updated_at, last_modified, sort_order, recurrence_type, recurrence_series_id)
		VALUES (?, ?, 1, ?, ?, 'todo', ?, ?, ?, ?, ?, 0, ?, ?)
	`, userID, nextLocalID, title, description, priority, nextDue, now, now, now, rule, series); err != nil {
		return err
	}
	return nil
}
