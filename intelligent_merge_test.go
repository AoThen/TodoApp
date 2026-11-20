package main

import "testing"

func TestIntelligentMergeNoClientProps(t *testing.T) {
	title, desc, status := intelligentMerge("服务端标题", "服务端描述", "todo", map[string]interface{}{})
	if title != "服务端标题" || desc != "服务端描述" || status != "todo" {
		t.Errorf("空客户端属性应保持服务端值: (%q, %q, %q)", title, desc, status)
	}
}

func TestIntelligentMergeTitleServerWins(t *testing.T) {
	// 客户端与服务端标题不同 → 服务器优先
	title, _, _ := intelligentMerge("服务端标题", "", "todo", map[string]interface{}{
		"title": "客户端标题",
	})
	if title != "服务端标题" {
		t.Errorf("标题冲突应服务器优先, got %q", title)
	}

	// 标题一致 → 保持
	title2, _, _ := intelligentMerge("相同标题", "", "todo", map[string]interface{}{
		"title": "相同标题",
	})
	if title2 != "相同标题" {
		t.Errorf("标题一致时应保持, got %q", title2)
	}

	// 客户端标题为空串 → 不覆盖
	title3, _, _ := intelligentMerge("服务端标题", "", "todo", map[string]interface{}{
		"title": "",
	})
	if title3 != "服务端标题" {
		t.Errorf("客户端标题为空应保持服务端, got %q", title3)
	}
}

func TestIntelligentMergeDescriptionConcat(t *testing.T) {
	// 双方描述均非空且不同 → 拼接
	_, desc, _ := intelligentMerge("t", "服务器描述", "todo", map[string]interface{}{
		"description": "客户端描述",
	})
	want := "[服务器更新] 服务器描述\n\n[客户端更新] 客户端描述"
	if desc != want {
		t.Errorf("描述应双向拼接:\n want %q\n got  %q", want, desc)
	}
}

func TestIntelligentMergeDescriptionFallbacks(t *testing.T) {
	cases := []struct {
		name       string
		serverDesc string
		clientDesc string
		want       string
	}{
		{"客户端为空保持服务端", "服务端", "", "服务端"},
		{"服务端为空取客户端", "", "客户端", "客户端"},
		{"双方一致取其一", "相同", "相同", "相同"},
		{"双方均空", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, desc, _ := intelligentMerge("t", c.serverDesc, "todo", map[string]interface{}{
				"description": c.clientDesc,
			})
			if desc != c.want {
				t.Errorf("desc = %q, want %q", desc, c.want)
			}
		})
	}
}

func TestIntelligentMergeStatusPriority(t *testing.T) {
	cases := []struct {
		name         string
		serverStatus string
		clientStatus string
		want         string
	}{
		{"客户端 done 优先", "todo", "done", "done"},
		{"服务端 done 优先", "done", "todo", "done"},
		{"客户端 in_progress 优先", "todo", "in_progress", "in_progress"},
		{"相同状态保持", "done", "done", "done"},
		{"客户端状态未知按未知处理", "in_progress", "archived", "in_progress"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, status := intelligentMerge("t", "", c.serverStatus, map[string]interface{}{
				"status": c.clientStatus,
			})
			if status != c.want {
				t.Errorf("status = %q, want %q", status, c.want)
			}
		})
	}
}

func TestIntelligentMergeCombined(t *testing.T) {
	title, desc, status := intelligentMerge(
		"服务器标题", "服务器描述", "in_progress",
		map[string]interface{}{
			"title":       "客户端标题",
			"description": "客户端描述",
			"status":      "done",
		})
	if title != "服务器标题" {
		t.Errorf("title = %q", title)
	}
	if desc != "[服务器更新] 服务器描述\n\n[客户端更新] 客户端描述" {
		t.Errorf("desc = %q", desc)
	}
	if status != "done" {
		t.Errorf("status = %q, want done（done 优先级最高）", status)
	}
}
