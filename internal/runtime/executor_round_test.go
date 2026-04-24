package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	authsvc "github.com/nexus-research-lab/nexus/internal/auth"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	providercfg "github.com/nexus-research-lab/nexus/internal/provider"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type fakeRoundExecutionClient struct {
	sessionID string
	queryErr  error
	messages  chan sdkprotocol.ReceivedMessage
}

func (c *fakeRoundExecutionClient) Connect(context.Context) error { return nil }

func (c *fakeRoundExecutionClient) Query(context.Context, string) error { return c.queryErr }

func (c *fakeRoundExecutionClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	return c.messages
}

func (c *fakeRoundExecutionClient) Interrupt(context.Context) error { return nil }

func (c *fakeRoundExecutionClient) Disconnect(context.Context) error { return nil }

func (c *fakeRoundExecutionClient) Reconfigure(context.Context, agentclient.Options) error {
	return nil
}

func (c *fakeRoundExecutionClient) SessionID() string { return c.sessionID }

type fakeRoundExecutionMapper struct {
	sessionID string
	results   []RoundMapResult
	err       error
	index     int
}

func (m *fakeRoundExecutionMapper) Map(
	sdkprotocol.ReceivedMessage,
	...string,
) (RoundMapResult, error) {
	if m.err != nil {
		return RoundMapResult{}, m.err
	}
	if m.index >= len(m.results) {
		return RoundMapResult{}, nil
	}
	result := m.results[m.index]
	m.index++
	return result, nil
}

func (m *fakeRoundExecutionMapper) SessionID() string {
	return m.sessionID
}

type fakeRuntimeConfigResolver struct {
	config *providercfg.RuntimeConfig
	err    error
}

func (r fakeRuntimeConfigResolver) ResolveRuntimeConfig(
	context.Context,
	string,
) (*providercfg.RuntimeConfig, error) {
	return r.config, r.err
}

func TestExecuteRoundPersistsDurableMessagesAndEvents(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		messages:  make(chan sdkprotocol.ReceivedMessage, 2),
	}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeResult}

	mapper := &fakeRoundExecutionMapper{
		results: []RoundMapResult{
			{
				DurableMessages: []protocol.Message{
					{"message_id": "assistant-1", "role": "assistant"},
				},
				Events: []protocol.EventMessage{
					protocol.NewEvent(protocol.EventTypeMessage, map[string]any{"message_id": "assistant-1"}),
				},
			},
			{
				DurableMessages: []protocol.Message{
					{"message_id": "result-1", "role": "result", "subtype": "success"},
				},
				Events: []protocol.EventMessage{
					protocol.NewEvent(protocol.EventTypeRoundStatus, map[string]any{"status": "finished"}),
				},
				TerminalStatus: "finished",
				ResultSubtype:  "success",
			},
		},
	}

	synced := make([]string, 0, 2)
	handled := make([]map[string]any, 0, 2)
	emitted := make([]protocol.EventMessage, 0, 2)
	result, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:  "你好",
		Client: client,
		Mapper: mapper,
		SyncSessionID: func(sessionID string) error {
			synced = append(synced, sessionID)
			return nil
		},
		HandleDurableMessage: func(messageValue protocol.Message) error {
			copied := make(map[string]any, len(messageValue))
			for key, value := range messageValue {
				copied[key] = value
			}
			handled = append(handled, copied)
			return nil
		},
		EmitEvent: func(event protocol.EventMessage) error {
			emitted = append(emitted, event)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ExecuteRound 失败: %v", err)
	}
	if result.TerminalStatus != "finished" || result.ResultSubtype != "success" {
		t.Fatalf("终态结果不正确: %+v", result)
	}
	if len(synced) != 2 {
		t.Fatalf("session_id 同步次数不正确: %+v", synced)
	}
	if synced[0] != "sdk-session-1" {
		t.Fatalf("同步的 session_id 不正确: %+v", synced)
	}
	if len(handled) != 2 {
		t.Fatalf("durable 消息处理次数不正确: %+v", handled)
	}
	for _, messageValue := range handled {
		if messageValue["session_id"] != "sdk-session-1" {
			t.Fatalf("durable 消息未补齐 session_id: %+v", messageValue)
		}
	}
	if len(emitted) != 2 {
		t.Fatalf("事件扇出次数不正确: %+v", emitted)
	}
}

func TestExecuteRoundReturnsInterruptedWhenContextCancelled(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		messages:  make(chan sdkprotocol.ReceivedMessage),
	}
	mapper := &fakeRoundExecutionMapper{}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := ExecuteRound(ctx, RoundExecutionRequest{
		Query:  "你好",
		Client: client,
		Mapper: mapper,
	})
	if !errors.Is(err, ErrRoundInterrupted) {
		t.Fatalf("期望返回 ErrRoundInterrupted，实际 %v", err)
	}
}

func TestBuildAgentClientOptionsUsesProviderRuntimeEnv(t *testing.T) {
	thinkingTokens := 2048
	maxTurns := 8
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{
		config: &providercfg.RuntimeConfig{
			AuthToken: "token-1",
			BaseURL:   "https://provider.example.com",
			Model:     "kimi-k2",
		},
	}, AgentClientOptionsInput{
		WorkspacePath:      "/tmp/workspace",
		Provider:           "kimi",
		AllowedTools:       []string{"Read"},
		DisallowedTools:    []string{"Edit"},
		SettingSources:     []string{"project"},
		AppendSystemPrompt: "你是测试 Agent",
		ResumeSessionID:    "sdk-session-1",
		MaxThinkingTokens:  &thinkingTokens,
		MaxTurns:           &maxTurns,
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.PermissionMode != sdkprotocol.PermissionModeDefault {
		t.Fatalf("默认权限模式不正确: %+v", options)
	}
	if options.Env["ANTHROPIC_MODEL"] != "kimi-k2" {
		t.Fatalf("运行时模型未写入 env: %+v", options.Env)
	}
	if options.Model != "kimi-k2" {
		t.Fatalf("运行时模型未写入 SDK options: %+v", options)
	}
	if options.Env["ENABLE_TOOL_SEARCH"] != "false" {
		t.Fatalf("kimi 模型应关闭 tool search: %+v", options.Env)
	}
	if options.Resume != "sdk-session-1" {
		t.Fatalf("resume session_id 不正确: %+v", options)
	}
	if options.MaxThinkingTokens != 2048 || options.MaxTurns != 8 {
		t.Fatalf("思考/轮次限制未透传: %+v", options)
	}
}

func TestBuildAgentClientOptionsInjectsScopedUserEnv(t *testing.T) {
	ctx := authsvc.WithState(context.Background(), authsvc.State{
		AuthRequired: true,
		UserCount:    2,
	})
	ctx = authsvc.WithPrincipal(ctx, &authsvc.Principal{
		UserID:     "user-123",
		Username:   "alice",
		AuthMethod: "test",
	})

	options, err := BuildAgentClientOptions(ctx, fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: "/tmp/workspace",
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Env[nexusctlUserIDEnvName] != "user-123" {
		t.Fatalf("未把当前 user_id 注入运行时环境: %+v", options.Env)
	}
	if options.Env[nexusRuntimeUserIDEnvName] != "user-123" {
		t.Fatalf("未把通用运行时 user_id 注入环境: %+v", options.Env)
	}
	if options.Env[nexusRuntimeScopeModeEnvName] != "user_scoped" {
		t.Fatalf("未把多用户作用域模式注入环境: %+v", options.Env)
	}
}

func TestBuildAgentClientOptionsInjectsSingleUserScopeEnv(t *testing.T) {
	ctx := authsvc.WithState(context.Background(), authsvc.State{
		AuthRequired: false,
		UserCount:    0,
	})

	options, err := BuildAgentClientOptions(ctx, fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: "/tmp/workspace",
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Env[nexusRuntimeScopeModeEnvName] != "single_user" {
		t.Fatalf("未把单用户作用域模式注入环境: %+v", options.Env)
	}
	if options.Env[nexusRuntimeUserIDEnvName] != authsvc.SystemUserID {
		t.Fatalf("未把单用户保底主体注入环境: %+v", options.Env)
	}
}
