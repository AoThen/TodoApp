package utils

import (
	"encoding/csv"
	"strings"
	"testing"
)

func TestCSVReaderReadAll(t *testing.T) {
	input := "ID,Title,Status\n1,写周报,todo\n2,\"带,逗号\",done\n"
	rows, err := NewCSVReader(strings.NewReader(input)).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll 失败: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("应解析出 2 行, got %d", len(rows))
	}
	if rows[0]["id"] != "1" || rows[0]["title"] != "写周报" || rows[0]["status"] != "todo" {
		t.Errorf("第一行解析错误: %v", rows[0])
	}
	if rows[1]["title"] != "带,逗号" {
		t.Errorf("带引号逗号的字段解析错误: %v", rows[1])
	}
}

func TestCSVReaderHeaderNormalization(t *testing.T) {
	input := "  Title , STATUS\ntest,DONE\n"
	rows, err := NewCSVReader(strings.NewReader(input)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := rows[0]["title"]; !ok {
		t.Errorf("表头应小写化: %v", rows[0])
	}
	if _, ok := rows[0]["status"]; !ok {
		t.Errorf("表头应去除空白: %v", rows[0])
	}
}

func TestCSVReaderEmpty(t *testing.T) {
	if _, err := NewCSVReader(strings.NewReader("")).ReadAll(); err == nil {
		t.Error("空输入应当返回错误（缺少表头）")
	}
}

func TestCSVReaderValueTrim(t *testing.T) {
	input := "Title\n  hello  \n"
	rows, err := NewCSVReader(strings.NewReader(input)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if rows[0]["title"] != "hello" {
		t.Errorf("字段值应去除空白: %q", rows[0]["title"])
	}
}

func TestCSVStreamerWriteAndCount(t *testing.T) {
	var buf strings.Builder
	cs := NewCSVStreamer(&buf)
	if err := cs.WriteHeader([]string{"id", "title", "status"}); err != nil {
		t.Fatal(err)
	}
	task := map[string]interface{}{
		"id": 1, "title": "任务A", "status": "todo",
		"description": "备注", "local_id": "l-1", "server_version": 1,
	}
	if err := cs.WriteRow(task); err != nil {
		t.Fatal(err)
	}
	cs.Close()
	if cs.RowsWritten() != 1 {
		t.Errorf("RowsWritten = %d, want 1", cs.RowsWritten())
	}
	out := buf.String()
	if !strings.Contains(out, "任务A") || !strings.Contains(out, "id,title,status") {
		t.Errorf("CSV 输出缺少预期内容: %q", out)
	}
}

func TestCSVStreamerEscapesSpecialChars(t *testing.T) {
	var buf strings.Builder
	cs := NewCSVStreamer(&buf)
	// 含逗号/引号/换行的字段应能被正确解析还原（转义往返一致）
	task := map[string]interface{}{"id": 1, "title": "a,\"b\"\nc", "status": "todo"}
	if err := cs.WriteRow(task); err != nil {
		t.Fatal(err)
	}
	cs.Close()

	parsed, err := csv.NewReader(strings.NewReader(buf.String())).Read()
	if err != nil {
		t.Fatalf("写出内容无法解析: %v", err)
	}
	// WriteRow 仅做公式注入防护，结构转义由 csv.Writer 单次完成，解析应还原原始值
	if parsed[3] != "a,\"b\"\nc" {
		t.Errorf("特殊字符字段应被单次转义并可解析还原: %q", parsed[3])
	}

	// 以 = 开头的公式字符串应被前缀单引号，防止 Excel/WPS 公式注入
	var buf2 strings.Builder
	cs2 := NewCSVStreamer(&buf2)
	if err := cs2.WriteRow(map[string]interface{}{"id": 2, "title": "=SUM(A1)", "status": "todo"}); err != nil {
		t.Fatal(err)
	}
	cs2.Close()
	if !strings.Contains(buf2.String(), "'=SUM(A1)") {
		t.Errorf("公式字符串应被前缀单引号防护: %q", buf2.String())
	}
}

func TestSanitizeCSVFormula(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"空串", "", ""},
		{"普通文本", "写周报", "写周报"},
		{"等号公式", "=SUM(A1)", "'=SUM(A1)"},
		{"加号公式", "+cmd|'/C calc'", "'+cmd|'/C calc'"},
		{"减号公式", "-2+3", "'-2+3"},
		{"@宏", "@SUM(1)", "'@SUM(1)"},
		{"制表符开头", "\ttab", "'\ttab"},
		{"回车开头", "\rcr", "'\rcr"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitizeCSVFormula(c.value); got != c.want {
				t.Errorf("sanitizeCSVFormula(%q) = %q, want %q", c.value, got, c.want)
			}
		})
	}
}

func TestToCSVCell(t *testing.T) {
	if got := toCSVCell(42); got != "42" {
		t.Errorf("toCSVCell(42) = %q, want %q", got, "42")
	}
	if got := toCSVCell(nil); got != "" {
		t.Errorf("toCSVCell(nil) = %q, want 空串", got)
	}
	if got := toCSVCell("-1"); got != "'-1" {
		t.Errorf("toCSVCell(\"-1\") = %q, want %q（负数前缀防护）", got, "'-1")
	}
}

func TestEscapeCSV(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
		want  string
	}{
		{"nil", nil, ""},
		{"普通字符串", "abc", "abc"},
		{"整数", 42, "42"},
		{"int64", int64(7), "7"},
		{"浮点", 1.5, "1.5"},
		{"布尔真", true, "true"},
		{"布尔假", false, "false"},
		{"含逗号", "a,b", "\"a,b\""},
		{"含引号", "a\"b", "\"a\"\"b\""},
		{"含换行", "a\nb", "\"a\nb\""},
		{"未知类型", struct{}{}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := EscapeCSV(c.value); got != c.want {
				t.Errorf("EscapeCSV(%v) = %q, want %q", c.value, got, c.want)
			}
		})
	}
}

func TestCalculatePagination(t *testing.T) {
	cases := []struct {
		name                  string
		page, pageSize, total int
		offset, totalPages    int
		hasPrev, hasNext      bool
	}{
		{"正常中间页", 2, 10, 25, 10, 3, true, true},
		{"首页", 1, 10, 25, 0, 3, false, true},
		{"末页", 3, 10, 25, 20, 3, true, false},
		{"页码越界", 9, 10, 25, 80, 3, true, false},
		{"页码过小", 0, 10, 25, 0, 3, false, true},
		{"pageSize 过小", 1, 0, 25, 0, 2, false, true},
		{"空数据", 1, 10, 0, 0, 0, false, false},
		{"整除边界", 2, 10, 20, 10, 2, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			offset, totalPages, hasPrev, hasNext := CalculatePagination(c.page, c.pageSize, c.total)
			if offset != c.offset || totalPages != c.totalPages || hasPrev != c.hasPrev || hasNext != c.hasNext {
				t.Errorf("CalculatePagination(%d,%d,%d) = (%d,%d,%v,%v), want (%d,%d,%v,%v)",
					c.page, c.pageSize, c.total, offset, totalPages, hasPrev, hasNext,
					c.offset, c.totalPages, c.hasPrev, c.hasNext)
			}
		})
	}
}

func TestCSVStreamerManyRowsFlush(t *testing.T) {
	var buf strings.Builder
	cs := NewCSVStreamer(&buf)
	if err := cs.WriteHeader([]string{"id", "title"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 250; i++ {
		if err := cs.WriteRow(map[string]interface{}{"id": i, "title": "t"}); err != nil {
			t.Fatal(err)
		}
	}
	cs.Close()
	if cs.RowsWritten() != 250 {
		t.Errorf("RowsWritten = %d, want 250", cs.RowsWritten())
	}
	if got := strings.Count(buf.String(), "\n"); got < 250 {
		t.Errorf("输出行数不足: %d", got)
	}
}
