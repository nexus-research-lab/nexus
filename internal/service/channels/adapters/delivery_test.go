package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmanagement "github.com/nexus-research-lab/nexus/internal/service/channels/management"
)

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
	if ingress.requests[1].ReqID != "update:11" {
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

func TestChunkedDeliveryPreservesPartialReceipt(t *testing.T) {
	for _, platform := range []string{"telegram", "discord"} {
		t.Run(platform, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if calls > 1 {
					return nil, errors.New("第二段网络中断")
				}
				if platform == "telegram" {
					return jsonResponse(`{"ok":true,"result":{"message_id":42}}`), nil
				}
				return jsonResponse(`{"id":"42"}`), nil
			})}
			var result channelcontract.DeliveryResult
			var err error
			target := channelcontract.DeliveryTarget{Mode: "explicit", Channel: platform, To: "chat"}
			if platform == "telegram" {
				result, err = NewTelegramChannel("test", client).SendDeliveryMessage(t.Context(), target, strings.Repeat("a", 4500))
			} else {
				result, err = NewDiscordChannel("test", client).SendDeliveryMessage(t.Context(), target, strings.Repeat("a", 2400))
			}
			if err == nil || calls != 2 || result.Receipt == nil || result.Receipt.PrimaryPlatformMessageID != "42" {
				t.Fatalf("部分成功回执丢失: %+v, %v", result, err)
			}
		})
	}
}

func TestDeliveryRetriesOnlyExplicitRateLimit(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, 0} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if calls > 1 {
					return jsonResponse(`{"id":"42"}`), nil
				}
				if status == 0 {
					return nil, errors.New("连接中断")
				}
				response := jsonResponse(`{"retry_after":0.001}`)
				response.StatusCode = status
				return response, nil
			})}
			_, err := NewDiscordChannel("test", client).SendDeliveryMessage(t.Context(), channelcontract.DeliveryTarget{Mode: "explicit", Channel: "discord", To: "chat"}, "hello")
			if status == http.StatusTooManyRequests {
				if err != nil || calls != 2 {
					t.Fatalf("限流未按平台延迟恢复: %v %d", err, calls)
				}
			} else if err == nil || calls != 1 {
				t.Fatalf("未知结果被重试: %v %d", err, calls)
			}
		})
	}
	response := jsonResponse(`{"parameters":{"retry_after":60}}`)
	response.StatusCode = 429
	err := channeltransport.ExpectSuccess(response)
	var rejected *channeltransport.HTTPError
	if !errors.As(err, &rejected) || rejected.RetryAfter != time.Minute {
		t.Fatalf("Telegram 限流信息丢失: %v", err)
	}
}

func TestChunkProgressFailureStopsRemainingChunks(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { calls++; return jsonResponse(`{"id":"42"}`), nil })}
	ctx := channelcontract.WithDeliveryProgress(t.Context(), func(result channelcontract.DeliveryResult) error {
		if result.Receipt == nil || result.Receipt.PrimaryPlatformMessageID != "42" {
			t.Fatal("持久化回调未收到分段回执")
		}
		return errors.New("回执落盘失败")
	})
	result, err := NewDiscordChannel("test", client).SendDeliveryMessage(ctx, channelcontract.DeliveryTarget{Mode: "explicit", Channel: "discord", To: "chat"}, strings.Repeat("a", 2400))
	if err == nil || calls != 1 || result.Receipt == nil || result.Receipt.PrimaryPlatformMessageID != "42" {
		t.Fatalf("落盘失败后继续发送或丢失回执: %+v %v %d", result, err, calls)
	}
}
