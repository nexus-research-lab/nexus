package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	corehandler "github.com/nexus-research-lab/nexus/internal/handler/core"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	sqlitestorage "github.com/nexus-research-lab/nexus/internal/storage/sqlite"
	versionpkg "github.com/nexus-research-lab/nexus/internal/version"
)

func TestHandleSystemVersion(t *testing.T) {
	handler := corehandler.New(handlershared.NewAPI(nil), nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/system/version", nil)
	recorder := httptest.NewRecorder()

	handler.HandleSystemVersion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码不正确: got=%d", recorder.Code)
	}
	var payload struct {
		Data versionpkg.Info `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if payload.Data.Project != versionpkg.ProjectName || payload.Data.Target == "" {
		t.Fatalf("版本响应不正确: %+v", payload.Data)
	}
}

func TestHandleNXSRuntimeStatus(t *testing.T) {
	t.Setenv("NEXUS_NXS_COMMAND_PATH", "")
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/settings/runtime/nxs/status", nil)
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码不正确: got=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data struct {
			Available   bool   `json:"available"`
			CanDownload bool   `json:"can_download"`
			Message     string `json:"message"`
		} `json:"data"`
	}
	if err = json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if !payload.Data.Available && !payload.Data.CanDownload {
		t.Fatalf("不可用时应允许下载或给出阻断原因: %+v", payload.Data)
	}
}

func TestHandleRuntimeOptionsReturnsDefaultProvider(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()
	agents := agentpkg.NewService(cfg, sqlitestorage.NewAgentRepository(db))
	providers := providercfg.NewServiceWithDB(cfg, db)
	createdProvider, err := providers.Create(context.Background(), providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("创建默认 provider 失败: %v", err)
	}
	if _, err = providers.UpdateModel(context.Background(), createdProvider.Provider, "glm-5.1", providercfg.UpdateModelInput{
		Enabled:   true,
		IsDefault: true,
	}); err != nil {
		t.Fatalf("设置默认模型失败: %v", err)
	}
	defaultAgent, err := agents.GetDefaultAgent(context.Background())
	if err != nil {
		t.Fatalf("加载默认 agent 失败: %v", err)
	}
	avatar := "12"
	if _, err = agents.UpdateAgent(context.Background(), defaultAgent.AgentID, protocol.UpdateRequest{
		Avatar: &avatar,
	}); err != nil {
		t.Fatalf("更新默认 agent 头像失败: %v", err)
	}

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/runtime/options", nil)
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码不正确: got=%d", recorder.Code)
	}

	var payload struct {
		Data struct {
			DefaultAgentID       string  `json:"default_agent_id"`
			DefaultAgentAvatar   string  `json:"default_agent_avatar"`
			DefaultAgentProvider *string `json:"default_agent_provider"`
			DefaultAgentModel    *string `json:"default_agent_model"`
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
	if payload.Data.DefaultAgentModel == nil || *payload.Data.DefaultAgentModel != "glm-5.1" {
		t.Fatalf("default_agent_model 不正确: got=%v", payload.Data.DefaultAgentModel)
	}
	if payload.Data.DefaultAgentAvatar != avatar {
		t.Fatalf("default_agent_avatar 不正确: got=%s want=%s", payload.Data.DefaultAgentAvatar, avatar)
	}
}
