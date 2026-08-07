// INPUT: 两个 Room Agent、一个活跃 slot、持久暂停/恢复与后续多目标用户输入。
// OUTPUT: 只中断暂停成员、保留其队列、允许同 Room 其他成员运行并在恢复后续派发。
// POS: Room 成员参与控制跨持久化、runtime 与多 Agent 队列的集成回归测试。
package realtime_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	_ "modernc.org/sqlite"
)

func TestRealtimeServicePausesOneMemberAndResumesPreservedWork(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	pausedAgent := createTestAgent(t, agentService, ctx, "暂停目标")
	activeAgent := createTestAgent(t, agentService, ctx, "继续目标")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{pausedAgent.AgentID, activeAgent.AgentID},
		Name:     "成员暂停恢复集成测试",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 Room 失败: %v", err)
	}

	pausedClient := newFakeRoomClient()
	pausedPrompts := make(chan string, 2)
	var pausedQueryMu sync.Mutex
	pausedQueryCount := 0
	pausedClient.onQuery = func(_ context.Context, prompt string) error {
		pausedQueryMu.Lock()
		pausedQueryCount++
		queryIndex := pausedQueryCount
		pausedQueryMu.Unlock()
		pausedPrompts <- prompt
		if queryIndex == 2 {
			go sendFakeAssistantResult(
				pausedClient,
				"paused-agent-resumed-result",
				"恢复后完成保留工作。",
			)
		}
		return nil
	}
	pausedClient.onInterrupt = func(_ context.Context) {
		go func() {
			pausedClient.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: pausedClient.sessionID,
				UUID:      "paused-agent-interrupted",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	activeClient := newFakeRoomClient()
	activePrompts := make(chan string, 1)
	activeClient.onQuery = func(_ context.Context, prompt string) error {
		activePrompts <- prompt
		go sendFakeAssistantResult(
			activeClient,
			"active-agent-result",
			"未暂停成员正常完成。",
		)
		return nil
	}

	permission := permissionctx.NewContext()
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{pausedClient, activeClient}},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-member-participation")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "先执行一项长任务",
		TargetAgentIDs: []string{pausedAgent.AgentID},
		RoundID:        "room-round-before-member-pause",
	}); err != nil {
		t.Fatalf("启动待暂停成员失败: %v", err)
	}
	waitForParticipationPrompt(t, pausedPrompts, "暂停前首个 prompt")
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart && event.AgentID == pausedAgent.AgentID
	})

	updated, err := service.SetRoomMemberParticipation(
		ctx,
		roomContext.Room.ID,
		pausedAgent.AgentID,
		true,
	)
	if err != nil {
		t.Fatalf("暂停成员参与失败: %v", err)
	}
	if !roomMemberParticipationPaused(updated.Members, pausedAgent.AgentID) {
		t.Fatalf("暂停状态未持久化: %+v", updated.Members)
	}
	pausedClient.mu.Lock()
	interruptCalls := pausedClient.interruptCalls
	pausedClient.mu.Unlock()
	if interruptCalls != 1 {
		t.Fatalf("暂停成员应精确中断一次当前 slot: got %d", interruptCalls)
	}

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "两位成员分别处理后续任务",
		TargetAgentIDs: []string{pausedAgent.AgentID, activeAgent.AgentID},
		RoundID:        "room-round-during-member-pause",
	}); err != nil {
		t.Fatalf("暂停期间发送多目标输入失败: %v", err)
	}
	activePrompt := waitForParticipationPrompt(t, activePrompts, "未暂停成员 prompt")
	if !strings.Contains(activePrompt, "两位成员分别处理后续任务") {
		t.Fatalf("未暂停成员 prompt 缺少用户输入: %s", activePrompt)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["round_id"] == "room-round-during-member-pause" &&
			event.Data["status"] == "finished"
	})

	pausedLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  pausedAgent.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, pausedAgent.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	queueStore := workspacestore.NewInputQueueStore(cfg.WorkspacePath)
	queuedItems, err := queueStore.Snapshot(pausedLocation)
	if err != nil {
		t.Fatalf("读取暂停成员队列失败: %v", err)
	}
	if len(queuedItems) != 1 ||
		queuedItems[0].AgentID != pausedAgent.AgentID ||
		queuedItems[0].ID != "room-round-during-member-pause" {
		t.Fatalf("暂停成员输入未按精确目标保留: %+v", queuedItems)
	}
	pausedQueryMu.Lock()
	queriesBeforeResume := pausedQueryCount
	pausedQueryMu.Unlock()
	if queriesBeforeResume != 1 {
		t.Fatalf("暂停期间不应启动第二个 runtime query: got %d", queriesBeforeResume)
	}

	updated, err = service.SetRoomMemberParticipation(
		ctx,
		roomContext.Room.ID,
		pausedAgent.AgentID,
		false,
	)
	if err != nil {
		t.Fatalf("恢复成员参与失败: %v", err)
	}
	if roomMemberParticipationPaused(updated.Members, pausedAgent.AgentID) {
		t.Fatalf("恢复状态未持久化: %+v", updated.Members)
	}
	resumedPrompt := waitForParticipationPrompt(t, pausedPrompts, "恢复后的保留工作 prompt")
	if !strings.Contains(resumedPrompt, "两位成员分别处理后续任务") {
		t.Fatalf("恢复后 prompt 未携带保留输入: %s", resumedPrompt)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeAgentRoundStatus &&
			event.Data["agent_id"] == pausedAgent.AgentID &&
			event.Data["status"] == "finished"
	})
	queuedItems, err = queueStore.Snapshot(pausedLocation)
	if err != nil {
		t.Fatalf("恢复后读取成员队列失败: %v", err)
	}
	if len(queuedItems) != 0 {
		t.Fatalf("恢复派发后队列应为空: %+v", queuedItems)
	}
	activeClient.mu.Lock()
	activeQueryCount := len(activeClient.queryPrompts)
	activeClient.mu.Unlock()
	if activeQueryCount != 1 {
		t.Fatalf("恢复暂停成员不得重启其他成员: active queries = %d", activeQueryCount)
	}
}

func waitForParticipationPrompt(
	t *testing.T,
	prompts <-chan string,
	label string,
) string {
	t.Helper()
	select {
	case prompt := <-prompts:
		return prompt
	case <-time.After(3 * time.Second):
		t.Fatalf("等待%s超时", label)
		return ""
	}
}

func roomMemberParticipationPaused(
	members []protocol.MemberRecord,
	agentID string,
) bool {
	for _, member := range members {
		if member.MemberType == protocol.MemberTypeAgent && member.MemberAgentID == agentID {
			return member.ParticipationPaused
		}
	}
	return false
}
