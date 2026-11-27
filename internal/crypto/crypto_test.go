package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"
)

const testKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // 32 字节

func newTestManager(t *testing.T) *CryptoManager {
	t.Helper()
	cm, err := NewManagerFromHex(testKeyHex)
	if err != nil {
		t.Fatalf("创建测试密钥管理器失败: %v", err)
	}
	return cm
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
	other, _ := NewManager([]byte(strings.Repeat("x", 32)))
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

func TestNewManagerValidation(t *testing.T) {
	cases := []struct {
		name    string
		keyHex  string
		wantErr bool
	}{
		{"非法 hex", "zzzz", true},
		{"长度不足", strings.Repeat("ab", 16), true},
		{"合法 32 字节", testKeyHex, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewManagerFromHex(c.keyHex)
			if (err != nil) != c.wantErr {
				t.Errorf("NewManagerFromHex() err = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
	if _, err := NewManager([]byte("short")); err == nil {
		t.Error("非 32 字节密钥应报错")
	}
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

func newClientKeyPair(t *testing.T) (*ecdsa.PrivateKey, []byte, string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return priv, der, base64.StdEncoding.EncodeToString(der)
}

func TestWrapUnwrapDeviceKey(t *testing.T) {
	priv, clientDER, clientPubB64 := newClientKeyPair(t)
	deviceKey := GenerateKey()

	wrapped, serverPub, nonce, err := WrapDeviceKey(clientPubB64, deviceKey, "dev-1", "pair-1")
	if err != nil {
		t.Fatalf("包装失败: %v", err)
	}

	privECDH, err := priv.ECDH()
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnwrapDeviceKey(privECDH, clientDER, serverPub, wrapped, nonce, "dev-1", "pair-1")
	if err != nil {
		t.Fatalf("解包失败: %v", err)
	}
	if got != deviceKey {
		t.Errorf("解包出的设备密钥不一致: want %s, got %s", deviceKey, got)
	}

	// 错误的配对码应导致 AAD 校验失败
	if _, err := UnwrapDeviceKey(privECDH, clientDER, serverPub, wrapped, nonce, "dev-1", "pair-2"); err == nil {
		t.Error("错误配对码解包应当失败")
	}

	// 错误的客户端私钥应导致共享密钥不一致
	otherPriv, _, _ := newClientKeyPair(t)
	otherECDH, _ := otherPriv.ECDH()
	if _, err := UnwrapDeviceKey(otherECDH, clientDER, serverPub, wrapped, nonce, "dev-1", "pair-1"); err == nil {
		t.Error("错误客户端私钥解包应当失败")
	}
}

func TestWrapDeviceKeyInvalidPublicKey(t *testing.T) {
	if _, _, _, err := WrapDeviceKey("not-base64!!", GenerateKey(), "dev", "pair"); err == nil {
		t.Error("非法客户端公钥应当报错")
	}
}

func TestSealOpenWithKEK(t *testing.T) {
	kek := make([]byte, 32)
	for i := range kek {
		kek[i] = byte(i)
	}
	deviceKey := GenerateKey()

	sealed, err := SealWithKEK(kek, deviceKey, "dev-1")
	if err != nil {
		t.Fatalf("封装失败: %v", err)
	}
	got, err := OpenWithKEK(kek, sealed, "dev-1")
	if err != nil {
		t.Fatalf("解封失败: %v", err)
	}
	if got != deviceKey {
		t.Errorf("解封设备密钥不一致: want %s, got %s", deviceKey, got)
	}

	// 错误 device_id（AAD）应失败
	if _, err := OpenWithKEK(kek, sealed, "dev-2"); err == nil {
		t.Error("错误 device_id 解封应当失败")
	}

	// 错误 KEK 应失败
	wrongKEK := make([]byte, 32)
	copy(wrongKEK, kek)
	wrongKEK[0] ^= 0xff
	if _, err := OpenWithKEK(wrongKEK, sealed, "dev-1"); err == nil {
		t.Error("错误 KEK 解封应当失败")
	}
}
