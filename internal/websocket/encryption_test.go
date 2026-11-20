package websocket

import (
	"encoding/json"
	"os"
	"testing"

	"todoapp/internal/crypto"
)

const testKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestMain(m *testing.M) {
	// crypto.GetManager() 依赖 crypto.Init()
	os.Setenv("ENCRYPTION_KEY", testKeyHex)
	if err := crypto.Init(); err != nil {
		panic(err)
	}
	m.Run()
}

func TestWebSocketEncryptorDisabledPassthrough(t *testing.T) {
	e := NewWebSocketEncryptor(false, "client-1")
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
	e := NewWebSocketEncryptor(true, "client-1")
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

func TestHandshakeFlow(t *testing.T) {
	e := NewWebSocketEncryptor(true, "client-1")
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
	e := NewWebSocketEncryptor(true, "client-1")

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
	e := NewWebSocketEncryptor(false, "client-3")
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
