package adapters

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var ErrFeishuCallbackUnauthorized = errors.New("feishu callback verification failed")

func feishuCallbackSignature(timestamp string, nonce string, encryptKey string, body []byte) string {
	hash := sha256.Sum256([]byte(timestamp + nonce + encryptKey + string(body)))
	return fmt.Sprintf("%x", hash[:])
}

func VerifyFeishuCallbackSignature(raw []byte, header http.Header, encryptKey string) error {
	key := strings.TrimSpace(encryptKey)
	if key == "" {
		return nil
	}
	timestamp := strings.TrimSpace(header.Get("X-Lark-Request-Timestamp"))
	nonce := strings.TrimSpace(header.Get("X-Lark-Request-Nonce"))
	signature := strings.ToLower(strings.TrimSpace(header.Get("X-Lark-Signature")))
	if timestamp == "" || nonce == "" || signature == "" {
		return fmt.Errorf("%w: missing feishu signature headers", ErrFeishuCallbackUnauthorized)
	}
	expected := feishuCallbackSignature(timestamp, nonce, key, raw)
	if subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) != 1 {
		return fmt.Errorf("%w: invalid feishu signature", ErrFeishuCallbackUnauthorized)
	}
	return nil
}

func VerifyFeishuCallbackToken(callback FeishuIngressCallback, verificationToken string) error {
	expected := strings.TrimSpace(verificationToken)
	if expected == "" {
		return nil
	}
	actual := strings.TrimSpace(callback.Token)
	if actual == "" {
		return fmt.Errorf("%w: missing feishu verification token", ErrFeishuCallbackUnauthorized)
	}
	if subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return fmt.Errorf("%w: invalid feishu verification token", ErrFeishuCallbackUnauthorized)
	}
	return nil
}

func FeishuEncryptEnvelope(raw []byte) (string, bool, error) {
	var envelope struct {
		Encrypt string `json:"encrypt"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", false, err
	}
	encrypt := strings.TrimSpace(envelope.Encrypt)
	return encrypt, encrypt != "", nil
}

func DecryptFeishuEncryptedPayload(encrypt string, encryptKey string) ([]byte, error) {
	if strings.TrimSpace(encryptKey) == "" {
		return nil, fmt.Errorf("%w: missing feishu encrypt key", ErrFeishuCallbackUnauthorized)
	}
	buf, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encrypt))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid feishu encrypted payload", ErrFeishuCallbackUnauthorized)
	}
	if len(buf) < aes.BlockSize {
		return nil, fmt.Errorf("%w: feishu encrypted payload too short", ErrFeishuCallbackUnauthorized)
	}
	key := sha256.Sum256([]byte(strings.TrimSpace(encryptKey)))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	iv := buf[:aes.BlockSize]
	cipherText := bytes.Clone(buf[aes.BlockSize:])
	if len(cipherText)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("%w: invalid feishu encrypted payload length", ErrFeishuCallbackUnauthorized)
	}
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(cipherText, cipherText)
	start := bytes.IndexByte(cipherText, '{')
	end := bytes.LastIndexByte(cipherText, '}')
	if start < 0 || end < start {
		return nil, fmt.Errorf("%w: decrypted feishu payload is not json", ErrFeishuCallbackUnauthorized)
	}
	return bytes.TrimSpace(cipherText[start : end+1]), nil
}
