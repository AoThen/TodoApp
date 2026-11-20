package utils

import (
	"encoding/csv"
	"io"
	"log"
)

// CSVStreamer 流式CSV写入器，避免一次性加载所有数据到内存
type CSVStreamer struct {
	writer      *csv.Writer
	rowsWritten int
}

// NewCSVStreamer 创建新的CSV流式写入器
func NewCSVStreamer(w io.Writer) *CSVStreamer {
	return &CSVStreamer{
		writer: csv.NewWriter(w),
	}
}

// WriteHeader 写入CSV表头
func (cs *CSVStreamer) WriteHeader(headers []string) error {
	return cs.writer.Write(headers)
}

// WriteRow 流式写入单行数据
func (cs *CSVStreamer) WriteRow(task map[string]interface{}) error {
	row := []string{
		toCSVCell(task["id"]),
		toCSVCell(task["local_id"]),
		toCSVCell(task["server_version"]),
		toCSVCell(task["title"]),
		toCSVCell(task["description"]),
		toCSVCell(task["status"]),
		toCSVCell(task["priority"]),
		toCSVCell(task["due_at"]),
		toCSVCell(task["created_at"]),
		toCSVCell(task["updated_at"]),
		toCSVCell(task["completed_at"]),
		toCSVCell(task["is_deleted"]),
		toCSVCell(task["last_modified"]),
	}

	if err := cs.writer.Write(row); err != nil {
		return err
	}

	cs.rowsWritten++

	if cs.rowsWritten%100 == 0 {
		cs.writer.Flush()
	}

	return nil
}

// toCSVCell 将字段值转为字符串并做公式注入防护；CSV 结构转义（逗号/引号/换行）统一交给 csv.Writer
func toCSVCell(value interface{}) string {
	return sanitizeCSVFormula(convertToString(value))
}

// sanitizeCSVFormula 防护 Excel/WPS 公式注入：以 = + - @ \t \r \n 或空格开头的值前缀单引号
// （Excel 打开 CSV 时会剥离前导空格，故空格前缀同样危险），使其按文本处理
func sanitizeCSVFormula(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r', '\n', ' ':
		return "'" + s
	}
	return s
}

// WriteRows 批量写入行数据
func (cs *CSVStreamer) WriteRows(tasks []map[string]interface{}) error {
	for _, task := range tasks {
		if err := cs.WriteRow(task); err != nil {
			return err
		}
	}
	return nil
}

// Close 关闭流式写入器并刷新缓冲区
func (cs *CSVStreamer) Close() {
	cs.writer.Flush()
	if err := cs.writer.Error(); err != nil {
		log.Printf("CSV刷新错误: %v", err)
	}
}

// RowsWritten 返回已写入的行数
func (cs *CSVStreamer) RowsWritten() int {
	return cs.rowsWritten
}
