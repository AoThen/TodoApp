package validator

import (
	"testing"
)

func TestIsValidEmail(t *testing.T) {
	cases := []struct {
		email string
		want  bool
	}{
		{"admin@example.com", true},
		{"user.name+tag@example.co.uk", true},
		{"a@b.co", true}, // TLD 至少 2 个字符
		{"", false},
		{"plainaddress", false},
		{"@missing-local.com", false},
		{"user@", false},
		{"user@.com", false},
		{"user@@example.com", false},
		{"user@exam ple.com", false},
		{"a", false},
	}
	for _, c := range cases {
		if got := IsValidEmail(c.email); got != c.want {
			t.Errorf("IsValidEmail(%q) = %v, want %v", c.email, got, c.want)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		password string
		wantErr  bool
	}{
		{"Abcd1234!", false},
		{"StrongP@ssw0rd", false},
		{"Abc1!", true},          // 5字符，太短
		{"alllowercase1!", true}, // 缺少大写
		{"ALLUPPERCASE1!", true}, // 缺少小写
		{"NoDigitsHere!", true},  // 缺少数字
		{"NoSpecial123", true},   // 缺少特殊字符
		{"", true},               // 空
	}
	for _, c := range cases {
		err := ValidatePassword(c.password)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidatePassword(%q) error = %v, wantErr %v", c.password, err, c.wantErr)
		}
	}
}

func TestSanitizeInput(t *testing.T) {
	if got := SanitizeInput("hello\x00world"); got != "helloworld" {
		t.Errorf("SanitizeInput 应移除控制字符，got %q", got)
	}
	if got := SanitizeInput("正常文本"); got != "正常文本" {
		t.Errorf("SanitizeInput 不应改变正常文本，got %q", got)
	}
}

func TestIsValidTaskTitle(t *testing.T) {
	if !IsValidTaskTitle("任务") {
		t.Error("合法标题应通过")
	}
	if IsValidTaskTitle("") {
		t.Error("空标题应失败")
	}
	long := make([]byte, 201)
	for i := range long {
		long[i] = 'a'
	}
	if IsValidTaskTitle(string(long)) {
		t.Error("超过200字符的标题应失败")
	}
}

func TestIsValidTaskDescription(t *testing.T) {
	if !IsValidTaskDescription("描述") {
		t.Error("合法描述应通过")
	}
	long := make([]byte, 5001)
	for i := range long {
		long[i] = 'a'
	}
	if IsValidTaskDescription(string(long)) {
		t.Error("超过5000字符的描述应失败")
	}
}
