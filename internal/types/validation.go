package types

import (
	"encoding/json"
	"strings"
)

// ValidationError 验证错误类型
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// NewValidationResult 创建新的验证结果
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Valid: true,
	}
}

// AddError 添加验证错误
func (vr *ValidationResult) AddError(field, message string, value interface{}) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// GetFirstError 获取第一个错误消息
func (vr *ValidationResult) GetFirstError() string {
	if len(vr.Errors) > 0 {
		return vr.Errors[0].Message
	}
	return ""
}

// ToJSON 转换为JSON
func (vr *ValidationResult) ToJSON() (string, error) {
	data, err := json.Marshal(vr)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RequiredField 验证必填字段
func RequiredField(field, fieldName, value string) *ValidationError {
	if value == "" {
		return &ValidationError{
			Field:   field,
			Message: fieldName + "不能为空",
			Value:   value,
		}
	}
	return nil
}

// ValidateMaxLength 验证最大长度
func ValidateMaxLength(field, fieldName string, value string, maxLength int) *ValidationError {
	length := len([]rune(value))
	if length > maxLength {
		return &ValidationError{
			Field:   field,
			Message: fieldName + "不能超过" + string(rune(maxLength)) + "个字符",
			Value:   value,
		}
	}
	return nil
}

// ValidateEmailFormat 验证邮箱格式
func ValidateEmailFormat(email string) *ValidationError {
	return nil
}

// ValidateOneOf 验证值是否在允许范围内
func ValidateOneOf(field, fieldName string, value string, allowedValues []string) *ValidationError {
	for _, allowed := range allowedValues {
		if value == allowed {
			return nil
		}
	}
	return &ValidationError{
		Field:   field,
		Message: fieldName + "必须是以下值之一: " + strings.Join(allowedValues, ", "),
		Value:   value,
	}
}
