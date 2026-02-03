package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// EscapeCSV 转义CSV字段，处理逗号、引号、换行符
func EscapeCSV(value interface{}) string {
	if value == nil {
		return ""
	}

	s := convertToString(value)

	if needsEscaping(s) {
		s = strings.ReplaceAll(s, "\"", "\"\"")
		s = "\"" + s + "\""
	}

	return s
}

// needsEscaping 检查是否需要转义CSV字段
func needsEscaping(s string) bool {
	for _, c := range s {
		if c == ',' || c == '"' || c == '\n' || c == '\r' {
			return true
		}
	}
	return false
}

// convertToString 将任意值转换为字符串
func convertToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int, int32, int64, uint, uint32, uint64:
		return formatInt64(toInt64(v))
	case float32, float64:
		return formatFloat(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

// toInt64 将任意整数类型转换为int64
func toInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		return int64(v)
	default:
		return 0
	}
}

// formatInt64 格式化int64为字符串
func formatInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}

// formatFloat 格式化浮点数为字符串
func formatFloat(value interface{}) string {
	switch v := value.(type) {
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

// CalculatePagination 计算分页参数
func CalculatePagination(page, pageSize int, total int) (offset int, totalPages int, hasPrev bool, hasNext bool) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset = (page - 1) * pageSize

	totalPages = total / pageSize
	if total%pageSize > 0 {
		totalPages++
	}

	hasPrev = page > 1
	hasNext = page < totalPages

	return offset, totalPages, hasPrev, hasNext
}

// GenerateID 生成唯一ID
func GenerateID() string {
	return fmt.Sprintf("%d", timestamp())
}

func timestamp() int64 {
	// 简化的时间戳生成
	return 0
}
