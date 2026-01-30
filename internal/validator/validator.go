package validator

import (
	"fmt"
	"regexp"
	"strings"
)

// IsValidEmail 使用正则表达式验证邮箱格式
func IsValidEmail(email string) bool {
	if len(email) < 3 || len(email) > 254 {
		return false
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// SanitizeInput 移除危险字符
func SanitizeInput(input string) string {
	var result strings.Builder
	for i := 0; i < len(input); i++ {
		b := input[i]
		if b >= 32 && b != 127 {
			result.WriteByte(b)
		}
	}
	return result.String()
}

// ValidatePassword 检查密码强度
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("密码至少需要 8 个字符")
	}

	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password)

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return fmt.Errorf("密码必须包含大写字母、小写字母、数字和特殊字符")
	}

	return nil
}

// IsValidUserID 验证用户 ID 格式
func IsValidUserID(userID string) bool {
	if userID == "" {
		return false
	}
	userIDRegex := regexp.MustCompile(`^\d+$`)
	return userIDRegex.MatchString(userID)
}

// IsValidTaskTitle 验证任务标题
func IsValidTaskTitle(title string) bool {
	if len(title) < 1 || len(title) > 200 {
		return false
	}
	return true
}

// IsValidTaskDescription 验证任务描述
func IsValidTaskDescription(description string) bool {
	if len(description) > 5000 {
		return false
	}
	return true
}
