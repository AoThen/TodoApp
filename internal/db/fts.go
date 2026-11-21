package db

import (
	"fmt"
	"log"
	"strings"
)

// ftsAvailable 标记 tasks_fts 全文索引是否可用。
// 依赖构建标签 sqlite_fts5（Dockerfile/CI/Makefile 已启用）；
// 未启用或运行时探测失败时自动退回 LIKE 搜索，行为与历史版本一致。
var (
	ftsAvailable bool
	ftsTokenizer string
)

// FTSAvailable 返回全文索引可用性（供测试与诊断）
func FTSAvailable() bool { return ftsAvailable }

// InitFTS 初始化任务全文索引（外部内容表 + 同步触发器）。
// 依次探测 trigram（支持中文等连续文字的子串匹配，语义最接近 LIKE）
// 与默认 unicode61（词前缀匹配）分词器；均不可用时保持 LIKE 模式。
func InitFTS() {
	ftsAvailable = false
	ftsTokenizer = ""

	for _, tokenizer := range []string{"trigram", ""} {
		if err := createFTSObjects(tokenizer); err == nil {
			ftsAvailable = true
			ftsTokenizer = tokenizer
			name := tokenizer
			if name == "" {
				name = "unicode61"
			}
			log.Printf("任务全文索引已启用 (tokenizer=%s)", name)
			return
		}
		// 清理半成品（如表建成但触发器/rebuild 失败），避免残留状态影响下次探测
		_ = dropFTSObjects()
	}

	log.Println("警告: 当前 SQLite 编译未启用 FTS5，任务搜索退回 LIKE 模式（如需全文索引请使用 -tags sqlite_fts5 构建）")
}

func createFTSObjects(tokenizer string) error {
	opts := ""
	if tokenizer != "" {
		opts = ", tokenize = '" + tokenizer + "'"
	}
	if _, err := DB.Exec(fmt.Sprintf(
		"CREATE VIRTUAL TABLE IF NOT EXISTS tasks_fts USING fts5(title, description, content='tasks', content_rowid='id'%s)",
		opts)); err != nil {
		return err
	}

	triggers := []string{
		`CREATE TRIGGER IF NOT EXISTS tasks_fts_insert AFTER INSERT ON tasks BEGIN
			INSERT INTO tasks_fts(rowid, title, description) VALUES (new.id, new.title, new.description);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS tasks_fts_delete AFTER DELETE ON tasks BEGIN
			INSERT INTO tasks_fts(tasks_fts, rowid, title, description) VALUES ('delete', old.id, old.title, old.description);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS tasks_fts_update AFTER UPDATE ON tasks BEGIN
			INSERT INTO tasks_fts(tasks_fts, rowid, title, description) VALUES ('delete', old.id, old.title, old.description);
			INSERT INTO tasks_fts(rowid, title, description) VALUES (new.id, new.title, new.description);
		END;`,
	}
	for _, t := range triggers {
		if _, err := DB.Exec(t); err != nil {
			return err
		}
	}

	// 重建索引以覆盖存量数据（幂等操作）
	if _, err := DB.Exec("INSERT INTO tasks_fts(tasks_fts) VALUES('rebuild')"); err != nil {
		return err
	}
	return nil
}

func dropFTSObjects() error {
	for _, q := range []string{
		"DROP TRIGGER IF EXISTS tasks_fts_insert",
		"DROP TRIGGER IF EXISTS tasks_fts_delete",
		"DROP TRIGGER IF EXISTS tasks_fts_update",
		"DROP TABLE IF EXISTS tasks_fts",
	} {
		if _, err := DB.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// buildFTSQuery 将用户搜索词转换为安全的 FTS MATCH 表达式。
// 统一按短语处理并剥离引号，杜绝 FTS 语法注入；返回空串表示应退回 LIKE。
func buildFTSQuery(search string) string {
	phrase := strings.Map(func(r rune) rune {
		if r == '"' || r == '*' || r == '(' || r == ')' || r == '^' || r == ':' {
			return -1 // 剥离 FTS 语法字符
		}
		return r
	}, search)
	phrase = strings.TrimSpace(phrase)
	if phrase == "" {
		return ""
	}
	// trigram 分词器做子串匹配，最少需要 3 个字符；更短的查询退回 LIKE
	if ftsTokenizer == "trigram" && len([]rune(phrase)) < 3 {
		return ""
	}
	if ftsTokenizer == "trigram" {
		return `"` + phrase + `"`
	}
	// unicode61：短语前缀匹配
	return `"` + phrase + `"*`
}
