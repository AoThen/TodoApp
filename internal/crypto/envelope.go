package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"

	"golang.org/x/crypto/hkdf"
)

// DeviceKeyWrapInfoPrefix 设备密钥包装 HKDF 上下文前缀（三端必须一致）
const DeviceKeyWrapInfoPrefix = "todoapp/device-key-v2"

func concatBytes(parts ...[]byte) []byte {
	total := 0
	for _, p := range parts {
		total += len(p)
	}
	out := make([]byte, 0, total)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// ParsePublicKeySPKI 解析 base64(SPKI DER) 为 ECDH P-256 公钥
func ParsePublicKeySPKI(pubB64 string) (*ecdh.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil {
		return nil, errors.New("客户端公钥格式无效")
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, errors.New("客户端公钥格式无效")
	}
	ec, ok := parsed.(*ecdsa.PublicKey)
	if !ok || ec.Curve != elliptic.P256() {
		return nil, errors.New("客户端公钥必须是 P-256")
	}
	return ec.ECDH()
}

func deriveWrapKey(shared []byte, clientDER, serverDER []byte, deviceID string) ([]byte, error) {
	saltHash := sha256.Sum256(concatBytes(clientDER, serverDER))
	info := []byte(DeviceKeyWrapInfoPrefix + "|" + deviceID)
	kek := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(sha256.New, shared, saltHash[:], info), kek); err != nil {
		return nil, err
	}
	return kek, nil
}

func wrapAAD(clientDER, serverDER []byte, deviceID, pairingKey string) []byte {
	return concatBytes(clientDER, serverDER, []byte(deviceID), []byte(pairingKey))
}

// WrapDeviceKey 用客户端公钥包装设备密钥。
// 返回 wrapped（base64 密文）、serverPublicKey（base64 SPKI）、nonce（base64）。
func WrapDeviceKey(clientPubB64, deviceKeyHex, deviceID, pairingKey string) (wrappedB64, serverPubB64, nonceB64 string, err error) {
	clientPub, err := ParsePublicKeySPKI(clientPubB64)
	if err != nil {
		return "", "", "", err
	}
	clientDER, err := base64.StdEncoding.DecodeString(clientPubB64)
	if err != nil {
		return "", "", "", errors.New("客户端公钥格式无效")
	}
	deviceKey, err := hex.DecodeString(deviceKeyHex)
	if err != nil || len(deviceKey) != 32 {
		return "", "", "", errors.New("设备密钥无效")
	}

	serverPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", "", err
	}
	serverPrivECDH, err := serverPriv.ECDH()
	if err != nil {
		return "", "", "", err
	}
	shared, err := serverPrivECDH.ECDH(clientPub)
	if err != nil {
		return "", "", "", err
	}
	serverDER, err := x509.MarshalPKIXPublicKey(&serverPriv.PublicKey)
	if err != nil {
		return "", "", "", err
	}

	kek, err := deriveWrapKey(shared, clientDER, serverDER, deviceID)
	if err != nil {
		return "", "", "", err
	}

	block, err := aes.NewCipher(kek)
	if err != nil {
		return "", "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", "", err
	}
	ciphertext := gcm.Seal(nil, nonce, deviceKey, wrapAAD(clientDER, serverDER, deviceID, pairingKey))

	return base64.StdEncoding.EncodeToString(ciphertext),
		base64.StdEncoding.EncodeToString(serverDER),
		base64.StdEncoding.EncodeToString(nonce), nil
}

// UnwrapDeviceKey 客户端侧解包（供测试与跨端校验使用）。
// clientPriv 为客户端临时私钥，clientPubDER 为其 SPKI DER。
func UnwrapDeviceKey(clientPriv *ecdh.PrivateKey, clientPubDER []byte, serverPubB64, wrappedB64, nonceB64, deviceID, pairingKey string) (string, error) {
	serverPub, err := ParsePublicKeySPKI(serverPubB64)
	if err != nil {
		return "", err
	}
	serverDER, err := base64.StdEncoding.DecodeString(serverPubB64)
	if err != nil {
		return "", errors.New("服务端公钥格式无效")
	}
	shared, err := clientPriv.ECDH(serverPub)
	if err != nil {
		return "", err
	}
	kek, err := deriveWrapKey(shared, clientPubDER, serverDER, deviceID)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil || len(nonce) != gcm.NonceSize() {
		return "", errors.New("nonce 无效")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(wrappedB64)
	if err != nil {
		return "", errors.New("密文无效")
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, wrapAAD(clientPubDER, serverDER, deviceID, pairingKey))
	if err != nil {
		return "", err
	}
	if len(plaintext) != 32 {
		return "", errors.New("设备密钥长度无效")
	}
	return hex.EncodeToString(plaintext), nil
}

// SealWithKEK 用服务端 KEK 封装设备密钥入库（AAD 绑定 device_id 防行间替换）
func SealWithKEK(kek []byte, deviceKeyHex, deviceID string) (string, error) {
	if len(kek) != 32 {
		return "", errors.New("KEK 必须为 32 字节")
	}
	plaintext, err := hex.DecodeString(deviceKeyHex)
	if err != nil || len(plaintext) != 32 {
		return "", errors.New("设备密钥无效")
	}
	block, err := aes.NewCipher(kek)
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
	sealed := gcm.Seal(nonce, nonce, plaintext, []byte(deviceID))
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// OpenWithKEK 解封入库的设备密钥
func OpenWithKEK(kek []byte, sealedB64, deviceID string) (string, error) {
	if len(kek) != 32 {
		return "", errors.New("KEK 必须为 32 字节")
	}
	raw, err := base64.StdEncoding.DecodeString(sealedB64)
	if err != nil {
		return "", errors.New("设备密钥密文无效")
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("设备密钥密文过短")
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(deviceID))
	if err != nil {
		return "", err
	}
	if len(plaintext) != 32 {
		return "", errors.New("设备密钥长度无效")
	}
	return hex.EncodeToString(plaintext), nil
}
