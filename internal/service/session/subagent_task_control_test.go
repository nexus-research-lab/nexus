package session_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type subagentControlCall struct {
	taskID  string
	message string
}

type subagentControlClient struct {
	mu       sync.Mutex
	stopped  []string
	messages []subagentControlCall
}

func (c *subagentControlClient) Connect(context.Context) error { return nil }

func (c *subagentControlClient) Query(context.Context, string) error { return nil }

func (c *subagentControlClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	messages := make(chan sdkprotocol.ReceivedMessage)
	close(messages)
	return messages
}

func (c *subagentControlClient) Interrupt(context.Context) error { return nil }

func (c *subagentControlClient) StopTask(_ context.Context, taskID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopped = append(c.stopped, taskID)
	return nil
}

func (c *subagentControlClient) SendTaskMessage(_ context.Context, taskID string, message string, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, subagentControlCall{taskID: taskID, message: message})
	return nil
}

func (c *subagentControlClient) RemoveMessages(context.Context, []string) error { return nil }

func (c *subagentControlClient) SetPermissionMode(context.Context, sdkpermission.Mode) error {
	return nil
}

func (c *subagentControlClient) Disconnect(context.Context) error { return nil }

func (c *subagentControlClient) Reconfigure(context.Context, agentclient.Options) error { return nil }

func (c *subagentControlClient) SessionID() string { return "" }

type subagentControlFactory struct {
	client runtimectx.Client
}

func (f subagentControlFactory) New(agentclient.Options) runtimectx.Client { return f.client }

func TestSessionServiceRoutesCompletedNXSTaskControlsToRoomHostRuntime(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)
	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)
	runtimeManager := runtimectx.NewManager()
	sessionService.SetRuntimeManager(runtimeManager)

	conversationID := "conversation-nxs-task-control"
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	runtimeSessionKey := protocol.BuildRoomAgentSessionKey(conversationID, "host-agent", protocol.RoomTypeGroup)
	client := &subagentControlClient{}
	if _, err = runtimeManager.GetOrCreateWithFactory(
		context.Background(),
		runtimeSessionKey,
		agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS}},
		subagentControlFactory{client: client},
	); err != nil {
		t.Fatalf("创建 nxs slot runtime 失败: %v", err)
	}

	history := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	if err = history.AppendInlineMessage(conversationID, protocol.Message{
		"message_id":  "task-notification-1",
		"session_key": sharedSessionKey,
		"agent_id":    "host-agent",
		"round_id":    "round-1",
		"role":        "system",
		"timestamp":   int64(1000),
		"metadata": map[string]any{
			"subtype":          "task_notification",
			"task_id":          "task-1",
			"agent_id":         "sdk-subagent-1",
			"child_session_id": "child-session-1",
			"runtime_kind":     "nxs",
			"status":           "completed",
		},
	}); err != nil {
		t.Fatalf("写入 Room task history 失败: %v", err)
	}

	list, err := sessionService.ListSubagentTasks(context.Background(), sharedSessionKey)
	if err != nil {
		t.Fatalf("ListSubagentTasks() error = %v", err)
	}
	if list.RuntimeKind != "nxs" || !list.Capabilities.SendMessage || !list.Capabilities.Resume {
		t.Fatalf("nxs list capabilities = %+v runtime=%q", list.Capabilities, list.RuntimeKind)
	}
	if len(list.Items) != 1 || list.Items[0].AgentID != "sdk-subagent-1" || list.Items[0].HostAgentID != "host-agent" {
		t.Fatalf("task identity = %+v", list.Items)
	}

	result, err := sessionService.SendSubagentTaskMessage(context.Background(), sharedSessionKey, "task-1", "继续检查")
	if err != nil {
		t.Fatalf("completed nxs task 应允许续聊: %v", err)
	}
	if result.Status != "queued" {
		t.Fatalf("SendSubagentTaskMessage() = %+v", result)
	}
	if _, err = sessionService.StopSubagentTask(context.Background(), sharedSessionKey, "task-1"); err != nil {
		t.Fatalf("StopSubagentTask() error = %v", err)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.messages) != 1 || client.messages[0].taskID != "task-1" || client.messages[0].message != "继续检查" {
		t.Fatalf("task messages = %+v", client.messages)
	}
	if len(client.stopped) != 1 || client.stopped[0] != "task-1" {
		t.Fatalf("stopped tasks = %+v", client.stopped)
	}
}

func TestSessionServiceRejectsCCSendBeforeRuntimeWire(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)
	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)
	runtimeManager := runtimectx.NewManager()
	sessionService.SetRuntimeManager(runtimeManager)

	conversationID := "conversation-cc-task-control"
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	runtimeSessionKey := protocol.BuildRoomAgentSessionKey(conversationID, "host-agent", protocol.RoomTypeGroup)
	client := &subagentControlClient{}
	if _, err = runtimeManager.GetOrCreateWithFactory(
		context.Background(),
		runtimeSessionKey,
		agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude}},
		subagentControlFactory{client: client},
	); err != nil {
		t.Fatalf("创建 CC slot runtime 失败: %v", err)
	}
	history := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	if err = history.AppendInlineMessage(conversationID, protocol.Message{
		"message_id":  "task-started-cc",
		"session_key": sharedSessionKey,
		"agent_id":    "host-agent",
		"round_id":    "round-1",
		"role":        "system",
		"timestamp":   int64(1000),
		"metadata": map[string]any{
			"subtype":      "task_started",
			"task_id":      "task-cc",
			"agent_id":     "sdk-subagent-cc",
			"runtime_kind": "claude",
		},
	}); err != nil {
		t.Fatalf("写入 CC task history 失败: %v", err)
	}

	_, err = sessionService.SendSubagentTaskMessage(context.Background(), sharedSessionKey, "task-cc", "继续")
	if !errors.Is(err, sessionsvc.ErrSubagentOperationUnsupported) {
		t.Fatalf("CC send error = %v, want unsupported", err)
	}
	client.mu.Lock()
	if len(client.messages) != 0 {
		t.Fatalf("CC unsupported 不应进入 wire: %+v", client.messages)
	}
	client.mu.Unlock()
	if _, err = sessionService.StopSubagentTask(context.Background(), sharedSessionKey, "task-cc"); err != nil {
		t.Fatalf("CC stop 应受支持: %v", err)
	}

	if err = runtimeManager.CloseSession(context.Background(), runtimeSessionKey); err != nil {
		t.Fatalf("关闭 CC runtime 失败: %v", err)
	}
	list, err := sessionService.ListSubagentTasks(context.Background(), sharedSessionKey)
	if err != nil {
		t.Fatalf("离线后读取 task 失败: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].RuntimeKind != "claude" || list.Items[0].Capabilities.SendMessage {
		t.Fatalf("离线 task 应从 metadata 恢复 CC 能力: %+v", list.Items)
	}
}
