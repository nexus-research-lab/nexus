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
	request := httptest.NewRequest(http.MethodPost, "/agent/v1/channels/internal/messages", bytes.NewReader(body))
	handler.HandleInternalChannelIngress(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码不正确: %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(ingress.requests) != 1 || ingress.requests[0].Channel != channelspkg.ChannelTypeInternal {
		t.Fatalf("internal handler 未强制覆盖 channel: %+v", ingress.requests)
	}
}
