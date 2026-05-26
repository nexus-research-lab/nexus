package channels

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
)

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
	header := http.Header{}
	header.Set("X-Lark-Request-Timestamp", timestamp)
	header.Set("X-Lark-Request-Nonce", nonce)
	header.Set("X-Lark-Signature", feishuCallbackSignature(timestamp, nonce, encryptKey, raw))
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

func TestDecodeFeishuIngressCallbackChallenge(t *testing.T) {
	callback, err := DecodeFeishuIngressCallback([]byte(`{
		"type": "url_verification",
		"token": "verification-token",
		"challenge": "challenge-token"
	}`))
	if err != nil {
		t.Fatalf("解析飞书 URL 校验失败: %v", err)
	}
	if callback.Challenge != "challenge-token" {
		t.Fatalf("challenge 不正确: %+v", callback)
	}
	if callback.Request != nil {
		t.Fatalf("URL 校验不应生成 ingress request: %+v", callback.Request)
	}
	if callback.Token != "verification-token" {
		t.Fatalf("verification token 未解析: %+v", callback)
	}
}

func TestDecodeFeishuIngressCallbackMessage(t *testing.T) {
	callback, err := DecodeFeishuIngressCallback([]byte(`{
		"schema": "2.0",
		"header": {
			"event_id": "evt-1",
			"event_type": "im.message.receive_v1",
			"app_id": "cli_a"
		},
		"event": {
			"sender": {
				"sender_id": {
					"open_id": "ou_sender"
				}
			},
			"message": {
				"message_id": "om_1",
				"chat_id": "oc_group_123",
				"chat_type": "group",
				"message_type": "text",
				"content": "{\"text\":\"检查今天的定时任务发送情况\"}"
			}
		}
	}`))
	if err != nil {
		t.Fatalf("解析飞书消息失败: %v", err)
	}
	if callback.AppID != "cli_a" {
		t.Fatalf("app_id 不正确: %q", callback.AppID)
	}
	if callback.Request == nil {
		t.Fatal("飞书消息应生成 ingress request")
	}
	request := callback.Request
	if request.Channel != ChannelTypeFeishu || request.ChatType != "group" || request.Ref != "oc_group_123" {
		t.Fatalf("飞书路由不正确: %+v", request)
	}
	if request.Content != "检查今天的定时任务发送情况" {
		t.Fatalf("飞书文本不正确: %q", request.Content)
	}
	if request.Delivery == nil || request.Delivery.Channel != ChannelTypeFeishu || request.Delivery.To != "oc_group_123" || request.Delivery.AccountID != "chat_id" {
		t.Fatalf("飞书回投目标不正确: %+v", request.Delivery)
	}
	if request.ReqID != "om_1" || request.RoundID != "evt-1" {
		t.Fatalf("飞书请求 ID 不正确: req=%q round=%q", request.ReqID, request.RoundID)
	}
}

func TestDecryptFeishuCallback(t *testing.T) {
	plain := []byte(`{
		"schema": "2.0",
		"header": {
			"event_id": "evt-1",
			"event_type": "im.message.receive_v1",
			"app_id": "cli_a",
			"token": "verification-token"
		},
		"event": {
			"message": {
				"message_id": "om_1",
				"chat_id": "oc_group_123",
				"chat_type": "group",
				"message_type": "text",
				"content": "{\"text\":\"检查今天的定时任务发送情况\"}"
			}
		}
	}`)
	body := encryptFeishuCallbackForTest(t, "encrypt-key", plain)
	decrypted, err := decryptFeishuCallback(body, "encrypt-key")
	if err != nil {
		t.Fatalf("飞书加密回调解密失败: %v", err)
	}
	callback, err := DecodeFeishuIngressCallback(decrypted)
	if err != nil {
		t.Fatalf("解析解密后的飞书消息失败: %v", err)
	}
	if callback.AppID != "cli_a" || callback.Token != "verification-token" {
		t.Fatalf("解密后的 app/token 不正确: %+v", callback)
	}
	if callback.Request == nil || callback.Request.Content != "检查今天的定时任务发送情况" {
		t.Fatalf("解密后的 ingress request 不正确: %+v", callback.Request)
	}
}

func TestVerifyFeishuCallbackSignature(t *testing.T) {
	body := []byte(`{"encrypt":"cipher"}`)
	header := signedFeishuHeaderForTest(body, "encrypt-key")
	if err := verifyFeishuCallbackSignature(body, header, "encrypt-key"); err != nil {
		t.Fatalf("飞书签名校验失败: %v", err)
	}
	if err := verifyFeishuCallbackSignature(body, header, "wrong-key"); err == nil {
		t.Fatal("错误 encrypt_key 不应通过签名校验")
	}
}

func TestDecodeFeishuIngressCallbackIgnoresBotSender(t *testing.T) {
	callback, err := DecodeFeishuIngressCallback([]byte(`{
		"header": {
			"event_type": "im.message.receive_v1"
		},
		"event": {
			"sender": {
				"sender_type": "app"
			},
			"message": {
				"message_id": "om_bot",
				"chat_id": "oc_group_123",
				"chat_type": "group",
				"message_type": "text",
				"content": "{\"text\":\"机器人自己发送的消息\"}"
			}
		}
	}`))
	if err != nil {
		t.Fatalf("解析飞书机器人消息失败: %v", err)
	}
	if callback.Request != nil || callback.IgnoredReason != "bot_message" {
		t.Fatalf("机器人消息应被忽略: %+v", callback)
	}
}
