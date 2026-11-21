package types

import "testing"

// TestConflictResolutionConstants 守护冲突解决选项与 API 契约的一致性
func TestConflictResolutionConstants(t *testing.T) {
	if ConflictKeepServer != "keep_server" {
		t.Errorf(`ConflictKeepServer = %q, 期望 "keep_server"`, ConflictKeepServer)
	}
	if ConflictKeepClient != "keep_client" {
		t.Errorf(`ConflictKeepClient = %q, 期望 "keep_client"`, ConflictKeepClient)
	}
	if ConflictMerge != "merge" {
		t.Errorf(`ConflictMerge = %q, 期望 "merge"`, ConflictMerge)
	}
}

func TestPrioritizeStatus(t *testing.T) {
	cases := []struct {
		name string
		s1   string
		s2   string
		want string
	}{
		{"完成优先于待办", "todo", "done", "done"},
		{"完成优先于进行中", "in_progress", "done", "done"},
		{"进行中优先于待办", "todo", "in_progress", "in_progress"},
		{"归档让位于待办", "archived", "todo", "todo"},
		{"归档让位于完成", "done", "archived", "done"},
		{"双方均归档保持归档", "archived", "archived", "archived"},
		{"相同状态保持原样", "in_progress", "in_progress", "in_progress"},
		{"双方未知返回第一个", "unknown_a", "unknown_b", "unknown_a"},
		{"单方未知返回已知方", "archived", "unknown", "archived"},
		{"单方未知返回已知方2", "unknown", "todo", "todo"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PrioritizeStatus(tc.s1, tc.s2); got != tc.want {
				t.Errorf("PrioritizeStatus(%q, %q) = %q, 期望 %q", tc.s1, tc.s2, got, tc.want)
			}
		})
	}
}
