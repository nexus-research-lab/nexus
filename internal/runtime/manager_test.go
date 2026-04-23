package runtime

import (
	"context"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type fakeRuntimeClient struct {
	reconfigureCalls int
	lastOptions      agentclient.Options
}

func (c *fakeRuntimeClient) Connect(context.Context) error { return nil }

func (c *fakeRuntimeClient) Query(context.Context, string) error { return nil }

func (c *fakeRuntimeClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
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
