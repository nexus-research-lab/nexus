package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type fakeRuntimeClient struct {
	reconfigureCalls int
	lastOptions      agentclient.Options
	sentContents     []string
}

func (c *fakeRuntimeClient) Connect(context.Context) error { return nil }

func (c *fakeRuntimeClient) Query(context.Context, string) error { return nil }

func (c *fakeRuntimeClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	return nil
}

func (c *fakeRuntimeClient) SendContent(_ context.Context, content any, _ *string, _ string) error {
	if text, ok := content.(string); ok {
		c.sentContents = append(c.sentContents, text)
	}
	return nil
}

func (c *fakeRuntimeClient) Interrupt(context.Context) error { return nil }

func (c *fakeRuntimeClient) Disconnect(context.Context) error { return nil }

func (c *fakeRuntimeClient) Reconfigure(_ context.Context, options agentclient.Options) error {
	c.reconfigureCalls++
	c.lastOptions = options
	return nil
}

func (c *fakeRuntimeClient) SessionID() string { return "" }

type fakeRuntimeFactory struct {
	client  *fakeRuntimeClient
	clients []*fakeRuntimeClient
	index   int
}

func (f *fakeRuntimeFactory) New(agentclient.Options) Client {
	if len(f.clients) > 0 {
		client := f.clients[f.index]
		f.index++
		return client
	}
	return f.client
}

func TestManagerGetOrCreateReconfiguresExistingClient(t *testing.T) {
	client := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})

	first, err := manager.GetOrCreate(context.Background(), "agent:nexus:ws:dm:test", agentclient.Options{
		CWD: "/tmp/a",
	})
	if err != nil {
		t.Fatalf("首次创建 client 失败: %v", err)
	}
	second, err := manager.GetOrCreate(context.Background(), "agent:nexus:ws:dm:test", agentclient.Options{
		CWD:            "/tmp/b",
		PermissionMode: sdkprotocol.PermissionModeAcceptEdits,
	})
	if err != nil {
		t.Fatalf("复用 client 失败: %v", err)
	}

	if first != second {
		t.Fatal("期望复用同一个 client 实例")
	}
	if client.reconfigureCalls != 1 {
		t.Fatalf("期望调用一次 Reconfigure，实际 %d", client.reconfigureCalls)
	}
	if client.lastOptions.CWD != "/tmp/b" {
		t.Fatalf("Reconfigure 未收到最新配置: %+v", client.lastOptions)
	}
	if client.lastOptions.PermissionMode != sdkprotocol.PermissionModeAcceptEdits {
		t.Fatalf("Reconfigure 未收到权限模式: %+v", client.lastOptions)
	}
}

func TestManagerSendContentToRunningRound(t *testing.T) {
	client := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	sessionKey := "agent:nexus:ws:dm:test-queue"

	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 client 失败: %v", err)
	}
	manager.StartRound(sessionKey, "round-queue", func() {})

	roundIDs, err := manager.SendContentToRunningRound(context.Background(), sessionKey, "补充信息")
	if err != nil {
		t.Fatalf("排队 streaming input 失败: %v", err)
	}
	if len(roundIDs) != 1 || roundIDs[0] != "round-queue" {
		t.Fatalf("返回运行中 round 不正确: %+v", roundIDs)
	}
	if len(client.sentContents) != 1 || client.sentContents[0] != "补充信息" {
		t.Fatalf("client 未收到排队输入: %+v", client.sentContents)
	}
}

func TestManagerSendContentWithoutRunningRound(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	_, err := manager.SendContentToRunningRound(context.Background(), "agent:nexus:ws:dm:missing", "补充信息")
	if !errors.Is(err, ErrNoRunningRound) {
		t.Fatalf("期望 ErrNoRunningRound，实际 %v", err)
	}
}

func TestManagerGuidanceHookInjectsPostToolUseAdditionalContext(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:ws:dm:test-guide"
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 client 失败: %v", err)
	}
	manager.StartRound(sessionKey, "round-guide", func() {})

	roundIDs, err := manager.QueueGuidanceInput(context.Background(), sessionKey, "round-guide-msg", "请优先检查日志")
	if err != nil {
		t.Fatalf("登记引导输入失败: %v", err)
	}
	if len(roundIDs) != 1 || roundIDs[0] != "round-guide" {
		t.Fatalf("返回运行中 round 不正确: %+v", roundIDs)
	}
	if count := manager.PendingGuidanceCount(sessionKey); count != 1 {
		t.Fatalf("PendingGuidanceCount = %d, want 1", count)
	}

	options := manager.WithGuidanceHook(agentclient.Options{}, sessionKey)
	matchers := options.Hooks[sdkprotocol.HookEventPostToolUse]
	if len(matchers) != 1 || len(matchers[0].Hooks) != 1 {
		t.Fatalf("PostToolUse hook 未注册: %+v", matchers)
	}
	output, err := matchers[0].Hooks[0](context.Background(), sdkprotocol.HookInput{
		EventName: sdkprotocol.HookEventPostToolUse,
	}, "tool-1")
	if err != nil {
		t.Fatalf("执行 PostToolUse hook 失败: %v", err)
	}
	additionalContext, _ := output.HookSpecificOutput["additionalContext"].(string)
	if !strings.Contains(additionalContext, "请优先检查日志") || !strings.Contains(additionalContext, "round-guide-msg") {
		t.Fatalf("additionalContext 未包含引导内容: %q", additionalContext)
	}
	if count := manager.PendingGuidanceCount(sessionKey); count != 0 {
		t.Fatalf("PendingGuidanceCount = %d, want 0", count)
	}
}
