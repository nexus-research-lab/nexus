package dm

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestServiceHandleChatForwardsRuntimeOptions(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	maxThinkingTokens := 2048
	maxTurns := 6
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	}, "glm-5.1", true)
	updatedAgent, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, protocol.UpdateRequest{
		Options: &protocol.Options{
			MaxThinkingTokens: &maxThinkingTokens,
			MaxTurns:          &maxTurns,
			SettingSources:    []string{"user"},
		},
	})
	if err != nil {
		t.Fatalf("更新 agent 配置失败: %v", err)
	}
	if updatedAgent == nil {
		t.Fatal("更新 agent 后返回为空")
	}
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-no-model",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetProviderResolver(providerService)
	titleScheduler := &fakeDMTitleScheduler{}
	service.SetTitleGenerator(titleScheduler)
	sender := newDMTestSender("sender-no-model")
	sessionKey := "agent:nexus:ws:dm:no-model"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 model 透传",
		RoundID:    "round-no-model",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Model != "glm-5.1" {
		t.Fatalf("runtime 未向 SDK options 透传 provider model: %+v", options)
	}
	if options.Env["ANTHROPIC_MODEL"] != "glm-5.1" {
		t.Fatalf("runtime 未注入 provider model: %+v", options.Env)
	}
	if options.Env["ANTHROPIC_AUTH_TOKEN"] != "glm-token" {
		t.Fatalf("runtime 未注入 provider bearer token: %+v", options.Env)
	}
	if options.Env["ANTHROPIC_API_KEY"] != "" {
		t.Fatalf("非官方 Anthropic-compatible runtime 应清空 provider API key，避免继承脏 key: %+v", options.Env)
	}
	if options.Env["ANTHROPIC_DEFAULT_SONNET_MODEL"] != "glm-5.1" {
		t.Fatalf("runtime 未注入默认 sonnet model: %+v", options.Env)
	}
	if options.Env["NEXUS_SUBAGENT_MODEL"] != "glm-5.1" {
		t.Fatalf("runtime 未注入 subagent model: %+v", options.Env)
	}
	if options.Runtime.MaxThinkingTokens != maxThinkingTokens {
		t.Fatalf("runtime 未向 SDK 透传 max thinking tokens: %+v", options)
	}
	if options.Runtime.MaxTurns != maxTurns {
		t.Fatalf("runtime 未向 SDK 透传 max turns: %+v", options)
	}
	if len(options.SettingSources) != 1 || options.SettingSources[0] != "user" {
		t.Fatalf("runtime 未向 SDK 透传 setting_sources: %+v", options)
	}
	if !options.IncludePartialMessages {
		t.Fatalf("runtime 未开启 partial messages: %+v", options)
	}
	if len(options.Tools.Allow) != 0 {
		t.Fatalf("runtime 不应在无显式白名单时为了 Goal 收窄 allowed tools: %+v", options.Tools.Allow)
	}
	if options.Callbacks.PermissionHandler == nil {
		t.Fatal("runtime 权限处理器为空")
	}
	goalSkillDecision, err := options.Callbacks.PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "Skill",
		Input:    map[string]any{"name": "goal-manager"},
	})
	if err != nil {
		t.Fatalf("Goal Skill 权限处理失败: %v", err)
	}
	if goalSkillDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("Goal Skill 应自动放行: %+v", goalSkillDecision)
	}
	goalToolDecision, err := options.Callbacks.PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_goal__update_goal",
		Input:    map[string]any{"status": "complete"},
	})
	if err != nil {
		t.Fatalf("Goal 工具权限处理失败: %v", err)
	}
	if goalToolDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("Goal 工具应自动放行: %+v", goalToolDecision)
	}
	titleRequest := titleScheduler.LastRequest()
	if titleRequest.Provider != "glm" || titleRequest.Model != "glm-5.1" {
		t.Fatalf("标题生成未复用本轮 runtime provider/model: %+v", titleRequest)
	}
}

func TestServiceHandleChatUsesPreferenceDefaultModelForIncompleteAgentSelection(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "deepseek",
		DisplayName: "DeepSeek",
		AuthToken:   "deepseek-token",
		BaseURL:     "https://api.deepseek.com/anthropic",
		Enabled:     true,
	}, "deepseek-v4-flash", true)
	updatedAgent, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, protocol.UpdateRequest{
		Options: &protocol.Options{
			Provider: "kimi-code",
		},
	})
	if err != nil {
		t.Fatalf("写入历史 provider-only agent 配置失败: %v", err)
	}
	if updatedAgent == nil {
		t.Fatal("更新 agent 后返回为空")
	}
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-preference-default",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetProviderResolver(providerService)
	service.SetPreferences(fakeDMPreferencesService{prefs: preferencessvc.Preferences{
		AgentRuntimeKind: "nxs",
		DefaultAgentOptions: protocol.Options{
			Provider: "deepseek",
			Model:    "deepseek-v4-flash",
		},
	}})
	sender := newDMTestSender("sender-preference-default")
	sessionKey := "agent:nexus:ws:dm:preference-default"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试常规默认模型",
		RoundID:    "round-preference-default",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Model != "deepseek-v4-flash" {
		t.Fatalf("runtime 未使用常规设置默认模型: %+v", options)
	}
	if options.Env["NEXUS_RUNTIME_PROVIDER"] != "deepseek" {
		t.Fatalf("runtime 未使用常规设置默认 Provider: %+v", options.Env)
	}
	if options.Runtime.Kind != agentclient.RuntimeNXS {
		t.Fatalf("runtime 未使用常规设置中的 nxs: %+v", options)
	}
}

func TestServiceHandleChatBypassPermissionsKeepsQuestionChannel(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	maxTurns := 4
	agentValue, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, protocol.UpdateRequest{
		Options: &protocol.Options{
			PermissionMode: "bypassPermissions",
			MaxTurns:       &maxTurns,
			SettingSources: []string{"project"},
		},
	})
	if err != nil || agentValue == nil {
		t.Fatalf("更新 agent 失败: value=%+v err=%v", agentValue, err)
	}

	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-bypass",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-bypass")
	sessionKey := "agent:nexus:ws:dm:bypass"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 bypass 权限处理器",
		RoundID:    "round-bypass",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Runtime.PermissionMode != sdkpermission.ModeBypassPermissions {
		t.Fatalf("bypass 权限模式未透传: %+v", options)
	}
	if options.Callbacks.PermissionHandler == nil {
		t.Fatalf("bypass 权限模式应保留 AskUserQuestion 交互通道: %+v", options)
	}
}

func TestServiceHandleChatUsesExplicitProvider(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	}, "glm-5.1", true)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "kimi",
		DisplayName: "Kimi",
		AuthToken:   "kimi-token",
		BaseURL:     "https://api.moonshot.cn/anthropic",
		Enabled:     true,
	}, "kimi-k2.5", false)

	created, err := agentService.CreateAgent(context.Background(), protocol.CreateRequest{
		Name: "显式 Provider 助手",
		Options: &protocol.Options{
			Provider: "kimi",
			Model:    "kimi-k2.5",
		},
	})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-explicit-provider",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetProviderResolver(providerService)
	sessionKey := "agent:" + created.AgentID + ":ws:dm:explicit-provider"
	sender := newDMTestSender("sender-explicit-provider")
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		AgentID:    created.AgentID,
		Content:    "测试显式 provider",
		RoundID:    "round-explicit-provider",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Env["ANTHROPIC_MODEL"] != "kimi-k2.5" {
		t.Fatalf("显式 provider 未命中新 provider model: %+v", options.Env)
	}
	if options.Env["ANTHROPIC_BASE_URL"] != "https://api.moonshot.cn/anthropic" {
		t.Fatalf("显式 provider 未命中新 provider base_url: %+v", options.Env)
	}
	if !options.IncludePartialMessages {
		t.Fatalf("显式 provider runtime 未开启 partial messages: %+v", options)
	}
}
