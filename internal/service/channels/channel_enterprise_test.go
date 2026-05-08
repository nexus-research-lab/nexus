package channels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDingTalkChannelSendDeliveryTextUsesSessionWebhook(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("钉钉 webhook 应使用 POST，实际 %s", request.Method)
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析钉钉 webhook payload 失败: %v", err)
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"errcode":0}`))
	}))
	defer server.Close()

	channel := newDingTalkChannel("client-id", "client-secret", server.Client())
	if err := channel.SendDeliveryText(context.Background(), DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeDingTalk,
		To:      server.URL,
	}, "hello"); err != nil {
		t.Fatalf("钉钉投递失败: %v", err)
	}

	if payload["msgtype"] != "text" {
		t.Fatalf("钉钉消息类型不正确: %+v", payload)
	}
	textPayload, ok := payload["text"].(map[string]any)
	if !ok || textPayload["content"] != "hello" {
		t.Fatalf("钉钉文本内容不正确: %+v", payload)
	}
}

func TestFeishuTextHelpers(t *testing.T) {
	raw := `{"text":" hello "}`
	messageType := "text"
	if got := extractFeishuText(&messageType, &raw); got != "hello" {
		t.Fatalf("飞书文本提取不正确: %q", got)
	}
	imageType := "image"
	if got := extractFeishuText(&imageType, &raw); got != "" {
		t.Fatalf("非文本飞书消息应忽略，实际 %q", got)
	}
	if got := normalizeFeishuReceiveIDType("", "ou_xxx"); got != feishuReceiveIDTypeOpenID {
		t.Fatalf("open_id 推断不正确: %s", got)
	}
	if got := normalizeFeishuReceiveIDType("", "oc_xxx"); got != feishuReceiveIDTypeChatID {
		t.Fatalf("chat_id 推断不正确: %s", got)
	}
}
