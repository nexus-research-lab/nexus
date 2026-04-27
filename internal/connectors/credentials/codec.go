package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

// DecodeKey 解析用于连接器凭据加密的 32 字节 base64 密钥。
func DecodeKey(raw string) ([]byte, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("CONNECTOR_CREDENTIALS_KEY 未配置")
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(key) != 32 {
		return nil, errors.New("CONNECTOR_CREDENTIALS_KEY 必须是 32 字节 base64")
	}
	return key, nil
}

// EncryptPayload 使用 AES-GCM 加密凭据明文。
func EncryptPayload(key []byte, payload []byte) (string, error) {
	block, err := aes.NewCipher(key)
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
	ciphertext := gcm.Seal(nil, nonce, payload, nil)
	encoded := append(nonce, ciphertext...)
	return "v1:" + base64.StdEncoding.EncodeToString(encoded), nil
}

// DecryptPayload 解密由 EncryptPayload 生成的凭据密文。
func DecryptPayload(key []byte, payload string) ([]byte, error) {
	encoded := strings.TrimPrefix(strings.TrimSpace(payload), "v1:")
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("connector credentials payload 格式不正确")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
