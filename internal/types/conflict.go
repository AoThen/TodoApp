package types

// ConflictResolution 冲突解决策略类型
type ConflictResolution string

const (
	ConflictKeepServer ConflictResolution = "keep_server"
	ConflictKeepClient ConflictResolution = "keep_client"
	ConflictMerge      ConflictResolution = "merge"
)

// FieldLevelConflict 字段级冲突信息
type FieldLevelConflict struct {
	FieldName   string      `json:"field_name"`
	ServerValue interface{} `json:"server_value"`
	ClientValue interface{} `json:"client_value"`
}

// ConflictRecord 冲突记录
type ConflictRecord struct {
	LocalID   string               `json:"local_id"`
	ServerID  int64                `json:"server_id"`
	Reason    string               `json:"reason"`
	Options   []ConflictResolution `json:"options"`
	Conflicts []FieldLevelConflict `json:"conflicts,omitempty"`
	CreatedAt string               `json:"created_at"`
}

// MergeResult 合并结果
type MergeResult struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
}

// PriorityState 优先级状态
type PriorityState string

const (
	PriorityLow    PriorityState = "low"
	PriorityMedium PriorityState = "medium"
	PriorityHigh   PriorityState = "high"
)

// StatusState 任务状态
type StatusState string

const (
	StatusTodo       StatusState = "todo"
	StatusInProgress StatusState = "in_progress"
	StatusDone       StatusState = "done"
	StatusArchived   StatusState = "archived"
)

// PrioritizeStatus 状态优先级排序（已完成 > 进行中 > 待办）
func PrioritizeStatus(s1, s2 string) string {
	statusMap := map[string]int{
		"done":        3,
		"in_progress": 2,
		"todo":        1,
	}

	p1, ok1 := statusMap[s1]
	p2, ok2 := statusMap[s2]

	if !ok1 && !ok2 {
		return s1
	}
	if !ok1 {
		return s2
	}
	if !ok2 {
		return s1
	}

	if p1 >= p2 {
		return s1
	}
	return s2
}
