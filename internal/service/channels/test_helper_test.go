package channels

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func ptrString(value string) *string {
	return &value
}

func encryptFeishuCallbackForTest(t *testing.T, encryptKey string, plain []byte) []byte {
	t.Helper()
	key := sha256.Sum256([]byte(encryptKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		t.Fatalf("创建 AES cipher 失败: %v", err)
	}
	iv := []byte("0123456789abcdef")
	padded := pkcs7PadForTest(plain, aes.BlockSize)
	cipherText := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(cipherText, padded)
	payload := append(append([]byte{}, iv...), cipherText...)
	body, err := json.Marshal(map[string]string{"encrypt": base64.StdEncoding.EncodeToString(payload)})
	if err != nil {
		t.Fatalf("编码飞书加密测试 payload 失败: %v", err)
	}
	return body
}

func signedFeishuHeaderForTest(raw []byte, encryptKey string) http.Header {
	timestamp := "1779412618"
	nonce := "nonce-1"
	hash := sha256.Sum256([]byte(timestamp + nonce + encryptKey + string(raw)))
	header := http.Header{}
	header.Set("X-Lark-Request-Timestamp", timestamp)
	header.Set("X-Lark-Request-Nonce", nonce)
	header.Set("X-Lark-Signature", fmt.Sprintf("%x", hash[:]))
	return header
}

func pkcs7PadForTest(raw []byte, blockSize int) []byte {
	padding := blockSize - len(raw)%blockSize
	padded := make([]byte, 0, len(raw)+padding)
	padded = append(padded, raw...)
	for i := 0; i < padding; i++ {
		padded = append(padded, byte(padding))
	}
	return padded
}
