// input_queue_test.go 覆盖 DM 输入队列、游标、配置溯源、派发锁与中断测试。
package dm

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	queueadmissionstore "github.com/nexus-research-lab/nexus/internal/storage/queueadmission"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

func TestServiceGuidancePreflightFailureKeepsQueuedInput(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.onInterrupt = func(_ context.Context) {
		client.sendResultAsync(client.sessionID, "result-guide-preflight-cleanup", "interrupted", "")
	}
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client}),
		permission,
	)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sender := newDMTestSender("sender-guide-preflight-failure")
	sessionKey := "agent:" + newPlainDMAgent(t, agentService).AgentID + ":ws:dm:test-guide-preflight-failure"
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先查项目",
		RoundID:    "round-guide-preflight-1",
	}); err != nil {
		t.Fatalf("启动运行中 DM round 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")
	t.Cleanup(func() {
		if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey, UserStop: true}); err != nil {
			t.Errorf("清理运行中 round 失败: %v", err)
			return
		}
		// Stopping retains the invalid input without attempting another dispatch.
		_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
		if err != nil {
			t.Error(err)
			return
		}
		paused, err := service.InputQueue.DispatchPaused(location)
		if err != nil || !paused {
			t.Errorf("queue stop gate = %v, %v", paused, err)
		}
		// 等在途派发退出临界区。
		if unlock, lockErr := service.dispatchLocks.lock(context.Background(), sessionKey); lockErr == nil {
			unlock()
		}
	})

	if err := service.HandleChat(context.Background(), Request{
		SessionKey:                  sessionKey,
		Content:                     "补充一个失效附件",
		RoundID:                     "round-guide-preflight-2",
		UserMessageID:               "msg-user-guide-preflight-2",
		DeliveryPolicy:              protocol.ChatDeliveryPolicyGuide,
		TrustedConfigurationContext: true,
		Attachments: []protocol.ChatAttachment{{
			FileName:         "missing.txt",
			WorkspacePath:    "missing.txt",
			WorkspaceAgentID: "missing-agent",
			Scope:            protocol.ChatAttachmentScopeAgentWorkspace,
			Kind:             protocol.ChatAttachmentKindFile,
		}},
	}); err != nil {
		t.Fatalf("写入待预检 DM 引导失败: %v", err)
	}

	_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("解析 DM 队列位置失败: %v", err)
	}
	before, err := service.InputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取预检前 DM 引导队列失败: %v", err)
	}
	if len(before) != 1 || before[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide {
		t.Fatalf("预检前应持久保留一条引导: %+v", before)
	}

	output, hookErr := service.inputQueueGuidanceHook(sessionKey, location)(
		context.Background(),
		sdkhook.Input{EventName: sdkhook.EventPostToolUse},
		"tool-1",
	)
	if hookErr == nil {
		t.Fatal("失效附件应使 DM 引导预渲染失败")
	}
	if output.SpecificOutput != nil {
		t.Fatalf("预渲染失败不应向模型注入部分上下文: %+v", output)
	}

	after, err := service.InputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取预检失败后的 DM 引导队列失败: %v", err)
	}
	if len(after) != 1 ||
		after[0].ID != before[0].ID ||
		after[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide ||
		after[0].RootRoundID != before[0].RootRoundID ||
		len(after[0].Attachments) != 1 {
		t.Fatalf("预渲染失败后不应丢失或改写引导: before=%+v after=%+v", before, after)
	}
}

func TestServiceInputQueueGuideWaitsForPostToolUse(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sender := newDMTestSender("sender-guide-input-queue")
	sessionKey := "agent:" + newPlainDMAgent(t, agentService).AgentID + ":ws:dm:test-guide-input-queue"
	t.Cleanup(func() {
		_ = runtimeManager.CloseSession(context.Background(), sessionKey)
	})
	permission.BindSession(sessionKey, sender)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先查项目",
		RoundID:    "round-guide-input-queue-1",
	}); err != nil {
		t.Fatalf("启动运行中 DM round 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if _, err := service.HandleInputQueue(context.Background(), InputQueueRequest{
		SessionKey:      sessionKey,
		ClientMessageID: "client-message-guide-input-queue",
		Action:          "enqueue",
		Content:         "路径发给我吧",
		DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
	}); err != nil {
		t.Fatalf("写入 DM 待发送队列失败: %v", err)
	}
	_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("解析 DM 队列位置失败: %v", err)
	}
	items, err := service.InputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取 DM 待发送队列失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("待发送队列应保留一条消息: %+v", items)
	}
	itemID := items[0].ID

	if _, err := service.HandleInputQueue(context.Background(), InputQueueRequest{
		SessionKey: sessionKey,
		Action:     "guide",
		ItemID:     itemID,
	}); err != nil {
		t.Fatalf("标记 DM 引导队列失败: %v", err)
	}
	items, err = service.InputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取标记后的 DM 待发送队列失败: %v", err)
	}
	if len(items) != 1 ||
		items[0].ID != itemID ||
		items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide ||
		items[0].RootRoundID != "round-guide-input-queue-1" {
		t.Fatalf("点击引导后应锚定当前运行 round，避免被其他回复消费: %+v", items)
	}

	var additionalContext string
	var onApplied func(sdkhook.AppliedAck)
	for _, matcher := range factory.options[0].Hooks.Matchers[sdkhook.EventPostToolUse] {
		for _, hook := range matcher.Hooks {
			output, hookErr := hook(context.Background(), sdkhook.Input{
				EventName: sdkhook.EventPostToolUse,
			}, "tool-1")
			if hookErr != nil {
				t.Fatalf("执行 PostToolUse hook 失败: %v", hookErr)
			}
			if output.SpecificOutput != nil && output.SpecificOutput.AdditionalContext != "" {
				text := output.SpecificOutput.AdditionalContext
				additionalContext = text
			}
			if output.OnApplied != nil {
				onApplied = output.OnApplied
			}
		}
	}
	if !strings.Contains(additionalContext, "路径发给我吧") ||
		!strings.Contains(additionalContext, "queue_"+itemID) {
		t.Fatalf("PostToolUse hook 未注入队列引导: %q", additionalContext)
	}
	items, err = service.InputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取待确认 DM 引导失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("首次 hook 返回后必须等待后续可观察行为确认: %+v", items)
	}
	if onApplied == nil {
		t.Fatal("DM guide output 缺少 runtime applied ACK callback")
	}
	if _, err = service.HandleInputQueue(context.Background(), InputQueueRequest{
		SessionKey: sessionKey,
		Action:     "reorder",
		OrderedIDs: []string{itemID},
	}); err == nil {
		t.Fatal("已返回给 runtime 但尚未 applied ACK 的 DM 引导不应允许重排")
	}
	onApplied(sdkhook.AppliedAck{RequestID: "dm-guide-applied-1"})
	var confirmationContext string
	for _, matcher := range factory.options[0].Hooks.Matchers[sdkhook.EventPostToolUse] {
		for _, hook := range matcher.Hooks {
			output, hookErr := hook(context.Background(), sdkhook.Input{
				EventName: sdkhook.EventPostToolUse,
			}, "tool-2")
			if hookErr != nil {
				t.Fatalf("第二次 PostToolUse 确认前次引导失败: %v", hookErr)
			}
			if output.SpecificOutput != nil {
				confirmationContext += output.SpecificOutput.AdditionalContext
			}
		}
	}
	if strings.Contains(confirmationContext, "路径发给我吧") {
		t.Fatalf("确认前次引导时不应在同一 round 重复注入: %q", confirmationContext)
	}
	items, err = service.InputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取确认后的 DM 待发送队列失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("下一次 PostToolUse 应先确认并消费前次引导: %+v", items)
	}

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("清理运行中 round 失败: %v", err)
	}
}

// A claim that survived a crash after its round was persisted must be
// consumed, not dispatched as a second identical round.
func TestSurvivingQueueClaimWithPersistedRoundIsConsumedWithoutReplay(t *testing.T) {
	cfg := newDMTestConfig(t)
	agentService := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sessionKey := "agent:nexus:ws:dm:queue-claim-replay"
	t.Cleanup(func() { _ = runtimeManager.CloseSession(context.Background(), sessionKey) })
	_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.InputQueue.Enqueue(location, protocol.InputQueueItem{ID: "claimed", Content: "already sent"}); err != nil {
		t.Fatal(err)
	}
	if err := service.InputQueue.ClaimDispatch(location, "claimed"); err != nil {
		t.Fatal(err)
	}
	if err := service.History.ForOwner(location.OwnerUserID).AppendRoundMarkerWithOptions(
		location.WorkspacePath, sessionKey, inputQueueItemRoundID(protocol.InputQueueItem{ID: "claimed"}), "already sent", 1, workspacestore.RoundMarkerOptions{},
	); err != nil {
		t.Fatal(err)
	}

	service.dispatchNextInputQueueItemAtLocation(context.Background(), sessionKey, "", location)

	items, err := service.InputQueue.Snapshot(location)
	if err != nil || len(items) != 0 {
		t.Fatalf("surviving claim not consumed: %+v %v", items, err)
	}
	client.mu.Lock()
	queries := len(client.queryPrompts)
	client.mu.Unlock()
	if queries != 0 {
		t.Fatalf("persisted queued round was replayed %d time(s)", queries)
	}
}

func TestTamperedDMQueuePayloadLosesConfigurationContextAndRevokesAdmission(t *testing.T) {
	service, client, sources, _, prompts, db, sessionKey := newDMQueueProvenanceFixture(t)
	ctx := trustedDMQueueContext()

	if err := service.HandleChat(ctx, Request{
		SessionKey:                  sessionKey,
		Content:                     "keep the first round running",
		RoundID:                     "round-tamper-first",
		UserMessageID:               "message-tamper-first",
		TrustedConfigurationContext: true,
	}); err != nil {
		t.Fatal(err)
	}
	expectDMQueueSource(t, sources, "agent")
	expectDMQueuePrompt(t, prompts, "keep the first round running")

	if err := service.HandleChat(ctx, Request{
		SessionKey:                  sessionKey,
		Content:                     "original queued request",
		RoundID:                     "round-tamper-queued",
		UserMessageID:               "message-tamper-queued",
		TrustedConfigurationContext: true,
	}); err != nil {
		t.Fatal(err)
	}
	_, location, err := service.resolveInputQueueLocation(
		context.Background(),
		sessionKey,
		service.Config.DefaultAgentID,
	)
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.InputQueue.Snapshot(location)
	if err != nil || len(items) != 1 {
		t.Fatalf("queued items = %+v, err=%v", items, err)
	}
	tampered := items[0]
	tampered.Content = "tampered destructive request"
	if _, err = service.InputQueue.Enqueue(location, tampered); err != nil {
		t.Fatal(err)
	}

	finishDMQueueRound(client, "result-tamper-first")
	expectDMQueueSource(t, sources, "agent_queue")
	expectDMQueuePrompt(t, prompts, "tampered destructive request")

	status := waitForDMQueueAdmissionStatus(t, db, "round-tamper-queued", queueadmissionstore.StatusRevoked)
	if status != queueadmissionstore.StatusRevoked {
		t.Fatalf("tampered queue admission status = %q, want revoked", status)
	}
	finishDMQueueRound(client, "result-tamper-queued")
}

type rejectingDMQueueAdmissionStore struct {
	err error
}

func (s rejectingDMQueueAdmissionStore) Record(context.Context, queueadmissionstore.Admission) error {
	return s.err
}

func (rejectingDMQueueAdmissionStore) Claim(context.Context, queueadmissionstore.Binding) (queueadmissionstore.Claim, bool, error) {
	return queueadmissionstore.Claim{}, false, nil
}

func (rejectingDMQueueAdmissionStore) Release(context.Context, queueadmissionstore.Claim) error {
	return nil
}

func (rejectingDMQueueAdmissionStore) Consume(context.Context, queueadmissionstore.Claim) error {
	return nil
}

func (rejectingDMQueueAdmissionStore) Revoke(context.Context, queueadmissionstore.Binding) error {
	return nil
}

func TestDMQueueAdmissionFailureKeepsAndRecoversDurableUserInput(t *testing.T) {
	service, client, _, _, prompts, db, sessionKey := newDMQueueProvenanceFixture(t)
	admissionErr := errors.New("queue admission unavailable")
	service.SetQueueAdmissionStore(rejectingDMQueueAdmissionStore{err: admissionErr})
	request := InputQueueRequest{
		SessionKey: sessionKey, ClientMessageID: "client-dm-queue-retained",
		Action: "enqueue", Content: "keep this DM input for retry",
		TrustedConfigurationContext: true,
	}
	if _, err := service.HandleInputQueue(trustedDMQueueContext(), request); !errors.Is(err, admissionErr) {
		t.Fatalf("HandleInputQueue error = %v, want %v", err, admissionErr)
	}
	_, location, err := service.resolveInputQueueLocation(
		context.Background(),
		sessionKey,
		service.Config.DefaultAgentID,
	)
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.InputQueue.Snapshot(location)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 ||
		items[0].ClientMessageID != request.ClientMessageID ||
		items[0].Content != request.Content {
		t.Fatalf("durable DM input was lost after admission failure: %+v", items)
	}

	service.SetQueueAdmissionStore(queueadmissionstore.NewRepository(service.Config, db))
	retry, err := service.HandleInputQueue(trustedDMQueueContext(), request)
	if err != nil {
		t.Fatalf("retry retained DM input: %v", err)
	}
	if !retry.Duplicate || retry.ItemID != items[0].ID {
		t.Fatalf("retry result = %+v, want original item %q", retry, items[0].ID)
	}
	expectDMQueuePrompt(t, prompts, request.Content)
	finishDMQueueRound(client, "result-retained-dm-queue")
}

func trustedDMQueueContext() context.Context {
	return authctx.WithState(context.Background(), authctx.State{AuthRequired: false})
}

func newDMQueueProvenanceFixture(
	t *testing.T,
) (*Service, *fakeDMClient, <-chan string, <-chan authctx.QueuedHumanPrincipalBinding, <-chan string, *sql.DB, string) {
	t.Helper()
	cfg := newDMTestConfig(t)
	db := dmTestDB(t, cfg)

	agentService := newDMAgentService(t, cfg)
	client := newFakeDMClient()
	prompts := make(chan string, 4)
	client.onQuery = func(_ context.Context, prompt string) {
		prompts <- prompt
	}
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeDMFactory{client: client})
	sessionKey := protocol.BuildAgentSessionKey(
		newPlainDMAgent(t, agentService).AgentID,
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"queue-provenance",
		"",
	)
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = runtimeManager.CloseSession(closeCtx, sessionKey)
	})
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	service.SetInboxStore(newDMInboxStore(t, cfg))
	service.SetQueueAdmissionStore(queueadmissionstore.NewRepository(cfg, db))
	sources := make(chan string, 4)
	bindings := make(chan authctx.QueuedHumanPrincipalBinding, 2)
	service.SetMCPServerBuilder(func(
		ctx context.Context,
		_ *protocol.Agent,
		_ string,
		_ string,
		sourceContextType string,
		_ string,
		_ string,
		_ *atomic.Int64,
		_ sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		sources <- sourceContextType
		if binding, ok := authctx.QueuedHumanPrincipalBindingFromContext(ctx); ok {
			bindings <- binding
		}
		return nil
	})
	return service, client, sources, bindings, prompts, db, sessionKey
}

func expectDMQueueSource(t *testing.T, sources <-chan string, want string) {
	t.Helper()
	select {
	case got := <-sources:
		if got != want {
			t.Fatalf("configuration source = %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for configuration source %q", want)
	}
}

func expectDMQueuePrompt(t *testing.T, prompts <-chan string, content string) {
	t.Helper()
	select {
	case prompt := <-prompts:
		if !strings.Contains(prompt, content) {
			t.Fatalf("runtime prompt does not contain %q: %s", content, prompt)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for prompt containing %q", content)
	}
}

func finishDMQueueRound(client *fakeDMClient, resultID string) {
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      resultID,
		Result:    &sdkprotocol.ResultMessage{Subtype: "success", DurationMS: 1, NumTurns: 1},
	}
}

func waitForDMQueueAdmissionStatus(
	t *testing.T,
	db *sql.DB,
	itemID string,
	want string,
) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	status := ""
	for time.Now().Before(deadline) {
		err := db.QueryRow(
			`SELECT status FROM configuration_queue_admissions
WHERE queue_item_id = ? AND scope = 'dm'`,
			itemID,
		).Scan(&status)
		if err == nil && status == want {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	return status
}
