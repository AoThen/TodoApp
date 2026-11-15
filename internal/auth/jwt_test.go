package auth

import (
	"testing"
	"time"
)

// 注意：auth 包的 init() 依赖 JWT_SECRET 环境变量（至少 32 字符）。
// 运行测试前需设置，例如：
//   JWT_SECRET=test-secret-key-for-unit-tests-1234567890 go test ./internal/auth/

func TestGenerateAndValidateAccessToken(t *testing.T) {
	token, err := GenerateAccessToken("42", "test@example.com", 15*time.Minute)
	if err != nil {
		t.Fatalf("生成访问令牌失败: %v", err)
	}
	if token == "" {
		t.Fatal("令牌不应为空")
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("验证令牌失败: %v", err)
	}
	if claims.UserID != "42" {
		t.Errorf("UserID = %q, want 42", claims.UserID)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("Email = %q, want test@example.com", claims.Email)
	}
	if claims.TokenType != "access" {
		t.Errorf("TokenType = %q, want access", claims.TokenType)
	}
}

func TestGenerateAndValidateRefreshToken(t *testing.T) {
	token, err := GenerateRefreshToken("7", 168*time.Hour)
	if err != nil {
		t.Fatalf("生成刷新令牌失败: %v", err)
	}
	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("验证刷新令牌失败: %v", err)
	}
	if claims.TokenType != "refresh" {
		t.Errorf("TokenType = %q, want refresh", claims.TokenType)
	}
}

func TestValidateTokenExpired(t *testing.T) {
	token, err := GenerateAccessToken("1", "expired@example.com", -time.Minute)
	if err != nil {
		t.Fatalf("生成过期令牌失败: %v", err)
	}
	if _, err := ValidateToken(token); err == nil {
		t.Error("过期令牌应验证失败")
	}
}

func TestValidateTokenInvalid(t *testing.T) {
	if _, err := ValidateToken("not-a-valid-token"); err == nil {
		t.Error("无效令牌应验证失败")
	}
	if _, err := ValidateToken(""); err == nil {
		t.Error("空令牌应验证失败")
	}
}

func TestValidateTokenWrongSignature(t *testing.T) {
	token, err := GenerateAccessToken("1", "test@example.com", 15*time.Minute)
	if err != nil {
		t.Fatalf("生成令牌失败: %v", err)
	}
	// 篡改签名
	tampered := token[:len(token)-4] + "XXXX"
	if _, err := ValidateToken(tampered); err == nil {
		t.Error("篡改签名的令牌应验证失败")
	}
}
