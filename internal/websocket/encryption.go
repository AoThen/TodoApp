package websocket

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"

	"todoapp/internal/crypto"
)

// WebSocketEncryptor WebSocket加密管理器
type WebSocketEncryptor struct {
	cm       *crypto.CryptoManager
	enabled  bool
	ready    bool
	clientID string
}

// HandshakeReq 握手请求
type HandshakeReq struct {
	Type        string `json:"type"`
	Encryption  bool   `json:"encryption"`
	ClientNonce string `json:"client_nonce,omitempty"`
}

// HandshakeResp 握手响应
type HandshakeResp struct {
	Type              string `json:"type"`
	EncryptionEnabled bool   `json:"encryption_enabled"`
	ServerNonce       string `json:"server_nonce,omitempty"`
	Timestamp         string `json:"timestamp"`
}

// NewWebSocketEncryptor 创建加密器
func NewWebSocketEncryptor(enabled bool, clientID string) *WebSocketEncryptor {
	return &WebSocketEncryptor{
		cm:       crypto.GetManager(),
		enabled:  enabled,
		clientID: clientID,
		ready:    !enabled, // 不加密模式下直接就绪
	}
}

// EncryptMessage 加密消息
func (we *WebSocketEncryptor) EncryptMessage(msg []byte) ([]byte, error) {
	if !we.enabled {
		return msg, nil
	}

	encryptedBytes, err := we.cm.EncryptBytes(msg)
	if err != nil {
		return nil, err
	}

	return encryptedBytes, nil
}

// DecryptMessage 解密消息
func (we *WebSocketEncryptor) DecryptMessage(encryptedData []byte) ([]byte, error) {
	if !we.enabled {
		return encryptedData, nil
	}

	decrypted, err := we.cm.DecryptBytes(encryptedData)
	if err != nil {
		return nil, err
	}

	return decrypted, nil
}

// CreateHandshakeResponse 创建握手响应
func (we *WebSocketEncryptor) CreateHandshakeResponse() (HandshakeResp, error) {
	resp := HandshakeResp{
		Type:              "handshake",
		EncryptionEnabled: we.enabled,
	}

	if we.enabled {
		// 生成服务器nonce用于验证
		serverNonceBytes := make([]byte, 12)
		if _, err := rand.Read(serverNonceBytes); err != nil {
			return resp, err
		}
		resp.ServerNonce = base64.StdEncoding.EncodeToString(serverNonceBytes)
	}

	return resp, nil
}

// ProcessHandshake 处理客户端握手
func (we *WebSocketEncryptor) ProcessHandshake(data []byte) error {
	var req HandshakeReq
	if err := json.Unmarshal(data, &req); err != nil {
		return errors.New("invalid handshake format")
	}

	if req.Type != "handshake" {
		return errors.New("expected handshake message")
	}

	// 加密模式验证
	if we.enabled && !req.Encryption {
		return errors.New("server requires encryption, client does not support it")
	}

	we.ready = true
	return nil
}

// IsReady 加密通道是否就绪
func (we *WebSocketEncryptor) IsReady() bool {
	return we.ready
}
