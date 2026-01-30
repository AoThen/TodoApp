package response

import (
	"encoding/json"
	"net/http"
)

// SuccessResponse 发送成功的 JSON 响应
func SuccessResponse(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// ErrorResponse 发送错误 JSON 响应
func ErrorResponse(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

// ValidationErrorResponse 发送带有字段详情的验证错误
func ValidationErrorResponse(w http.ResponseWriter, errors map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   "验证失败",
		"fields":  errors,
	})
}
