// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：channel_handlers_test.go
// @Date   ：2026/04/12 00:24:00
// @Author ：leemysw
// 2026/04/12 00:24:00   Create
// =====================================================

package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/channels"
)

type fakeChannelIngress struct {
	requests []channels.IngressRequest
	result   *channels.IngressResult
	err      error
}

func (f *fakeChannelIngress) Accept(_ context.Context, request channels.IngressRequest) (*channels.IngressResult, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &channels.IngressResult{
		Channel:    request.Channel,
		AgentID:    request.AgentID,
		SessionKey: request.SessionKey,
		RoundID:    request.RoundID,
		ReqID:      request.ReqID,
	}, nil
}

func TestHandleInternalChannelIngressOverridesChannel(t *testing.T) {
	fakeIngress := &fakeChannelIngress{
		result: &channels.IngressResult{
			Channel:    channels.ChannelTypeInternal,
			AgentID:    "nexus",
			SessionKey: "agent:nexus:internal:dm:chat",
			RoundID:    "round-1",
			ReqID:      "req-1",
		},
	}
	server := &Server{
		ingress: fakeIngress,
	}

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
	server.handleInternalChannelIngress(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码不正确: %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(fakeIngress.requests) != 1 || fakeIngress.requests[0].Channel != channels.ChannelTypeInternal {
		t.Fatalf("internal handler 未强制覆盖 channel: %+v", fakeIngress.requests)
	}

	payload := decodeGatewayResponse(t, recorder.Body.Bytes())
	if payload["success"] != true {
		t.Fatalf("响应 success 不正确: %+v", payload)
	}
}

func TestHandleChannelIngressRejectsClientError(t *testing.T) {
	server := &Server{
		ingress: &fakeChannelIngress{
			err: channels.ErrIngressChannelRequired,
		},
	}

	body, err := json.Marshal(map[string]any{
		"content": "hello",
	})
	if err != nil {
		t.Fatalf("编码请求失败: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/v1/channels/messages", bytes.NewReader(body))
	server.handleChannelIngress(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("状态码不正确: %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func decodeGatewayResponse(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	return payload
}
