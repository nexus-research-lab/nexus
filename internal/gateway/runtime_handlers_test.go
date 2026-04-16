// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：runtime_handlers_test.go
// @Date   ：2026/04/16 23:33:00
// @Author ：leemysw
// 2026/04/16 23:33:00   Create
// =====================================================

package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	providercfg "github.com/nexus-research-lab/nexus/internal/provider"
)

func TestHandleRuntimeOptionsReturnsDefaultProvider(t *testing.T) {
	cfg := newGatewayTestConfig(t)
	migrateGatewaySQLite(t, cfg.DatabaseURL)

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("创建网关失败: %v", err)
	}
	if _, err = server.providers.Create(context.Background(), providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Model:       "glm-5.1",
		Enabled:     true,
		IsDefault:   true,
	}); err != nil {
		t.Fatalf("创建默认 provider 失败: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/agent/v1/runtime/options", nil)
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码不正确: got=%d", recorder.Code)
	}

	var payload struct {
		Data struct {
			DefaultAgentID       string  `json:"default_agent_id"`
			DefaultAgentProvider *string `json:"default_agent_provider"`
		} `json:"data"`
	}
	if err = json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if payload.Data.DefaultAgentID != cfg.DefaultAgentID {
		t.Fatalf("default_agent_id 不正确: got=%s want=%s", payload.Data.DefaultAgentID, cfg.DefaultAgentID)
	}
	if payload.Data.DefaultAgentProvider == nil || *payload.Data.DefaultAgentProvider != "glm" {
		t.Fatalf("default_agent_provider 不正确: got=%v", payload.Data.DefaultAgentProvider)
	}
}

func TestHandleEnsureDirectRoomAllowsMainAgent(t *testing.T) {
	cfg := newGatewayTestConfig(t)
	migrateGatewaySQLite(t, cfg.DatabaseURL)

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("创建网关失败: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/agent/v1/rooms/dm/"+cfg.DefaultAgentID, nil)
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("主智能体直聊状态码不正确: got=%d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Data struct {
			Room struct {
				RoomType string `json:"room_type"`
			} `json:"room"`
		} `json:"data"`
	}
	if err = json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析主智能体直聊响应失败: %v", err)
	}
	if payload.Data.Room.RoomType != "dm" {
		t.Fatalf("主智能体直聊 room_type 不正确: got=%s want=dm", payload.Data.Room.RoomType)
	}
}
