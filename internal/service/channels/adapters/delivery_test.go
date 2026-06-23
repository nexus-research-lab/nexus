package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmanagement "github.com/nexus-research-lab/nexus/internal/service/channels/management"
)

func TestDiscordChannelSendDeliveryMessage(t *testing.T) {
	requests := make([]*http.Request, 0)
	payloads := make([]map[string]any, 0)
	channel := NewDiscordChannel("token-1", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request)
			var payload map[string]any
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("解析 Discord 请求失败: %w", err)
			}
			payloads = append(payloads, payload)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.WithBaseURL("https://discord.test/api/v10")

	text := strings.Repeat("a", 2400)
	if _, err := channel.SendDeliveryMessage(context.Background(), channelcontract.DeliveryTarget{
		Mode:    channelcontract.DeliveryModeExplicit,
		Channel: channelcontract.ChannelTypeDiscord,
		To:      "123456",
	}, text); err != nil {
		t.Fatalf("Discord 发送失败: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("期望分片发送 2 次，实际 %d", len(requests))
	}
	if got := requests[0].Header.Get("Authorization"); got != "Bot token-1" {
		t.Fatalf("Authorization 头不正确: %s", got)
	}
	if !strings.HasSuffix(requests[0].URL.Path, "/channels/123456/messages") {
		t.Fatalf("Discord 路径不正确: %s", requests[0].URL.Path)
	}
	allowedMentions, ok := payloads[0]["allowed_mentions"].(map[string]any)
	if !ok {
		t.Fatalf("Discord payload 应禁用 mention 解析: %+v", payloads[0])
	}
	parseValues, ok := allowedMentions["parse"].([]any)
	if !ok || len(parseValues) != 0 {
		t.Fatalf("Discord allowed_mentions.parse 应为空: %+v", allowedMentions)
	}
}

func TestDiscordChannelSendDeliveryTyping(t *testing.T) {
	requests := make([]*http.Request, 0)
	channel := NewDiscordChannel("token-1", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request)
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader(``)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.WithBaseURL("https://discord.test/api/v10")

	if err := channel.SendDeliveryTyping(context.Background(), channelcontract.DeliveryTarget{
		Mode:     channelcontract.DeliveryModeExplicit,
		Channel:  channelcontract.ChannelTypeDiscord,
		To:       "channel-1",
		ThreadID: "thread-1",
	}, false); err != nil {
		t.Fatalf("Discord typing stop 应静默忽略: %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("Discord typing stop 不应请求 API，实际 %d", len(requests))
	}

	if err := channel.SendDeliveryTyping(context.Background(), channelcontract.DeliveryTarget{
		Mode:     channelcontract.DeliveryModeExplicit,
		Channel:  channelcontract.ChannelTypeDiscord,
		To:       "channel-1",
		ThreadID: "thread-1",
	}, true); err != nil {
		t.Fatalf("Discord typing start 失败: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("期望 typing 请求 1 次，实际 %d", len(requests))
	}
	if requests[0].Method != http.MethodPost || !strings.HasSuffix(requests[0].URL.Path, "/channels/thread-1/typing") {
		t.Fatalf("Discord typing 路径不正确: %s %s", requests[0].Method, requests[0].URL.Path)
	}
	if got := requests[0].Header.Get("Authorization"); got != "Bot token-1" {
		t.Fatalf("Discord typing Authorization 不正确: %s", got)
	}
}

func TestTelegramChannelSendDeliveryMessage(t *testing.T) {
	requests := make([]*http.Request, 0)
	var payload map[string]any
	channel := NewTelegramChannel("token-2", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request)
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("解析 Telegram 请求失败: %w", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.WithBaseURL("https://telegram.test")

	if _, err := channel.SendDeliveryMessage(context.Background(), channelcontract.DeliveryTarget{
		Mode:     channelcontract.DeliveryModeExplicit,
		Channel:  channelcontract.ChannelTypeTelegram,
		To:       "-1001",
		ThreadID: "12",
	}, "hello"); err != nil {
		t.Fatalf("Telegram 发送失败: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("期望发送 1 次，实际 %d", len(requests))
	}
	if !strings.HasSuffix(requests[0].URL.Path, "/bottoken-2/sendMessage") {
		t.Fatalf("Telegram 路径不正确: %s", requests[0].URL.Path)
	}
	if payload["chat_id"] != "-1001" || payload["message_thread_id"] != float64(12) {
		t.Fatalf("Telegram topic payload 不正确: %+v", payload)
	}
	if payload["disable_web_page_preview"] != true {
		t.Fatalf("Telegram 应关闭链接预览: %+v", payload)
	}
}

func TestTelegramChannelSendDeliveryMessageReturnsReceipt(t *testing.T) {
	channel := NewTelegramChannel("token-2", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":42}}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.WithBaseURL("https://telegram.test")

	result, err := channel.SendDeliveryMessage(context.Background(), channelcontract.DeliveryTarget{
		Mode:     channelcontract.DeliveryModeExplicit,
		Channel:  channelcontract.ChannelTypeTelegram,
		To:       "-1001",
		ThreadID: "12",
	}, "hello")
	if err != nil {
		t.Fatalf("Telegram receipt 发送失败: %v", err)
	}
	receipt := result.Receipt
	if receipt == nil || receipt.PrimaryPlatformMessageID != "42" {
		t.Fatalf("Telegram receipt 未记录 message_id: %+v", receipt)
	}
	if receipt.Channel != channelcontract.ChannelTypeTelegram || receipt.Target != "-1001" || receipt.ThreadID != "12" {
		t.Fatalf("Telegram receipt 目标信息不正确: %+v", receipt)
	}
}

func TestTelegramChannelSendDeliveryTyping(t *testing.T) {
	requests := make([]*http.Request, 0)
	var payload map[string]any
	channel := NewTelegramChannel("token-2", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request)
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("解析 Telegram typing 请求失败: %w", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.WithBaseURL("https://telegram.test")

	if err := channel.SendDeliveryTyping(context.Background(), channelcontract.DeliveryTarget{
		Mode:     channelcontract.DeliveryModeExplicit,
		Channel:  channelcontract.ChannelTypeTelegram,
		To:       "-1001",
		ThreadID: "12",
	}, false); err != nil {
		t.Fatalf("Telegram typing stop 应静默忽略: %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("Telegram typing stop 不应请求 API，实际 %d", len(requests))
	}

	if err := channel.SendDeliveryTyping(context.Background(), channelcontract.DeliveryTarget{
		Mode:     channelcontract.DeliveryModeExplicit,
		Channel:  channelcontract.ChannelTypeTelegram,
		To:       "-1001",
		ThreadID: "12",
	}, true); err != nil {
		t.Fatalf("Telegram typing start 失败: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("期望 typing 请求 1 次，实际 %d", len(requests))
	}
	if !strings.HasSuffix(requests[0].URL.Path, "/bottoken-2/sendChatAction") {
		t.Fatalf("Telegram typing 路径不正确: %s", requests[0].URL.Path)
	}
	if payload["chat_id"] != "-1001" || payload["action"] != "typing" || payload["message_thread_id"] != float64(12) {
		t.Fatalf("Telegram typing payload 不正确: %+v", payload)
	}
}

func TestTelegramChannelSendDeliveryGeneralTopicHandling(t *testing.T) {
	var messagePayload map[string]any
	var typingPayload map[string]any
	requests := make([]*http.Request, 0, 2)
	channel := NewTelegramChannel("token-2", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request)
			var payload map[string]any
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("解析 Telegram 请求失败: %w", err)
			}
			if strings.HasSuffix(request.URL.Path, "/sendMessage") {
				messagePayload = payload
			}
			if strings.HasSuffix(request.URL.Path, "/sendChatAction") {
				typingPayload = payload
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.WithBaseURL("https://telegram.test")
	target := channelcontract.DeliveryTarget{
		Mode:     channelcontract.DeliveryModeExplicit,
		Channel:  channelcontract.ChannelTypeTelegram,
		To:       "-1001",
		ThreadID: "1",
	}

	if _, err := channel.SendDeliveryMessage(context.Background(), target, "hello"); err != nil {
		t.Fatalf("Telegram General topic 发送失败: %v", err)
	}
	if err := channel.SendDeliveryTyping(context.Background(), target, true); err != nil {
		t.Fatalf("Telegram General topic typing 失败: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("期望 Telegram 请求 2 次，实际 %d", len(requests))
	}
	if _, ok := messagePayload["message_thread_id"]; ok {
		t.Fatalf("Telegram sendMessage 不应携带 General topic thread_id=1: %+v", messagePayload)
	}
	if typingPayload["message_thread_id"] != float64(1) {
		t.Fatalf("Telegram sendChatAction 应携带 General topic thread_id=1: %+v", typingPayload)
	}
}

func TestTelegramFetchUpdatesSubscribesEditedMessages(t *testing.T) {
	var payload map[string]any
	channel := NewTelegramChannel("token-2", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("解析 Telegram getUpdates 请求失败: %w", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"ok": true,
					"result": [{
						"update_id": 4,
						"edited_message": {
							"message_id": 9,
							"text": "edited",
							"from": {"id": 8, "is_bot": false},
							"chat": {"id": 7, "type": "private"}
						}
					}]
				}`)),
				Header: make(http.Header),
			}, nil
		}),
	})
	channel.WithBaseURL("https://telegram.test")

	updates, nextOffset, err := channel.fetchUpdates(context.Background(), 3)
	if err != nil {
		t.Fatalf("Telegram getUpdates 失败: %v", err)
	}
	if len(updates) != 1 || updates[0].EditedMessage == nil || nextOffset != 5 {
		t.Fatalf("Telegram edited update 解析不正确: updates=%+v next=%d", updates, nextOffset)
	}
	allowed, ok := payload["allowed_updates"].([]any)
	if !ok {
		t.Fatalf("Telegram allowed_updates 未发送: %+v", payload)
	}
	foundEdited := false
	for _, item := range allowed {
		if item == "edited_message" {
			foundEdited = true
			break
		}
	}
	if !foundEdited {
		t.Fatalf("Telegram allowed_updates 应包含 edited_message: %+v", allowed)
	}
}

func TestTelegramFetchUpdatesRedactsBotTokenInErrors(t *testing.T) {
	token := "123456:secret-token"
	channel := NewTelegramChannel(token, &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("boom %s", request.URL.String())
		}),
	})
	channel.WithBaseURL("https://telegram.test")

	_, _, err := channel.fetchUpdates(context.Background(), 0)
	if err == nil {
		t.Fatal("Telegram getUpdates 应返回错误")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("Telegram 错误不应包含 bot token: %s", err)
	}
	if !strings.Contains(err.Error(), "bot<redacted>") {
		t.Fatalf("Telegram 错误应标记 token 已脱敏: %s", err)
	}
}

func TestTelegramChannelHandleEditedUpdateUsesDistinctReqID(t *testing.T) {
	channel := NewTelegramChannel("token-2", nil)
	ingress := &recordingIngressAcceptor{}
	channel.SetIngress(ingress)

	channel.handleUpdate(context.Background(), telegramUpdate{
		UpdateID: 10,
		Message: &telegramMessage{
			MessageID: 9,
			Text:      "original",
			From:      &telegramUser{ID: 8},
			Chat:      telegramChat{ID: 7, Type: "private"},
		},
	})
	channel.handleUpdate(context.Background(), telegramUpdate{
		UpdateID: 11,
		EditedMessage: &telegramMessage{
			MessageID: 9,
			Text:      "edited",
			From:      &telegramUser{ID: 8},
			Chat:      telegramChat{ID: 7, Type: "private"},
		},
	})

	if len(ingress.requests) != 2 {
		t.Fatalf("Telegram 原消息和编辑事件都应进入 ingress: %+v", ingress.requests)
	}
	if ingress.requests[0].ReqID == ingress.requests[1].ReqID {
		t.Fatalf("Telegram 编辑事件不应复用原消息 req_id: %+v", ingress.requests)
	}
	if ingress.requests[1].ReqID != "9:edited:11" {
		t.Fatalf("Telegram 编辑事件 req_id 不正确: %q", ingress.requests[1].ReqID)
	}
	if ingress.requests[1].Content != "edited" || !ingress.requests[1].Message.Edited {
		t.Fatalf("Telegram 编辑事件内容未保留: %+v", ingress.requests[1])
	}
}

func TestTelegramChannelHandleUpdateSendsPairingApprovalNotice(t *testing.T) {
	var outboundRequests int
	var outboundPayload map[string]any
	channel := NewTelegramChannel("token-2", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			outboundRequests++
			if !strings.HasSuffix(request.URL.Path, "/bottoken-2/sendMessage") {
				t.Fatalf("待配对提醒应调用 Telegram sendMessage，实际 path=%s", request.URL.Path)
			}
			if err := json.NewDecoder(request.Body).Decode(&outboundPayload); err != nil {
				t.Fatalf("解析 Telegram 待配对提醒失败: %v", err)
			}
			return jsonResponse(`{"ok":true,"result":{"message_id":42}}`), nil
		}),
	})
	channel.WithBaseURL("https://telegram.test")
	ingress := &recordingIngressAcceptor{err: &channelmanagement.PairingApprovalError{
		PairingID: "pair_pending_1",
		Message:   "IM 对象尚未配对授权，请先在配对控制台批准",
	}}
	channel.SetIngress(ingress)

	channel.handleUpdate(context.Background(), telegramUpdate{
		Message: &telegramMessage{
			MessageID: 8,
			Text:      "hello",
			From:      &telegramUser{ID: 7},
			Chat:      telegramChat{ID: 7, Type: "private"},
		},
	})

	if len(ingress.requests) != 1 {
		t.Fatalf("Telegram 消息未进入 ingress: %+v", ingress.requests)
	}
	if outboundRequests != 1 {
		t.Fatalf("待配对授权应回发配对提醒，实际请求数: %d", outboundRequests)
	}
	text := fmt.Sprint(outboundPayload["text"])
	if !strings.Contains(text, "配对控制台") || !strings.Contains(text, "pair_pending_1") {
		t.Fatalf("待配对提醒文案不正确: %q", text)
	}
	if strings.Contains(text, "消息处理失败") {
		t.Fatalf("待配对提醒不应伪装成处理失败: %q", text)
	}
}

func TestFeishuChannelSendDeliveryMessage(t *testing.T) {
	var tokenRequests int
	var messagePayload map[string]string
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			tokenRequests++
			var payload map[string]string
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("解析 token 请求失败: %w", err)
			}
			if payload["app_id"] != "cli_test" || payload["app_secret"] != "secret_test" {
				return nil, fmt.Errorf("token 请求凭据不正确: %+v", payload)
			}
			return jsonResponse(`{"code":0,"tenant_access_token":"tenant-token","expire":7200}`), nil
		case "/open-apis/im/v1/messages":
			if request.URL.Query().Get("receive_id_type") != "chat_id" {
				return nil, fmt.Errorf("receive_id_type 不正确: %s", request.URL.RawQuery)
			}
			if request.Header.Get("Authorization") != "Bearer tenant-token" {
				return nil, fmt.Errorf("Authorization 不正确: %s", request.Header.Get("Authorization"))
			}
			if err := json.NewDecoder(request.Body).Decode(&messagePayload); err != nil {
				return nil, fmt.Errorf("解析消息请求失败: %w", err)
			}
			return jsonResponse(`{"code":0,"msg":"ok"}`), nil
		default:
			return nil, fmt.Errorf("未知飞书请求路径: %s", request.URL.Path)
		}
	})}

	channel := NewFeishuChannel("cli_test", "secret_test", client).WithConnectionMode("webhook")
	channel.WithBaseURL("https://feishu.test")
	if err := channel.Start(context.Background()); err != nil {
		t.Fatalf("飞书通道启动失败: %v", err)
	}
	if _, err := channel.SendDeliveryMessage(context.Background(), channelcontract.DeliveryTarget{
		Mode:    channelcontract.DeliveryModeExplicit,
		Channel: channelcontract.ChannelTypeFeishu,
		To:      "oc_group_123",
	}, "今日新闻摘要"); err != nil {
		t.Fatalf("飞书发送失败: %v", err)
	}
	if tokenRequests != 1 {
		t.Fatalf("token 请求次数不正确: %d", tokenRequests)
	}
	if messagePayload["receive_id"] != "oc_group_123" || messagePayload["msg_type"] != "text" {
		t.Fatalf("飞书消息请求不正确: %+v", messagePayload)
	}
	var content map[string]string
	if err := json.Unmarshal([]byte(messagePayload["content"]), &content); err != nil {
		t.Fatalf("解析飞书消息 content 失败: %v", err)
	}
	if content["text"] != "今日新闻摘要" {
		t.Fatalf("飞书消息正文不正确: %+v", content)
	}
}
