// adapters_test.go 覆盖渠道适配器夹具、投递、Telegram 与企微 Socket 测试。
package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmanagement "github.com/nexus-research-lab/nexus/internal/service/channels/management"
	channeltransport "github.com/nexus-research-lab/nexus/internal/service/channels/transport"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type recordingIngressAcceptor struct {
	requests []channelcontract.IngressRequest
	err      error
}

func (r *recordingIngressAcceptor) Accept(
	_ context.Context,
	request channelcontract.IngressRequest,
) (*channelcontract.IngressResult, error) {
	r.requests = append(r.requests, request)
	if r.err != nil {
		return nil, r.err
	}
	return &channelcontract.IngressResult{
		Channel: request.Channel,
		AgentID: request.AgentID,
		ReqID:   request.ReqID,
	}, nil
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
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

type telegramPollingIngress func(context.Context, channelcontract.IngressRequest) (*channelcontract.IngressResult, error)

func (fn telegramPollingIngress) Accept(ctx context.Context, request channelcontract.IngressRequest) (*channelcontract.IngressResult, error) {
	return fn(ctx, request)
}

func TestTelegramPollingFailureDoesNotBlockOtherChats(t *testing.T) {
	for _, test := range []struct {
		name                             string
		retryable, recovers, noticeFails bool
		attempts                         int
		offsets                          string
	}{
		{name: "准备阶段短暂故障", retryable: true, recovers: true, attempts: 2, offsets: "[0 0 102]"},
		{name: "准备阶段持续故障", retryable: true, attempts: 3, offsets: "[0 0 0 102]"},
		{name: "结果未知不重跑", attempts: 1, offsets: "[0 102]"},
		{name: "通知失败不回退", noticeFails: true, attempts: 1, offsets: "[0 102]"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
			defer cancel()
			attempts, healthy, notices := 0, 0, 0
			offsets := []int{}
			rounds := []string{}
			transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if strings.HasSuffix(r.URL.Path, "/sendMessage") {
					notices++
					if body["chat_id"] != "-10" || body["message_thread_id"] != float64(77) {
						t.Errorf("错误通知发往其他会话: %+v", body)
					}
					text, _ := body["text"].(string)
					if strings.Contains(text, "provider_secret") || !strings.Contains(text, "42") {
						t.Errorf("通知泄露错误或缺少消息标识: %s", text)
					}
					if test.retryable && !strings.Contains(text, "本次接收失败") {
						t.Errorf("准备失败被当成未知执行: %s", text)
					}
					if !test.retryable && !strings.Contains(text, "未能确认") {
						t.Errorf("未知结果被当成确定失败: %s", text)
					}
					if test.noticeFails {
						response := jsonResponse(`{"ok":false}`)
						response.StatusCode = http.StatusInternalServerError
						return response, nil
					}
					return jsonResponse(`{"ok":true,"result":{"message_id":90}}`), nil
				}
				offset := int(body["offset"].(float64))
				offsets = append(offsets, offset)
				if offset == 102 {
					cancel()
					return nil, context.Canceled
				}
				return jsonResponse(`{"ok":true,"result":[{"update_id":100,"message":{"message_id":42,"text":"bad","message_thread_id":77,"from":{"id":7},"chat":{"id":-10,"type":"supergroup"}}},{"update_id":101,"message":{"message_id":43,"text":"good","from":{"id":8},"chat":{"id":9,"type":"private"}}}]}`), nil
			})
			channel := NewTelegramChannel("test", &http.Client{Transport: transport})
			channel.pollRetryDelay = time.Millisecond
			channel.SetIngress(telegramPollingIngress(func(_ context.Context, request channelcontract.IngressRequest) (*channelcontract.IngressResult, error) {
				if request.Content == "good" {
					healthy++
					return &channelcontract.IngressResult{}, nil
				}
				attempts++
				rounds = append(rounds, request.RoundID)
				if request.ReqID != "update:100" {
					t.Errorf("事件身份变化: %s", request.ReqID)
				}
				if test.recovers && attempts == 2 {
					return &channelcontract.IngressResult{}, nil
				}
				err := errors.New("provider_secret")
				if test.retryable {
					err = &channelcontract.RetryableIngressError{Err: err}
				}
				return nil, err
			}))
			channel.wg.Add(1)
			channel.pollUpdates(ctx)
			if attempts != test.attempts || healthy != 1 || fmt.Sprint(offsets) != test.offsets {
				t.Fatalf("attempts=%d healthy=%d offsets=%v", attempts, healthy, offsets)
			}
			expectedNotices := 1
			if test.recovers {
				expectedNotices = 0
			}
			if notices != expectedNotices {
				t.Fatalf("通知次数=%d", notices)
			}
			for _, round := range rounds {
				if round != "telegram_100" {
					t.Fatalf("轮次变化: %v", rounds)
				}
			}
		})
	}
}

func TestTelegramPollingCancellationDoesNotAcknowledgeUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	polls := 0
	channel := NewTelegramChannel("test", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		polls++
		if polls != 1 || !strings.HasSuffix(r.URL.Path, "/getUpdates") {
			t.Errorf("取消后继续确认或发送通知: %s", r.URL.Path)
		}
		return jsonResponse(`{"ok":true,"result":[{"update_id":100,"message":{"message_id":42,"text":"hello","from":{"id":7},"chat":{"id":8,"type":"private"}}}]}`), nil
	})})
	channel.SetIngress(telegramPollingIngress(func(context.Context, channelcontract.IngressRequest) (*channelcontract.IngressResult, error) {
		cancel()
		return nil, context.Canceled
	}))
	channel.wg.Add(1)
	channel.pollUpdates(ctx)
}

type recordingWeComBotSocket struct {
	mu        sync.Mutex
	deadlines []time.Time
	writes    []any
	closed    chan struct{}
	closeOnce sync.Once
}

func newRecordingWeComBotSocket() *recordingWeComBotSocket {
	return &recordingWeComBotSocket{closed: make(chan struct{})}
}

func (s *recordingWeComBotSocket) ReadMessage() (int, []byte, error) {
	<-s.closed
	return 0, nil, errors.New("socket closed")
}

func (s *recordingWeComBotSocket) SetWriteDeadline(deadline time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deadlines = append(s.deadlines, deadline)
	return nil
}

func (s *recordingWeComBotSocket) WriteJSON(value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, value)
	return nil
}

func (s *recordingWeComBotSocket) Close() error {
	s.closeOnce.Do(func() { close(s.closed) })
	return nil
}

func TestWeComBotWriteFrameUsesContextBoundedDeadline(t *testing.T) {
	channel := NewWeComBotChannel("bot-1", "secret-1")
	socket := newRecordingWeComBotSocket()
	channel.setSocket(socket, true)
	channel.writeTimeout = time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	ctxDeadline, _ := ctx.Deadline()

	if err := channel.writeFrame(ctx, weComBotCommandFrame{Cmd: weComBotPingCommand}, true); err != nil {
		t.Fatalf("写企业微信帧失败: %v", err)
	}
	socket.mu.Lock()
	deadlines := append([]time.Time(nil), socket.deadlines...)
	socket.mu.Unlock()
	if len(deadlines) != 2 || deadlines[0].IsZero() || !deadlines[1].IsZero() {
		t.Fatalf("企业微信写截止时间未设置并复位: %+v", deadlines)
	}
	if deadlines[0].After(ctxDeadline) {
		t.Fatalf("企业微信写截止时间越过请求 deadline: got=%v context=%v", deadlines[0], ctxDeadline)
	}
}

func TestWeComBotPingLoopClosesSocketAfterMissedPongs(t *testing.T) {
	channel := NewWeComBotChannel("bot-1", "secret-1")
	socket := newRecordingWeComBotSocket()
	channel.setSocket(socket, true)
	channel.pingInterval = 5 * time.Millisecond
	channel.maxMissedPongs = 2
	channel.writeTimeout = 50 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		channel.pingLoop(ctx)
		close(done)
	}()
	select {
	case <-socket.closed:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("连续缺失 pong 后未关闭半开连接")
	}
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("关闭半开连接后心跳循环未退出")
	}
	socket.mu.Lock()
	writes := len(socket.writes)
	socket.mu.Unlock()
	if writes != 2 {
		t.Fatalf("达到丢失阈值前的心跳次数不正确: %d", writes)
	}
}

func TestWeComBotPongResetsMissedHeartbeatState(t *testing.T) {
	channel := NewWeComBotChannel("bot-1", "secret-1")
	channel.setLastPingReqID("ping-1")
	channel.missedPongs = 2
	if err := channel.handleFrame(context.Background(), []byte(`{"cmd":"pong","headers":{"req_id":"ping-1"}}`)); err != nil {
		t.Fatalf("处理企业微信 pong 失败: %v", err)
	}
	channel.mu.RLock()
	lastPingReqID := channel.lastPingReqID
	missedPongs := channel.missedPongs
	channel.mu.RUnlock()
	if lastPingReqID != "" || missedPongs != 0 {
		t.Fatalf("pong 未重置心跳状态: req_id=%q missed=%d", lastPingReqID, missedPongs)
	}
}
