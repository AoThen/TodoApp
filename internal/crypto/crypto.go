package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
)

// CryptoManager 持有单个 32 字节 AES-256-GCM 密钥（每设备独立）
type CryptoManager struct {
	key []byte
}

// NewManager 用 32 字节原始密钥创建管理器
func NewManager(key []byte) (*CryptoManager, error) {
	if len(key) != 32 {
		return nil, errors.New("密钥必须为 32 字节 (256 位)")
	}
	clone := make([]byte, len(key))
	copy(clone, key)
	return &CryptoManager{key: clone}, nil
}

// NewManagerFromHex 从 64 字符 hex 密钥创建管理器
func NewManagerFromHex(keyHex string) (*CryptoManager, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, errors.New("密钥必须为合法 hex 字符串")
	}
	return NewManager(key)
}

func (cm *CryptoManager) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(cm.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (cm *CryptoManager) Decrypt(encryptedBase64 string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(cm.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func (cm *CryptoManager) EncryptBytes(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(cm.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

func (cm *CryptoManager) DecryptBytes(encryptedData []byte) ([]byte, error) {
	block, err := aes.NewCipher(cm.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// GenerateKey 生成 32 字节随机密钥（64 字符 hex）
func GenerateKey() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return hex.EncodeToString(raw)
}
