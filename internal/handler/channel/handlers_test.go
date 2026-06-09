package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	channelspkg "github.com/nexus-research-lab/nexus/internal/service/channels"
)

type fakeIngress struct {
	requests []channelspkg.IngressRequest
	result   *channelspkg.IngressResult
	err      error
}

func (f *fakeIngress) Accept(_ context.Context, request channelspkg.IngressRequest) (*channelspkg.IngressResult, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &channelspkg.IngressResult{
		Channel:    request.Channel,
		AgentID:    request.AgentID,
		SessionKey: request.SessionKey,
		RoundID:    request.RoundID,
		ReqID:      request.ReqID,
	}, nil
}

type fakeControl struct {
	prepared       channelspkg.FeishuIngressPreparation
	prepareErr     error
	wechatPrepared channelspkg.WeChatIngressPreparation
	wechatErr      error
	ownerByConfig  string
	ownerErr       error
}

func (f *fakeControl) ListChannels(context.Context, string) ([]channelspkg.ChannelConfigView, error) {
	return nil, nil
}

func (f *fakeControl) UpsertChannelConfig(context.Context, string, string, channelspkg.UpsertChannelConfigRequest) (*channelspkg.ChannelConfigView, error) {
	return nil, nil
}

func (f *fakeControl) DeleteChannelConfig(context.Context, string, string) error {
	return nil
}

func (f *fakeControl) ListPairings(context.Context, string, channelspkg.PairingQuery) ([]channelspkg.PairingView, error) {
	return nil, nil
}

func (f *fakeControl) CreatePairing(context.Context, string, channelspkg.CreatePairingRequest) (*channelspkg.PairingView, error) {
	return nil, nil
}

func (f *fakeControl) UpdatePairing(context.Context, string, string, channelspkg.UpdatePairingRequest) (*channelspkg.PairingView, error) {
	return nil, nil
}

func (f *fakeControl) DeletePairing(context.Context, string, string) error {
	return nil
}

func (f *fakeControl) ResolveChannelOwnerByConfig(context.Context, string, string, string) (string, error) {
	return f.ownerByConfig, f.ownerErr
}

func (f *fakeControl) PrepareFeishuIngress(context.Context, []byte, http.Header) (channelspkg.FeishuIngressPreparation, error) {
	return f.prepared, f.prepareErr
}

func (f *fakeControl) PrepareWeChatIngress(context.Context, []byte, *http.Request) (channelspkg.WeChatIngressPreparation, error) {
	return f.wechatPrepared, f.wechatErr
}

func TestHandleInternalChannelIngressOverridesChannel(t *testing.T) {
	ingress := &fakeIngress{
		result: &channelspkg.IngressResult{
			Channel:    channelspkg.ChannelTypeInternal,
			AgentID:    "nexus",
			SessionKey: "agent:nexus:internal:dm:chat",
			RoundID:    "round-1",
			ReqID:      "req-1",
		},
	}
	handler := New(handlershared.NewAPI(nil), ingress)

	body, err := json.Marshal(map[string]any{
		"channel": "telegram",
		"ref":     "chat",
		"content": "hello",
	})
	if err != nil {
		t.Fatalf("编码请求失败: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/nexus/v1/channels/internal/messages", bytes.NewReader(body))
	handler.HandleInternalChannelIngress(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码不正确: %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(ingress.requests) != 1 || ingress.requests[0].Channel != channelspkg.ChannelTypeInternal {
		t.Fatalf("internal handler 未强制覆盖 channel: %+v", ingress.requests)
	}
}

func TestHandleFeishuChannelIngressChallenge(t *testing.T) {
	handler := New(handlershared.NewAPI(nil), &fakeIngress{})
	body := []byte(`{"type":"url_verification","challenge":"challenge-token"}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/nexus/v1/channels/feishu/messages", bytes.NewReader(body))
	handler.HandleFeishuChannelIngress(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码不正确: %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("响应不是 JSON: %v", err)
	}
	if payload["challenge"] != "challenge-token" {
		t.Fatalf("challenge 响应不正确: %+v", payload)
	}
}

func TestHandleFeishuChannelIngressMessage(t *testing.T) {
	ingress := &fakeIngress{}
	handler := New(handlershared.NewAPI(nil), ingress)
	body := []byte(`{
		"schema": "2.0",
		"header": {
			"event_id": "evt-1",
			"event_type": "im.message.receive_v1",
			"app_id": "cli_a"
		},
		"event": {
			"message": {
				"message_id": "om_1",
				"chat_id": "oc_group_123",
				"chat_type": "group",
				"message_type": "text",
				"content": "{\"text\":\"停止每日新闻定时任务\"}"
			}
		}
	}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/nexus/v1/channels/feishu/messages", bytes.NewReader(body))
	handler.HandleFeishuChannelIngress(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码不正确: %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(ingress.requests) != 1 {
		t.Fatalf("feishu 消息未进入 ingress: %+v", ingress.requests)
	}
	accepted := ingress.requests[0]
	if accepted.Channel != channelspkg.ChannelTypeFeishu || accepted.Ref != "oc_group_123" || accepted.Content != "停止每日新闻定时任务" {
		t.Fatalf("feishu ingress 请求不正确: %+v", accepted)
	}
}

func TestHandleFeishuChannelIngressUsesPreparedOwner(t *testing.T) {
	ingress := &fakeIngress{}
	body := []byte(`{
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
				"content": "{\"text\":\"创建每日新闻定时任务\"}"
			}
		}
	}`)
	handler := New(handlershared.NewAPI(nil), ingress, &fakeControl{
		prepared: channelspkg.FeishuIngressPreparation{
			Body:        body,
			OwnerUserID: "owner-a",
			AppID:       "cli_a",
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/nexus/v1/channels/feishu/messages", bytes.NewReader(body))
	handler.HandleFeishuChannelIngress(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码不正确: %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(ingress.requests) != 1 {
		t.Fatalf("feishu 消息未进入 ingress: %+v", ingress.requests)
	}
	if ingress.requests[0].OwnerUserID != "owner-a" {
		t.Fatalf("Feishu handler 应把配置解析出的 owner 传给 ingress: %+v", ingress.requests[0])
	}
}

func TestHandleWeChatChannelIngressUsesPreparedOwner(t *testing.T) {
	ingress := &fakeIngress{}
	plain := []byte(`<xml>
		<ToUserName><![CDATA[ww_corp]]></ToUserName>
		<FromUserName><![CDATA[zhangsan]]></FromUserName>
		<CreateTime>1700000000</CreateTime>
		<MsgType><![CDATA[text]]></MsgType>
		<Content><![CDATA[检查今天发送情况]]></Content>
		<MsgId>msg-1</MsgId>
		<AgentID>100001</AgentID>
	</xml>`)
	handler := New(handlershared.NewAPI(nil), ingress, &fakeControl{
		wechatPrepared: channelspkg.WeChatIngressPreparation{
			Body:        plain,
			OwnerUserID: "owner-a",
			CorpID:      "ww_corp",
			AgentID:     "100001",
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/nexus/v1/channels/wechat/messages", bytes.NewReader([]byte(`<xml></xml>`)))
	handler.HandleWeChatChannelIngress(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码不正确: %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(ingress.requests) != 1 {
		t.Fatalf("wechat 消息未进入 ingress: %+v", ingress.requests)
	}
	accepted := ingress.requests[0]
	if accepted.OwnerUserID != "owner-a" || accepted.Channel != channelspkg.ChannelTypeWeChat || accepted.Ref != "zhangsan" {
		t.Fatalf("wechat ingress 请求不正确: %+v", accepted)
	}
	if accepted.Content != "检查今天发送情况" {
		t.Fatalf("wechat 消息内容不正确: %+v", accepted)
	}
}
