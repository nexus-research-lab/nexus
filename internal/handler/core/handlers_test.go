package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	corehandler "github.com/nexus-research-lab/nexus/internal/handler/core"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
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
	handlertest.CloseServer(t, server)

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
	if !payload.Data.Available && (payload.Data.CanDownload || payload.Data.Message == "") {
		t.Fatalf("不可用时应给出明确路径配置提示且不允许下载: %+v", payload.Data)
	}
}

func TestPreferencesHTTPUsesETagCASAndPreservesNewerSettings(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	prefs := preferencessvc.NewService(cfg)
	handler := corehandler.New(handlershared.NewAPI(nil), nil, nil, prefs)

	getRequest := httptest.NewRequest(http.MethodGet, "/nexus/v1/settings/preferences", nil)
	getRecorder := httptest.NewRecorder()
	handler.HandleGetPreferences(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("初始读取失败: status=%d body=%s", getRecorder.Code, getRecorder.Body.String())
	}
	initialETag := getRecorder.Header().Get("ETag")
	if initialETag != `"preferences-1"` {
		t.Fatalf("初始 ETag = %q, want preferences version 1", initialETag)
	}
	if getRecorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Preferences 派生展示响应不应被缓存: %q", getRecorder.Header().Get("Cache-Control"))
	}

	firstRequest := httptest.NewRequest(
		http.MethodPatch,
		"/nexus/v1/settings/preferences",
		strings.NewReader(`{"agent_sdk_diagnostics_enabled":true}`),
	)
	firstRequest.Header.Set("Content-Type", "application/json")
	firstRequest.Header.Set("If-Match", initialETag)
	firstRecorder := httptest.NewRecorder()
	handler.HandleUpdatePreferences(firstRecorder, firstRequest)
	if firstRecorder.Code != http.StatusOK || firstRecorder.Header().Get("ETag") != `"preferences-2"` {
		t.Fatalf("首个 CAS 更新失败: status=%d etag=%q body=%s",
			firstRecorder.Code, firstRecorder.Header().Get("ETag"), firstRecorder.Body.String())
	}
	if firstRecorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Preferences PATCH 成功响应应禁止缓存: %q", firstRecorder.Header().Get("Cache-Control"))
	}

	staleRequest := httptest.NewRequest(
		http.MethodPatch,
		"/nexus/v1/settings/preferences",
		strings.NewReader(`{"emotion_enabled":true}`),
	)
	staleRequest.Header.Set("Content-Type", "application/json")
	staleRequest.Header.Set("If-Match", initialETag)
	staleRecorder := httptest.NewRecorder()
	handler.HandleUpdatePreferences(staleRecorder, staleRequest)
	if staleRecorder.Code != http.StatusPreconditionFailed {
		t.Fatalf("陈旧 CAS status=%d body=%s", staleRecorder.Code, staleRecorder.Body.String())
	}
	if !strings.Contains(staleRecorder.Body.String(), "偏好设置版本与服务端最新版本不一致") ||
		strings.Contains(staleRecorder.Body.String(), `"detail":"请求失败"`) {
		t.Fatalf("412 文案退化为通用请求失败: %s", staleRecorder.Body.String())
	}
	if staleRecorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("412 Preferences 响应应禁止缓存: %q", staleRecorder.Header().Get("Cache-Control"))
	}
	var stalePayload struct {
		Data struct {
			Failure protocol.FailureCore `json:"failure"`
		} `json:"data"`
	}
	if err := json.Unmarshal(staleRecorder.Body.Bytes(), &stalePayload); err != nil {
		t.Fatalf("解析冲突响应: %v", err)
	}
	if stalePayload.Data.Failure.Code != "preferences.version_conflict" ||
		stalePayload.Data.Failure.Effect != protocol.FailureEffectNotApplied {
		t.Fatalf("冲突 FailureCore 不正确: %+v", stalePayload.Data.Failure)
	}

	stored, err := prefs.Get(context.Background(), authsvc.SystemUserID)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.AgentSDKDiagnosticsEnabled || stored.EmotionEnabled || stored.Version != 2 {
		t.Fatalf("陈旧写覆盖了最新偏好: %+v", stored)
	}
}

func TestPreferencesHTTPRejectsInvalidIfMatchBeforeMutation(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	prefs := preferencessvc.NewService(cfg)
	handler := corehandler.New(handlershared.NewAPI(nil), nil, nil, prefs)
	request := httptest.NewRequest(
		http.MethodPatch,
		"/nexus/v1/settings/preferences",
		strings.NewReader(`{"emotion_enabled":true}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("If-Match", `W/"preferences-1"`)
	recorder := httptest.NewRecorder()

	handler.HandleUpdatePreferences(recorder, request)

	if recorder.Code != http.StatusBadRequest ||
		!strings.Contains(recorder.Body.String(), `"code":"preferences.precondition_invalid"`) ||
		!strings.Contains(recorder.Body.String(), `"effect":"not_applied"`) {
		t.Fatalf("无效 If-Match 没有在写入前拒绝: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	stored, err := prefs.Get(context.Background(), authsvc.SystemUserID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version != 1 || stored.EmotionEnabled {
		t.Fatalf("无效 If-Match 改写了偏好: %+v", stored)
	}
}

func TestPreferencesHTTPKeepsUnconditionalPatchForLegacyClients(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	prefs := preferencessvc.NewService(cfg)
	handler := corehandler.New(handlershared.NewAPI(nil), nil, nil, prefs)
	request := httptest.NewRequest(
		http.MethodPatch,
		"/nexus/v1/settings/preferences",
		strings.NewReader(`{"emotion_enabled":true}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.HandleUpdatePreferences(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Header().Get("ETag") != `"preferences-2"` {
		t.Fatalf("旧客户端无 If-Match PATCH 不兼容: status=%d etag=%q body=%s",
			recorder.Code, recorder.Header().Get("ETag"), recorder.Body.String())
	}
	stored, err := prefs.Get(context.Background(), authsvc.SystemUserID)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.EmotionEnabled || stored.Version != 2 {
		t.Fatalf("旧客户端 PATCH 没有保持原行为: %+v", stored)
	}
}

func TestHandleRuntimeOptionsReturnsDefaultProvider(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()
	agents := agentpkg.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
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
	handlertest.CloseServer(t, server)

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
			Preferences          struct {
				DefaultAgentOptions struct {
					AllowedTools []string `json:"allowed_tools"`
				} `json:"default_agent_options"`
			} `json:"preferences"`
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
	if stringSliceContains(payload.Data.Preferences.DefaultAgentOptions.AllowedTools, "nexus_imagegen") {
		t.Fatalf("未配置生图 provider 时不应默认打开 nexus_imagegen: %+v", payload.Data.Preferences.DefaultAgentOptions.AllowedTools)
	}
}

func TestHandleRuntimeOptionsEnablesImagegenDefaultTool(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()
	providers := providercfg.NewServiceWithDB(cfg, db)
	imageProvider, err := providers.Create(context.Background(), providercfg.CreateInput{
		ProviderKind: providercfg.ProviderKindImageGeneration,
		Provider:     "image-default",
		PresetKey:    "custom",
		APIFormat:    providercfg.APIFormatOpenAIImageGeneration,
		AuthToken:    "image-token",
		BaseURL:      "https://image.example.com/v1",
		ModelsPath:   "/models",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("创建生图 provider 失败: %v", err)
	}
	if _, err = providers.UpdateModel(context.Background(), imageProvider.Provider, "image-model", providercfg.UpdateModelInput{
		Enabled:   true,
		IsDefault: true,
	}); err != nil {
		t.Fatalf("设置默认生图模型失败: %v", err)
	}
	prefs := preferencessvc.NewService(cfg)
	if _, err = prefs.Update(context.Background(), authsvc.SystemUserID, preferencessvc.UpdateRequest{
		DefaultAgentOptions: &protocol.Options{
			PermissionMode: "default",
			AllowedTools:   []string{"Read"},
			SettingSources: []string{"project"},
		},
	}); err != nil {
		t.Fatalf("写入默认工具偏好失败: %v", err)
	}

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)

	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/runtime/options", nil)
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码不正确: got=%d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Data struct {
			Preferences struct {
				DefaultAgentOptions struct {
					AllowedTools []string `json:"allowed_tools"`
				} `json:"default_agent_options"`
			} `json:"preferences"`
		} `json:"data"`
	}
	if err = json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if !stringSliceContains(payload.Data.Preferences.DefaultAgentOptions.AllowedTools, "nexus_imagegen") {
		t.Fatalf("配置生图 provider 后默认工具应打开 nexus_imagegen: %+v", payload.Data.Preferences.DefaultAgentOptions.AllowedTools)
	}
}

func TestHandleProviderOptionsUsesRuntimeKind(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)

	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()
	providers := providercfg.NewServiceWithDB(cfg, db)
	record, err := providers.Create(context.Background(), providercfg.CreateInput{
		Provider:  "openai",
		PresetKey: "openai",
		AuthToken: "openai-token",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("创建 OpenAI provider 失败: %v", err)
	}
	if _, err = providers.UpdateModel(context.Background(), record.Provider, "gpt-4o", providercfg.UpdateModelInput{
		Enabled:   true,
		IsDefault: true,
	}); err != nil {
		t.Fatalf("设置 OpenAI 默认模型失败: %v", err)
	}

	server, err := serverapp.New(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	handlertest.CloseServer(t, server)

	defaultOptions := requestProviderOptions(t, server, "/nexus/v1/settings/providers/options")
	if !providerOptionsContains(defaultOptions.Items, "openai") {
		t.Fatalf("默认 nxs runtime 应返回 OpenAI: %+v", defaultOptions.Items)
	}
	if defaultOptions.DefaultProvider == nil || *defaultOptions.DefaultProvider != "openai" ||
		defaultOptions.DefaultModel == nil || *defaultOptions.DefaultModel != "gpt-4o" {
		t.Fatalf("默认 nxs runtime 默认模型不正确: %+v", defaultOptions)
	}
	nxsOptions := requestProviderOptions(t, server, "/nexus/v1/settings/providers/options?agent_runtime_kind=nxs")
	if !providerOptionsContains(nxsOptions.Items, "openai") {
		t.Fatalf("nxs runtime 应返回 OpenAI: %+v", nxsOptions.Items)
	}
	if nxsOptions.DefaultProvider == nil || *nxsOptions.DefaultProvider != "openai" ||
		nxsOptions.DefaultModel == nil || *nxsOptions.DefaultModel != "gpt-4o" {
		t.Fatalf("nxs runtime 默认模型不正确: %+v", nxsOptions)
	}
	claudeOptions := requestProviderOptions(t, server, "/nexus/v1/settings/providers/options?agent_runtime_kind=claude")
	if providerOptionsContains(claudeOptions.Items, "openai") {
		t.Fatalf("显式 Claude runtime 不应返回 OpenAI: %+v", claudeOptions.Items)
	}
}

func stringSliceContains(items []string, target string) bool {
	return slices.Contains(items, target)
}

type routerServer interface {
	Router() http.Handler
}

func requestProviderOptions(t *testing.T, server routerServer, target string) providercfg.OptionsResponse {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, target, nil)
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("状态码不正确: got=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data providercfg.OptionsResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	return payload.Data
}

func providerOptionsContains(items []providercfg.Option, provider string) bool {
	return slices.ContainsFunc(items, func(item providercfg.Option) bool {
		return item.Provider == provider
	})
}
