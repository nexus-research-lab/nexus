package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	sqlitestorage "github.com/nexus-research-lab/nexus/internal/storage/sqlite"
)

func TestHandleRuntimeOptionsReturnsDefaultProvider(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()
	agents := agentpkg.NewService(cfg, sqlitestorage.NewAgentRepository(db))
	providers := providercfg.NewServiceWithDB(cfg, db)
	if _, err := providers.Create(context.Background(), providercfg.CreateInput{
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
	if payload.Data.DefaultAgentAvatar != avatar {
		t.Fatalf("default_agent_avatar 不正确: got=%s want=%s", payload.Data.DefaultAgentAvatar, avatar)
	}
}
