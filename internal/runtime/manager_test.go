package runtime

import (
	"context"
	"errors"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type fakeRuntimeClient struct {
	reconfigureCalls int
	lastOptions      agentclient.Options
	disconnectCalls  int
	disconnectErr    error
}

func (c *fakeRuntimeClient) Connect(context.Context) error { return nil }

func (c *fakeRuntimeClient) Query(context.Context, string) error { return nil }

func (c *fakeRuntimeClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	return nil
}

func (c *fakeRuntimeClient) Interrupt(context.Context) error { return nil }

func (c *fakeRuntimeClient) Disconnect(context.Context) error {
	c.disconnectCalls++
	return c.disconnectErr
}

func (c *fakeRuntimeClient) Reconfigure(_ context.Context, options agentclient.Options) error {
	c.reconfigureCalls++
	c.lastOptions = options
	return nil
}

func (c *fakeRuntimeClient) SetPermissionMode(context.Context, sdkprotocol.PermissionMode) error {
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

func TestManagerRecycleClientKeepsRunningRoundsAndCreatesFreshClient(t *testing.T) {
	first := &fakeRuntimeClient{}
	second := &fakeRuntimeClient{}
	factory := &fakeRuntimeFactory{clients: []*fakeRuntimeClient{first, second}}
	manager := NewManagerWithFactory(factory)

	client, err := manager.GetOrCreate(context.Background(), "agent:nexus:ws:dm:test", agentclient.Options{
		CWD: "/tmp/a",
	})
	if err != nil {
		t.Fatalf("首次创建 client 失败: %v", err)
	}
	if client != first {
		t.Fatalf("首次创建应返回首个 client: got=%p want=%p", client, first)
	}

	manager.StartRound("agent:nexus:ws:dm:test", "round-1", nil)
	if err := manager.RecycleClient(context.Background(), "agent:nexus:ws:dm:test"); err != nil {
		t.Fatalf("回收 client 失败: %v", err)
	}
	if first.disconnectCalls != 1 {
		t.Fatalf("回收旧 client 时应调用 Disconnect: got=%d want=1", first.disconnectCalls)
	}
	roundIDs := manager.GetRunningRoundIDs("agent:nexus:ws:dm:test")
	if len(roundIDs) != 1 || roundIDs[0] != "round-1" {
		t.Fatalf("回收 client 后不应丢失 round 状态: %+v", roundIDs)
	}

	next, err := manager.GetOrCreate(context.Background(), "agent:nexus:ws:dm:test", agentclient.Options{
		CWD: "/tmp/b",
	})
	if err != nil {
		t.Fatalf("重建 client 失败: %v", err)
	}
	if next != second {
		t.Fatalf("回收后应创建新 client: got=%p want=%p", next, second)
	}
}

func TestManagerRecycleClientIgnoresBrokenDisconnectError(t *testing.T) {
	first := &fakeRuntimeClient{
		disconnectErr: errors.New("process: command exited with error: signal: killed"),
	}
	second := &fakeRuntimeClient{}
	factory := &fakeRuntimeFactory{clients: []*fakeRuntimeClient{first, second}}
	manager := NewManagerWithFactory(factory)

	client, err := manager.GetOrCreate(context.Background(), "agent:nexus:ws:dm:test", agentclient.Options{
		CWD: "/tmp/a",
	})
	if err != nil {
		t.Fatalf("首次创建 client 失败: %v", err)
	}
	if client != first {
		t.Fatalf("首次创建应返回首个 client: got=%p want=%p", client, first)
	}

	if err := manager.RecycleClient(context.Background(), "agent:nexus:ws:dm:test"); err != nil {
		t.Fatalf("坏 client 的 disconnect 错误不应阻断回收: %v", err)
	}

	next, err := manager.GetOrCreate(context.Background(), "agent:nexus:ws:dm:test", agentclient.Options{
		CWD: "/tmp/b",
	})
	if err != nil {
		t.Fatalf("回收后应继续创建新 client: %v", err)
	}
	if next != second {
		t.Fatalf("回收后应创建新 client: got=%p want=%p", next, second)
	}
}
