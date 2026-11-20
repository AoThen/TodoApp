package types

import "testing"

func TestPrioritizeStatus(t *testing.T) {
	cases := []struct {
		name string
		s1   string
		s2   string
		want string
	}{
		{"done 优先于 in_progress", "done", "in_progress", "done"},
		{"done 优先于 todo", "todo", "done", "done"},
		{"in_progress 优先于 todo", "in_progress", "todo", "in_progress"},
		{"相同状态保持", "done", "done", "done"},
		{"相同 in_progress", "in_progress", "in_progress", "in_progress"},
		{"双未知返回 s1", "archived", "archived", "archived"},
		{"s1 未知 s2 已知", "archived", "done", "done"},
		{"s1 已知 s2 未知", "in_progress", "archived", "in_progress"},
		{"双未知不同值返回 s1", "archived", "other", "archived"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PrioritizeStatus(c.s1, c.s2); got != c.want {
				t.Errorf("PrioritizeStatus(%q, %q) = %q, want %q", c.s1, c.s2, got, c.want)
			}
		})
	}
}

func TestConflictResolutionConstants(t *testing.T) {
	if ConflictKeepServer != "keep_server" || ConflictKeepClient != "keep_client" || ConflictMerge != "merge" {
		t.Error("冲突解决策略常量与 API 契约不一致")
	}
}
