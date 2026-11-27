package websocket

import (
	"encoding/json"
	"testing"

	"todoapp/internal/crypto"
)

const testKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func testManager(t *testing.T) *crypto.CryptoManager {
	t.Helper()
	cm, err := crypto.NewManagerFromHex(testKeyHex)
	if err != nil {
		t.Fatalf("创建测试密钥管理器失败: %v", err)
	}
	return cm
}

func TestWebSocketEncryptorDisabledPassthrough(t *testing.T) {
	e := NewWebSocketEncryptor(false, "client-1", nil)
	if !e.IsReady() {
		t.Error("不加密模式应直接就绪")
	}
	msg := []byte("plain message")
	out, err := e.EncryptMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(msg) {
		t.Error("未加密模式应原样透传")
	}
	back, err := e.DecryptMessage(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(back) != string(msg) {
		t.Error("未加密模式解密应原样透传")
	}
}

func TestWebSocketEncryptorRoundTrip(t *testing.T) {
	e := NewWebSocketEncryptor(true, "client-1", testManager(t))
	if e.IsReady() {
		t.Error("加密模式握手前不应就绪")
	}
	msg := []byte("机密消息 secret-123")
	ct, err := e.EncryptMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	if string(ct) == string(msg) {
		t.Error("加密后不应等于明文")
	}
	pt, err := e.DecryptMessage(ct)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt) != string(msg) {
		t.Errorf("加解密往返不一致: %q", pt)
	}

	// 篡改密文应失败
	tampered := make([]byte, len(ct))
	copy(tampered, ct)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := e.DecryptMessage(tampered); err == nil {
		t.Error("篡改后的密文解密应当失败")
	}
}

func TestWebSocketEncryptorEnabledWithoutKeyFails(t *testing.T) {
	e := NewWebSocketEncryptor(true, "client-1", nil)
	if _, err := e.EncryptMessage([]byte("x")); err == nil {
		t.Error("加密模式缺少密钥时应当失败（fail-closed）")
	}
}

func TestHandshakeFlow(t *testing.T) {
	e := NewWebSocketEncryptor(true, "client-1", testManager(t))
	resp, err := e.CreateHandshakeResponse()
	if err != nil {
		t.Fatal(err)
	}
	if !resp.EncryptionEnabled {
		t.Error("加密模式握手响应应标记 encryption_enabled")
	}
	if resp.ServerNonce == "" {
		t.Error("加密模式应生成 server_nonce")
	}
	if resp.Type != "handshake" {
		t.Errorf("握手响应类型错误: %q", resp.Type)
	}

	data, _ := json.Marshal(HandshakeReq{Type: "handshake", Encryption: true})
	if err := e.ProcessHandshake(data); err != nil {
		t.Fatalf("合法握手应成功: %v", err)
	}
	if !e.IsReady() {
		t.Error("握手后应就绪")
	}
}

func TestHandshakeFailures(t *testing.T) {
	e := NewWebSocketEncryptor(true, "client-1", testManager(t))

	// 服务端要求加密但客户端不支持
	bad, _ := json.Marshal(HandshakeReq{Type: "handshake", Encryption: false})
	if err := e.ProcessHandshake(bad); err == nil {
		t.Error("客户端不支持加密时握手应失败")
	}

	// 消息类型错误
	wrongType, _ := json.Marshal(HandshakeReq{Type: "ping", Encryption: true})
	if err := e.ProcessHandshake(wrongType); err == nil {
		t.Error("非 handshake 类型应失败")
	}

	// 非法 JSON
	if err := e.ProcessHandshake([]byte("not json")); err == nil {
		t.Error("非法 JSON 应失败")
	}
}

func TestCreateHandshakeResponseDisabled(t *testing.T) {
	e := NewWebSocketEncryptor(false, "client-3", nil)
	resp, err := e.CreateHandshakeResponse()
	if err != nil {
		t.Fatal(err)
	}
	if resp.EncryptionEnabled {
		t.Error("未加密模式不应标记 encryption_enabled")
	}
	if resp.ServerNonce != "" {
		t.Error("未加密模式不应生成 nonce")
	}
}
