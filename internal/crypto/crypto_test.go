package crypto

import (
	"strings"
	"testing"
)

const testKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // 32 字节

func newTestManager(t *testing.T) *CryptoManager {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY", testKeyHex)
	if err := Init(); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	return GetManager()
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	cm := newTestManager(t)
	plaintexts := []string{
		"",
		"hello",
		"中文内容测试",
		"emoji 🔐 test",
		strings.Repeat("a", 10000),
		"line1\nline2\ttab",
	}
	for _, pt := range plaintexts {
		ct, err := cm.Encrypt(pt)
		if err != nil {
			t.Fatalf("Encrypt(%q) 失败: %v", pt, err)
		}
		got, err := cm.Decrypt(ct)
		if err != nil {
			t.Fatalf("Decrypt 失败: %v", err)
		}
		if got != pt {
			t.Errorf("往返不一致: want %q, got %q", pt, got)
		}
	}
}

func TestEncryptNonDeterministic(t *testing.T) {
	cm := newTestManager(t)
	ct1, _ := cm.Encrypt("same plaintext")
	ct2, _ := cm.Encrypt("same plaintext")
	if ct1 == ct2 {
		t.Error("相同明文两次加密产生相同密文（nonce 未随机化）")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	cm := newTestManager(t)
	ct, err := cm.Encrypt("sensitive data")
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(ct)
	// 翻转密文尾部一个比特（GCM tag 区域）
	raw[len(raw)-1] ^= 0x01
	if _, err := cm.Decrypt(string(raw)); err == nil {
		t.Error("篡改后的密文解密应当失败")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	cm := newTestManager(t)
	ct, err := cm.Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}

	other := &CryptoManager{key: []byte(strings.Repeat("x", 32))}
	if _, err := other.Decrypt(ct); err == nil {
		t.Error("错误密钥解密应当失败")
	}
}

func TestBytesRoundTrip(t *testing.T) {
	cm := newTestManager(t)
	data := []byte{0x00, 0x01, 0xff, 0xfe, 'G', 'o'}
	ct, err := cm.EncryptBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := cm.DecryptBytes(ct)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Errorf("字节往返不一致: want %v, got %v", data, got)
	}
}

func TestDecryptInvalidInputs(t *testing.T) {
	cm := newTestManager(t)
	cases := []struct {
		name string
		in   string
	}{
		{"非法 base64", "!!!not-base64!!!"},
		{"过短密文", "AAAA"},
		{"空串", ""},
	}
	for _, c := range cases {
		if _, err := cm.Decrypt(c.in); err == nil {
			t.Errorf("%s: 解密应当失败", c.name)
		}
	}
}

func TestInitValidation(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"未设置", "", true},
		{"非法 hex", "zzzz", true},
		{"长度不足", strings.Repeat("ab", 16), true},
		{"合法 32 字节", testKeyHex, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("ENCRYPTION_KEY", c.key)
			err := Init()
			if (err != nil) != c.wantErr {
				t.Errorf("Init() err = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

func TestGetManagerBeforeInitPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("未初始化时 GetManager 应当 panic")
		}
	}()
	// 重置全局以模拟未初始化
	saved := globalCM
	globalCM = nil
	defer func() { globalCM = saved }()
	GetManager()
}

func TestGenerateKey(t *testing.T) {
	k1 := GenerateKey()
	k2 := GenerateKey()
	if len(k1) != 64 {
		t.Errorf("密钥 hex 长度应为 64, got %d", len(k1))
	}
	if k1 == k2 {
		t.Error("两次生成的密钥不应相同")
	}
}
