package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// EnvDuration 读取时长环境变量（支持 Go duration 字符串，如 15m、30m、1h、24h）。
// 未设置时返回默认值；非法或非正值记录日志并返回默认值。
func EnvDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		log.Printf("环境变量 %s 无效（%q），使用默认值 %s", key, v, def)
		return def
	}
	return d
}

// EnvInt 读取整数环境变量。未设置时返回默认值；非法或非正值记录日志并返回默认值。
func EnvInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		log.Printf("环境变量 %s 无效（%q），使用默认值 %d", key, v, def)
		return def
	}
	return n
}

// EnvBool 读取布尔环境变量（接受 true/1 与 false/0，不区分大小写）。
// 未设置或非法时返回默认值。
func EnvBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return def
	}
	switch v {
	case "true", "1":
		return true
	case "false", "0":
		return false
	default:
		log.Printf("环境变量 %s 无效（%q），使用默认值 %v", key, v, def)
		return def
	}
}
