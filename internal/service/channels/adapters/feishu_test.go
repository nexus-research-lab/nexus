package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
)

func TestFeishuChannelStartsWebSocketByDefault(t *testing.T) {
	channel := NewFeishuChannel("cli_a", "secret-a", nil)
	var client *fakeFeishuEventClient
	channel.eventFactory = func(config feishuEventClientConfig) feishuEventClient {
		client = &fakeFeishuEventClient{config: config}
		return client
	}

	if err := channel.Start(context.Background()); err != nil {
		t.Fatalf("飞书长连接启动失败: %v", err)
	}
	if client == nil || !client.started {
		t.Fatalf("默认配置应启动飞书长连接: %+v", client)
	}
	if client.config.OnMessage == nil || client.config.OnReaction == nil {
		t.Fatalf("飞书长连接应注册消息和 reaction handler: %+v", client.config)
	}

	if err := channel.Stop(context.Background()); err != nil {
		t.Fatalf("停止飞书长连接失败: %v", err)
	}
	if !client.closed {
		t.Fatal("停止通道时应关闭飞书长连接客户端")
	}
}

func TestFeishuChannelWebhookModeSkipsWebSocket(t *testing.T) {
	channel := NewFeishuChannel("cli_a", "secret-a", nil).WithConnectionMode("webhook")
	channel.eventFactory = func(feishuEventClientConfig) feishuEventClient {
		t.Fatal("webhook 兼容模式不应启动飞书长连接")
		return nil
	}

	if err := channel.Start(context.Background()); err != nil {
		t.Fatalf("飞书 webhook 模式启动失败: %v", err)
	}
}

func TestFeishuChannelReplyUsesMessageReplyAPI(t *testing.T) {
	var replyPayload map[string]any
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			return jsonResponse(`{"code":0,"tenant_access_token":"tenant-token","expire":7200}`), nil
		case "/open-apis/im/v1/messages/om_parent_1/reply":
			if request.Header.Get("Authorization") != "Bearer tenant-token" {
				t.Fatalf("Authorization 不正确: %s", request.Header.Get("Authorization"))
			}
			if err := json.NewDecoder(request.Body).Decode(&replyPayload); err != nil {
				t.Fatalf("解析飞书 reply 请求失败: %v", err)
			}
			return jsonResponse(`{"code":0,"msg":"ok","data":{"message_id":"om_reply_1"}}`), nil
		default:
			t.Fatalf("未知飞书请求路径: %s", request.URL.Path)
			return nil, nil
		}
	})}
	channel := NewFeishuChannel("cli_a", "secret-a", client).
		WithConnectionMode("webhook").
		WithReplyInThread("enabled")
	channel.baseURL = "https://feishu.test"

	_, err := channel.SendDeliveryMessage(context.Background(), channelcontract.DeliveryTarget{
		Mode:     channelcontract.DeliveryModeExplicit,
		Channel:  channelcontract.ChannelTypeFeishu,
		To:       "oc_group_123",
		ThreadID: "om_parent_1",
	}, "收到，我继续处理")
	if err != nil {
		t.Fatalf("飞书 reply 发送失败: %v", err)
	}
	if replyPayload["msg_type"] != "text" || replyPayload["reply_in_thread"] != true {
		t.Fatalf("飞书 reply payload 不正确: %+v", replyPayload)
	}
	var content map[string]string
	if err := json.Unmarshal([]byte(replyPayload["content"].(string)), &content); err != nil {
		t.Fatalf("解析飞书 reply content 失败: %v", err)
	}
	if content["text"] != "收到，我继续处理" {
		t.Fatalf("飞书 reply 正文不正确: %+v", content)
	}
}

func TestFeishuChannelSendDeliveryTypingUsesReaction(t *testing.T) {
	var created bool
	var deleted bool
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			return jsonResponse(`{"code":0,"tenant_access_token":"tenant-token","expire":7200}`), nil
		case "/open-apis/im/v1/messages/om_parent_1/reactions":
			created = true
			var payload map[string]map[string]string
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatalf("解析飞书 reaction 请求失败: %v", err)
			}
			if payload["reaction_type"]["emoji_type"] != "Typing" {
				t.Fatalf("飞书 typing reaction 不正确: %+v", payload)
			}
			return jsonResponse(`{"code":0,"msg":"ok","data":{"reaction_id":"reaction-1"}}`), nil
		case "/open-apis/im/v1/messages/om_parent_1/reactions/reaction-1":
			if request.Method != http.MethodDelete {
				t.Fatalf("删除飞书 typing reaction 应使用 DELETE: %s", request.Method)
			}
			deleted = true
			return jsonResponse(`{"code":0,"msg":"ok"}`), nil
		default:
			t.Fatalf("未知飞书请求路径: %s", request.URL.Path)
			return nil, nil
		}
	})}
	channel := NewFeishuChannel("cli_a", "secret-a", client).WithConnectionMode("webhook")
	channel.baseURL = "https://feishu.test"
	target := channelcontract.DeliveryTarget{
		Mode:     channelcontract.DeliveryModeExplicit,
		Channel:  channelcontract.ChannelTypeFeishu,
		To:       "oc_group_123",
		ThreadID: "om_parent_1",
	}

	if err := channel.SendDeliveryTyping(context.Background(), target, true); err != nil {
		t.Fatalf("飞书 typing start 失败: %v", err)
	}
	if err := channel.SendDeliveryTyping(context.Background(), target, false); err != nil {
		t.Fatalf("飞书 typing stop 失败: %v", err)
	}
	if !created || !deleted {
		t.Fatalf("飞书 typing reaction 未完整创建/删除: created=%v deleted=%v", created, deleted)
	}
}

func TestFeishuChannelHandlesSDKMessageThroughIngress(t *testing.T) {
	ingress := &recordingFeishuIngress{}
	channel := NewFeishuChannel("cli_a", "secret-a", nil).WithOwner("owner-a")
	channel.SetIngress(ingress)

	content := `{"text":"检查今天的定时任务发送情况"}`
	event := &larkim.P2MessageReceiveV1{
		EventV2Base: &larkevent.EventV2Base{
			Schema: "2.0",
			Header: &larkevent.EventHeader{
				EventID:   "evt-1",
				EventType: "im.message.receive_v1",
				AppID:     "cli_a",
			},
		},
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: ptrString("ou_sender"),
				},
				SenderType: ptrString("user"),
			},
			Message: &larkim.EventMessage{
				MessageId:   ptrString("om_1"),
				ChatId:      ptrString("oc_group_123"),
				ChatType:    ptrString("group"),
				MessageType: ptrString("text"),
				Content:     &content,
			},
		},
	}

	if err := channel.handleSDKMessage(context.Background(), event); err != nil {
		t.Fatalf("飞书 SDK 消息进入 ingress 失败: %v", err)
	}
	if len(ingress.requests) != 1 {
		t.Fatalf("飞书 SDK 消息未进入 ingress: %+v", ingress.requests)
	}
	request := ingress.requests[0]
	if request.OwnerUserID != "owner-a" || request.Channel != channelcontract.ChannelTypeFeishu || request.Ref != "oc_group_123" {
		t.Fatalf("飞书 ingress 路由不正确: %+v", request)
	}
	if request.Content != "检查今天的定时任务发送情况" || request.ReqID != "om_1" || request.RoundID != "evt-1" {
		t.Fatalf("飞书 ingress 内容不正确: %+v", request)
	}
	if request.Delivery == nil || request.Delivery.Channel != channelcontract.ChannelTypeFeishu || request.Delivery.To != "oc_group_123" {
		t.Fatalf("飞书回投目标不正确: %+v", request.Delivery)
	}
}

func TestFeishuChannelHandlesSDKReactionThroughIngress(t *testing.T) {
	ingress := &recordingFeishuIngress{}
	channel := NewFeishuChannel("cli_a", "secret-a", nil).WithOwner("owner-a")
	channel.SetIngress(ingress)
	event := &larkim.P2MessageReactionCreatedV1{
		EventReq: &larkevent.EventReq{Body: []byte(`{
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
				}
			}
		}`)},
	}

	if err := channel.handleSDKReaction(context.Background(), event); err != nil {
		t.Fatalf("飞书 SDK reaction 进入 ingress 失败: %v", err)
	}
	if len(ingress.requests) != 1 {
		t.Fatalf("飞书 SDK reaction 未进入 ingress: %+v", ingress.requests)
	}
	if ingress.requests[0].Content != "[reacted with THUMBSUP to message om_bot_reply_1]" {
		t.Fatalf("飞书 SDK reaction 内容不正确: %+v", ingress.requests[0])
	}
}

func TestFeishuSDKExpectedCloseLog(t *testing.T) {
	closedLogs := []string{
		"receive message failed, err: read tcp 192.168.110.28:63283->59.36.213.115:443: use of closed network connection [conn_id=7649650647625518281]",
		"connection is closed, receive message loop exit [conn_id=7649650647625518281]",
		"receive message failed, err: websocket: close 1000 (normal) [conn_id=7649650647625518281]",
	}
	for _, detail := range closedLogs {
		if !isFeishuSDKExpectedCloseLog(detail) {
			t.Fatalf("正常关闭日志未被识别: %s", detail)
		}
	}
	if isFeishuSDKExpectedCloseLog("connect failed, err: auth failed") {
		t.Fatal("飞书鉴权失败不应该被识别为正常关闭")
	}
}

type fakeFeishuEventClient struct {
	config  feishuEventClientConfig
	started bool
	closed  bool
}

func (c *fakeFeishuEventClient) Start(context.Context) error {
	c.started = true
	if c.config.OnReady != nil {
		c.config.OnReady()
	}
	return nil
}

func (c *fakeFeishuEventClient) Close() {
	c.closed = true
}

type recordingFeishuIngress struct {
	requests []channelcontract.IngressRequest
}

func (r *recordingFeishuIngress) Accept(_ context.Context, request channelcontract.IngressRequest) (*channelcontract.IngressResult, error) {
	r.requests = append(r.requests, request)
	return &channelcontract.IngressResult{
		Channel:    request.Channel,
		AgentID:    request.AgentID,
		SessionKey: request.SessionKey,
		RoundID:    request.RoundID,
		ReqID:      request.ReqID,
	}, nil
}

func ptrString(value string) *string {
	return &value
}
