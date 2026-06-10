package channels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recordingPersonalWeixinIngress struct {
	requests []IngressRequest
}

func (r *recordingPersonalWeixinIngress) Accept(_ context.Context, request IngressRequest) (*IngressResult, error) {
	r.requests = append(r.requests, request)
	return &IngressResult{
		Channel: request.Channel,
		AgentID: request.AgentID,
	}, nil
}

func TestPersonalWeixinChannelSendDeliveryText(t *testing.T) {
	var receivedPath string
	var receivedAuth string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
		receivedAuth = request.Header.Get("Authorization")
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析个人微信回投请求失败: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ret":0}`))
	}))
	defer server.Close()

	channel := newPersonalWeixinChannel(personalWeixinClientConfig{
		BaseURL:   server.URL,
		Token:     "token-1",
		AccountID: "account-1",
	}, server.Client())
	err := channel.SendDeliveryText(context.Background(), DeliveryTarget{
		Mode:     DeliveryModeExplicit,
		Channel:  ChannelTypeWeixinPersonal,
		To:       "wx-user-1",
		ThreadID: "ctx-token-1",
	}, "你好")
	if err != nil {
		t.Fatalf("个人微信回投失败: %v", err)
	}
	if receivedPath != "/ilink/bot/sendmessage" {
		t.Fatalf("个人微信回投路径不正确: %s", receivedPath)
	}
	if receivedAuth != "Bearer token-1" {
		t.Fatalf("个人微信回投 Authorization 不正确: %q", receivedAuth)
	}
	message, ok := payload["msg"].(map[string]any)
	if !ok {
		t.Fatalf("个人微信回投 msg 不正确: %+v", payload)
	}
	if message["to_user_id"] != "wx-user-1" || message["context_token"] != "ctx-token-1" {
		t.Fatalf("个人微信回投目标不正确: %+v", message)
	}
	items, ok := message["item_list"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("个人微信回投 item_list 不正确: %+v", message)
	}
	textItem := items[0].(map[string]any)["text_item"].(map[string]any)
	if textItem["text"] != "你好" {
		t.Fatalf("个人微信回投文本不正确: %+v", textItem)
	}
	if _, ok := payload["base_info"].(map[string]any); !ok {
		t.Fatalf("个人微信回投应携带 base_info: %+v", payload)
	}
}

func TestPersonalWeixinChannelSendDeliveryTyping(t *testing.T) {
	var getConfigCalls int
	statuses := make([]float64, 0, 2)
	var getConfigPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析个人微信 typing 请求失败: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/ilink/bot/getconfig":
			getConfigCalls++
			getConfigPayload = payload
			_, _ = writer.Write([]byte(`{"ret":0,"typing_ticket":"ticket-1"}`))
		case "/ilink/bot/sendtyping":
			status, _ := payload["status"].(float64)
			statuses = append(statuses, status)
			if payload["ilink_user_id"] != "wx-user-1" || payload["typing_ticket"] != "ticket-1" {
				t.Fatalf("个人微信 typing payload 不正确: %+v", payload)
			}
			_, _ = writer.Write([]byte(`{"ret":0}`))
		default:
			t.Fatalf("未知个人微信请求路径: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	channel := newPersonalWeixinChannel(personalWeixinClientConfig{
		BaseURL: server.URL,
		Token:   "token-1",
	}, server.Client())
	target := DeliveryTarget{
		Mode:     DeliveryModeExplicit,
		Channel:  ChannelTypeWeixinPersonal,
		To:       "wx-user-1",
		ThreadID: "ctx-token-1",
	}

	if err := channel.SendDeliveryTyping(context.Background(), target, true); err != nil {
		t.Fatalf("个人微信 typing start 失败: %v", err)
	}
	if err := channel.SendDeliveryTyping(context.Background(), target, false); err != nil {
		t.Fatalf("个人微信 typing cancel 失败: %v", err)
	}

	if getConfigCalls != 1 {
		t.Fatalf("typing ticket 应缓存，getconfig calls=%d", getConfigCalls)
	}
	if getConfigPayload["ilink_user_id"] != "wx-user-1" || getConfigPayload["context_token"] != "ctx-token-1" {
		t.Fatalf("getconfig payload 不正确: %+v", getConfigPayload)
	}
	if len(statuses) != 2 || statuses[0] != personalWeixinTypingActive || statuses[1] != personalWeixinTypingCancel {
		t.Fatalf("typing status 顺序不正确: %+v", statuses)
	}
}

func TestPersonalWeixinChannelTypingIgnoresGetConfigFailure(t *testing.T) {
	getConfigCalls := 0
	sendTypingCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/ilink/bot/getconfig":
			getConfigCalls++
			_, _ = writer.Write([]byte(`{"ret":1,"errmsg":"typing unavailable"}`))
		case "/ilink/bot/sendtyping":
			sendTypingCalls++
			_, _ = writer.Write([]byte(`{"ret":0}`))
		default:
			t.Fatalf("未知个人微信请求路径: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	channel := newPersonalWeixinChannel(personalWeixinClientConfig{
		BaseURL: server.URL,
		Token:   "token-1",
	}, server.Client())
	err := channel.SendDeliveryTyping(context.Background(), DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeWeixinPersonal,
		To:      "wx-user-1",
	}, true)
	if err != nil {
		t.Fatalf("typing getconfig 失败应静默降级: %v", err)
	}
	if getConfigCalls != 1 || sendTypingCalls != 0 {
		t.Fatalf("typing 降级不正确: getconfig=%d sendtyping=%d", getConfigCalls, sendTypingCalls)
	}
}

func TestPersonalWeixinChannelSendDeliveryTextChecksBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ret":1,"errmsg":"send unavailable"}`))
	}))
	defer server.Close()

	channel := newPersonalWeixinChannel(personalWeixinClientConfig{
		BaseURL: server.URL,
		Token:   "token-1",
	}, server.Client())
	err := channel.SendDeliveryText(context.Background(), DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeWeixinPersonal,
		To:      "wx-user-1",
	}, "你好")
	if err == nil || !strings.Contains(err.Error(), "ret=1") {
		t.Fatalf("个人微信业务失败应返回错误: %v", err)
	}
}

func TestPersonalWeixinChannelHandlesTextMessage(t *testing.T) {
	ingress := &recordingPersonalWeixinIngress{}
	channel := newPersonalWeixinChannel(personalWeixinClientConfig{
		Token:     "token-1",
		AccountID: "account-1",
	}, nil).WithOwner("owner-a")
	channel.SetIngress(ingress)

	channel.handleMessage(context.Background(), personalWeixinMessage{
		FromUserID:   "wx-user-1",
		ContextToken: "ctx-token-1",
		MessageType:  personalWeixinMessageTypeUser,
		ItemList: []personalWeixinMessageItem{{
			Type: personalWeixinItemTypeText,
			TextItem: personalWeixinTextItem{
				Text: "检查今日任务",
			},
		}},
	})

	if len(ingress.requests) != 1 {
		t.Fatalf("个人微信消息未进入 ingress: %+v", ingress.requests)
	}
	request := ingress.requests[0]
	if request.OwnerUserID != "owner-a" || request.Channel != ChannelTypeWeixinPersonal || request.Ref != "wx-user-1" {
		t.Fatalf("个人微信 ingress 基础字段不正确: %+v", request)
	}
	if request.ThreadID != "" {
		t.Fatalf("个人微信 session key 不应使用 context_token: %+v", request)
	}
	if request.Delivery == nil || request.Delivery.To != "wx-user-1" || request.Delivery.ThreadID != "ctx-token-1" {
		t.Fatalf("个人微信 remembered delivery 不正确: %+v", request.Delivery)
	}
	if !strings.Contains(request.Content, "检查今日任务") {
		t.Fatalf("个人微信文本不正确: %+v", request)
	}
}
