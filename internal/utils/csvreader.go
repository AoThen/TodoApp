package utils

import (
	"encoding/csv"
	"io"
	"strings"
)

// CSVReader CSV读取器，用于数据导入
type CSVReader struct {
	reader *csv.Reader
}

// NewCSVReader 创建新的CSV读取器
func NewCSVReader(r io.Reader) *CSVReader {
	return &CSVReader{
		reader: csv.NewReader(r),
	}
}

// ReadAll 读取所有CSV记录并转换为map格式
func (cr *CSVReader) ReadAll() ([]map[string]string, error) {
	headers, err := cr.reader.Read()
	if err != nil {
		return nil, err
	}

	var records []map[string]string
	for {
		record, err := cr.reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		row := make(map[string]string)
		for i, value := range record {
			if i < len(headers) {
				key := strings.ToLower(strings.TrimSpace(headers[i]))
				row[key] = strings.TrimSpace(value)
			}
		}
		records = append(records, row)
	}

	return records, nil
}
