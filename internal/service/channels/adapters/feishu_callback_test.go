package adapters

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
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
	if request.Channel != channelcontract.ChannelTypeFeishu || request.ChatType != "group" || request.Ref != "oc_group_123" {
		t.Fatalf("飞书路由不正确: %+v", request)
	}
	if request.Content != "检查今天的定时任务发送情况" {
		t.Fatalf("飞书文本不正确: %q", request.Content)
	}
	if request.Delivery == nil || request.Delivery.Channel != channelcontract.ChannelTypeFeishu || request.Delivery.To != "oc_group_123" || request.Delivery.AccountID != "chat_id" {
		t.Fatalf("飞书回投目标不正确: %+v", request.Delivery)
	}
	if request.ReqID != "om_1" || request.RoundID != "evt-1" {
		t.Fatalf("飞书请求 ID 不正确: req=%q round=%q", request.ReqID, request.RoundID)
	}
	if request.Message == nil ||
		request.Message.PlatformMessageID != "om_1" ||
		request.Message.SenderID != "ou_sender" ||
		request.Message.Text != "检查今天的定时任务发送情况" {
		t.Fatalf("飞书消息 envelope 不正确: %+v", request.Message)
	}
}

func TestDecodeFeishuIngressCallbackKeepsP2PChatsSeparate(t *testing.T) {
	payloads := []string{
		`{
			"schema": "2.0",
			"header": {"event_id": "evt-p2p-1", "event_type": "im.message.receive_v1", "app_id": "cli_a"},
			"event": {
				"sender": {"sender_id": {"open_id": "ou_sender_1"}},
				"message": {
					"message_id": "om_p2p_1",
					"chat_id": "oc_p2p_1",
					"chat_type": "p2p",
					"message_type": "text",
					"content": "{\"text\":\"hello\"}"
				}
			}
		}`,
		`{
			"schema": "2.0",
			"header": {"event_id": "evt-p2p-2", "event_type": "im.message.receive_v1", "app_id": "cli_a"},
			"event": {
				"sender": {"sender_id": {"open_id": "ou_sender_2"}},
				"message": {
					"message_id": "om_p2p_2",
					"chat_id": "oc_p2p_2",
					"chat_type": "p2p",
					"message_type": "text",
					"content": "{\"text\":\"hello\"}"
				}
			}
		}`,
	}
	seenRefs := map[string]bool{}
	for _, payload := range payloads {
		callback, err := DecodeFeishuIngressCallback([]byte(payload))
		if err != nil {
			t.Fatalf("解析飞书 p2p 消息失败: %v", err)
		}
		request := callback.Request
		if request == nil {
			t.Fatalf("飞书 p2p 消息应生成 ingress request: %+v", callback)
		}
		if request.ChatType != "dm" || request.Ref == "" {
			t.Fatalf("飞书 p2p session ref 不正确: %+v", request)
		}
		if request.Delivery == nil ||
			request.Delivery.To != request.Ref ||
			request.Delivery.AccountID != "chat_id" {
			t.Fatalf("飞书 p2p 回投目标不正确: %+v", request.Delivery)
		}
		if seenRefs[request.Ref] {
			t.Fatalf("不同飞书 p2p chat 不应复用 session ref: %+v", request)
		}
		seenRefs[request.Ref] = true
	}
}

func TestDecodeFeishuIngressCallbackMessageThreadMetadata(t *testing.T) {
	callback, err := DecodeFeishuIngressCallback([]byte(`{
		"schema": "2.0",
		"header": {
			"event_id": "evt-thread-1",
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
				"message_id": "om_reply_1",
				"root_id": "om_root_1",
				"parent_id": "om_parent_1",
				"thread_id": "omt_thread_1",
				"chat_id": "oc_group_123",
				"chat_type": "topic_group",
				"message_type": "text",
				"content": "{\"text\":\"继续这个话题\"}"
			}
		}
	}`))
	if err != nil {
		t.Fatalf("解析飞书话题消息失败: %v", err)
	}
	if callback.Request == nil {
		t.Fatal("飞书话题消息应生成 ingress request")
	}
	if callback.Request.ThreadID != "omt_thread_1" || callback.Request.ChatType != "group" {
		t.Fatalf("飞书话题路由不正确: %+v", callback.Request)
	}
	if callback.Request.Delivery == nil || callback.Request.Delivery.ThreadID != "om_reply_1" {
		t.Fatalf("飞书话题回投目标应记录当前消息 ID: %+v", callback.Request.Delivery)
	}
	if callback.Request.Message == nil ||
		callback.Request.Message.PlatformMessageID != "om_reply_1" ||
		callback.Request.Message.ThreadID != "omt_thread_1" {
		t.Fatalf("飞书话题消息 envelope 不正确: %+v", callback.Request.Message)
	}
}

func TestDecodeFeishuIngressCallbackReactionCreated(t *testing.T) {
	callback, err := DecodeFeishuIngressCallback([]byte(`{
		"schema": "2.0",
		"header": {
			"event_id": "evt-reaction-1",
			"event_type": "im.message.reaction.created_v1",
			"app_id": "cli_a"
		},
		"event": {
			"message_id": "om_bot_reply_1",
			"chat_id": "oc_group_123",
			"chat_type": "group",
			"reaction_type": {
				"emoji_type": "THUMBSUP"
			},
			"operator_type": "user",
			"user_id": {
				"open_id": "ou_sender"
			},
			"action_time": "1779412618000"
		}
	}`))
	if err != nil {
		t.Fatalf("解析飞书 reaction 事件失败: %v", err)
	}
	if callback.Request == nil {
		t.Fatal("飞书 reaction 应生成 ingress request")
	}
	request := callback.Request
	if request.Content != "[reacted with THUMBSUP to message om_bot_reply_1]" {
		t.Fatalf("飞书 reaction 内容不正确: %q", request.Content)
	}
	if request.ReqID != "om_bot_reply_1:reaction:THUMBSUP:evt-reaction-1" {
		t.Fatalf("飞书 reaction req_id 不正确: %q", request.ReqID)
	}
	if request.Delivery == nil || request.Delivery.ThreadID != "om_bot_reply_1" {
		t.Fatalf("飞书 reaction 回投目标不正确: %+v", request.Delivery)
	}
}

func TestDecodeFeishuIngressCallbackIgnoresTypingReaction(t *testing.T) {
	callback, err := DecodeFeishuIngressCallback([]byte(`{
		"header": {
			"event_type": "im.message.reaction.created_v1"
		},
		"event": {
			"message_id": "om_1",
			"reaction_type": {
				"emoji_type": "Typing"
			},
			"operator_type": "user",
			"user_id": {
				"open_id": "ou_sender"
			}
		}
	}`))
	if err != nil {
		t.Fatalf("解析飞书 typing reaction 失败: %v", err)
	}
	if callback.Request != nil || callback.IgnoredReason != "typing_reaction" {
		t.Fatalf("Typing reaction 应被忽略: %+v", callback)
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
	encryptValue, encrypted, err := FeishuEncryptEnvelope(body)
	if err != nil || !encrypted {
		t.Fatalf("飞书加密 envelope 解析失败: encrypted=%v err=%v", encrypted, err)
	}
	decrypted, err := DecryptFeishuEncryptedPayload(encryptValue, "encrypt-key")
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
	if err := VerifyFeishuCallbackSignature(body, header, "encrypt-key"); err != nil {
		t.Fatalf("飞书签名校验失败: %v", err)
	}
	if err := VerifyFeishuCallbackSignature(body, header, "wrong-key"); err == nil {
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
