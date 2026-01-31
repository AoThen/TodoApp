package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

type CryptoManager struct {
	key []byte
}

var globalCM *CryptoManager

func Init() error {
	keyHex := os.Getenv("ENCRYPTION_KEY")
	if keyHex == "" {
		return errors.New("ENCRYPTION_KEY environment variable is required")
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return errors.New("ENCRYPTION_KEY must be a valid hex string")
	}

	if len(key) != 32 {
		return errors.New("ENCRYPTION_KEY must be 32 bytes (256 bits)")
	}

	globalCM = &CryptoManager{key: key}
	return nil
}

func GetManager() *CryptoManager {
	if globalCM == nil {
		panic("crypto: Init() must be called before GetManager()")
	}
	return globalCM
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

func GenerateKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}
