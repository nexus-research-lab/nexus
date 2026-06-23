package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	dingchatbot "github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
)

func TestDingTalkChannelSendDeliveryMessage(t *testing.T) {
	var tokenRequests int
	var messagePayload map[string]string
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v1.0/oauth2/accessToken":
			tokenRequests++
			var payload map[string]string
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("解析钉钉 token 请求失败: %w", err)
			}
			if payload["appKey"] != "ding-client" || payload["appSecret"] != "ding-secret" {
				return nil, fmt.Errorf("钉钉 token 请求凭据不正确: %+v", payload)
			}
			return jsonResponse(`{"accessToken":"ding-token","expireIn":7200}`), nil
		case "/v1.0/robot/groupMessages/send":
			if request.Header.Get("x-acs-dingtalk-access-token") != "ding-token" {
				return nil, fmt.Errorf("钉钉 Authorization 不正确: %s", request.Header.Get("x-acs-dingtalk-access-token"))
			}
			if err := json.NewDecoder(request.Body).Decode(&messagePayload); err != nil {
				return nil, fmt.Errorf("解析钉钉消息请求失败: %w", err)
			}
			return jsonResponse(`{}`), nil
		default:
			return nil, fmt.Errorf("未知钉钉请求路径: %s", request.URL.Path)
		}
	})}

	channel := NewDingTalkChannel("ding-client", "ding-secret", "robot-code", client)
	channel.WithBaseURL("https://dingtalk.test")

	if _, err := channel.SendDeliveryMessage(context.Background(), channelcontract.DeliveryTarget{
		Mode:    channelcontract.DeliveryModeExplicit,
		Channel: channelcontract.ChannelTypeDingTalk,
		To:      "cid-group-1",
	}, "今日新闻摘要"); err != nil {
		t.Fatalf("钉钉发送失败: %v", err)
	}
	if tokenRequests != 1 {
		t.Fatalf("钉钉 token 请求次数不正确: %d", tokenRequests)
	}
	if messagePayload["robotCode"] != "robot-code" || messagePayload["openConversationId"] != "cid-group-1" {
		t.Fatalf("钉钉消息路由不正确: %+v", messagePayload)
	}
	if messagePayload["msgKey"] != "sampleText" {
		t.Fatalf("钉钉消息类型不正确: %+v", messagePayload)
	}
	var msgParam map[string]string
	if err := json.Unmarshal([]byte(messagePayload["msgParam"]), &msgParam); err != nil {
		t.Fatalf("解析钉钉 msgParam 失败: %v", err)
	}
	if msgParam["content"] != "今日新闻摘要" {
		t.Fatalf("钉钉消息正文不正确: %+v", msgParam)
	}
}

func TestDingTalkChannelAccessTokenRefreshUsesSingleflight(t *testing.T) {
	var callers int32
	var tokenRequests int32
	releaseTokenResponse := make(chan struct{})
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v1.0/oauth2/accessToken" {
			return nil, fmt.Errorf("未知钉钉请求路径: %s", request.URL.Path)
		}
		atomic.AddInt32(&tokenRequests, 1)
		<-releaseTokenResponse
		return jsonResponse(`{"accessToken":"ding-token","expireIn":7200}`), nil
	})}
	channel := NewDingTalkChannel("ding-client", "ding-secret", "robot-code", client)
	channel.WithBaseURL("https://dingtalk.test")

	const concurrency = 12
	start := make(chan struct{})
	errs := make(chan error, concurrency)
	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			atomic.AddInt32(&callers, 1)
			token, err := channel.accessTokenForDelivery(context.Background())
			if err != nil {
				errs <- err
				return
			}
			if token != "ding-token" {
				errs <- fmt.Errorf("钉钉 token 不正确: %q", token)
			}
		}()
	}

	close(start)
	deadline := time.After(time.Second)
	for atomic.LoadInt32(&callers) < concurrency {
		select {
		case <-deadline:
			close(releaseTokenResponse)
			t.Fatalf("等待并发 token 请求进入调用路径超时，实际: %d", atomic.LoadInt32(&callers))
		default:
			time.Sleep(time.Millisecond)
		}
	}
	close(releaseTokenResponse)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("钉钉 token 刷新失败: %v", err)
		}
	}
	if got := atomic.LoadInt32(&tokenRequests); got != 1 {
		t.Fatalf("并发刷新应只发起 1 次 token 请求，实际: %d", got)
	}
}

func TestDingTalkStreamMessageAcknowledgesWhenWebhookReportsIngressFailure(t *testing.T) {
	var webhookRequests int32
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://dingtalk.test/session-webhook" {
			return nil, fmt.Errorf("未知钉钉请求地址: %s", request.URL.String())
		}
		atomic.AddInt32(&webhookRequests, 1)
		var payload struct {
			MsgType string `json:"msgtype"`
			Text    struct {
				Content string `json:"content"`
			} `json:"text"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("解析钉钉 webhook 请求失败: %w", err)
		}
		if payload.MsgType != "text" || !strings.Contains(payload.Text.Content, "DingTalk 消息处理失败") {
			return nil, fmt.Errorf("钉钉 webhook 错误提示不正确: %+v", payload)
		}
		return jsonResponse(`{}`), nil
	})}
	channel := NewDingTalkChannel("ding-client", "ding-secret", "", client)
	ingress := &recordingIngressAcceptor{err: errors.New("dm temporarily unavailable")}
	channel.SetIngress(ingress)

	response, err := channel.handleStreamMessage(context.Background(), &dingchatbot.BotCallbackDataModel{
		ConversationId:    "cid-group-1",
		ConversationType:  "2",
		ConversationTitle: "日报群",
		ChatbotCorpId:     "corp-1",
		MsgId:             "ding-message-1",
		SenderStaffId:     "staff-1",
		SenderNick:        "Alice",
		SessionWebhook:    "https://dingtalk.test/session-webhook",
		Text: dingchatbot.BotCallbackDataTextModel{
			Content: "检查今天日报",
		},
	})
	if err != nil {
		t.Fatalf("已通过 webhook 通知用户时不应向钉钉返回错误: %v", err)
	}
	if response == nil || string(response) != "" {
		t.Fatalf("钉钉 stream 应返回空 ACK: %q", string(response))
	}
	if len(ingress.requests) != 1 {
		t.Fatalf("钉钉 Stream 消息应先进入 ingress: %+v", ingress.requests)
	}
	if got := atomic.LoadInt32(&webhookRequests); got != 1 {
		t.Fatalf("钉钉错误 webhook 请求次数不正确: %d", got)
	}
}

func TestDingTalkStreamMessageRemembersSessionWebhookDelivery(t *testing.T) {
	channel := NewDingTalkChannel("ding-client", "ding-secret", "", nil)
	ingress := &recordingIngressAcceptor{}
	channel.SetIngress(ingress)

	if _, err := channel.handleStreamMessage(context.Background(), &dingchatbot.BotCallbackDataModel{
		ConversationId:    "cid-group-1",
		ConversationType:  "2",
		ConversationTitle: "日报群",
		ChatbotCorpId:     "corp-1",
		MsgId:             "ding-message-1",
		SenderStaffId:     "staff-1",
		SenderNick:        "Alice",
		SessionWebhook:    "https://dingtalk.test/session-webhook",
		Text: dingchatbot.BotCallbackDataTextModel{
			Content: "检查今天日报",
		},
	}); err != nil {
		t.Fatalf("钉钉 Stream 消息处理失败: %v", err)
	}

	if len(ingress.requests) != 1 {
		t.Fatalf("钉钉 Stream 消息未进入 ingress: %+v", ingress.requests)
	}
	accepted := ingress.requests[0]
	if accepted.Ref != "cid-group-1" || accepted.ChatType != "group" || accepted.Content != "检查今天日报" {
		t.Fatalf("钉钉 Stream ingress 请求不正确: %+v", accepted)
	}
	if accepted.Delivery == nil ||
		accepted.Delivery.Channel != channelcontract.ChannelTypeDingTalk ||
		accepted.Delivery.To != "https://dingtalk.test/session-webhook" ||
		accepted.Delivery.AccountID != "corp-1" {
		t.Fatalf("钉钉 Stream 回投目标应使用 sessionWebhook: %+v", accepted.Delivery)
	}
}
