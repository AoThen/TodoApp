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
		EscapeCSV(task["id"]),
		EscapeCSV(task["local_id"]),
		EscapeCSV(task["server_version"]),
		EscapeCSV(task["title"]),
		EscapeCSV(task["description"]),
		EscapeCSV(task["status"]),
		EscapeCSV(task["priority"]),
		EscapeCSV(task["due_at"]),
		EscapeCSV(task["created_at"]),
		EscapeCSV(task["updated_at"]),
		EscapeCSV(task["completed_at"]),
		EscapeCSV(task["is_deleted"]),
		EscapeCSV(task["last_modified"]),
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
