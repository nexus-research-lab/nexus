// collaboration_test.go 覆盖 Room 成员协作、目标解析、权限栅栏、私域通知与诊断测试。
package realtime

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type systemOnlyRoomContextStore struct {
	contextValue *protocol.ConversationContextAggregate
	userCalls    int
	systemCalls  int
	startedCalls int
	startedID    string
}

func TestRelayMentionsReuseRoomParserWithoutLocalMembers(t *testing.T) {
	round := &activeRoomRound{
		ExecutionOrigin:      "relay",
		Context:              &protocol.ConversationContextAggregate{Conversation: protocol.ConversationRecord{ID: "conversation"}},
		PublicAgentDirectory: map[string]string{"self": "Self", "remote": "Lucy", "duplicate-a": "Sam", "duplicate-b": "Sam"},
	}
	mentions := buildRoomMentionAnnotations(relayMentionContext(round), "self", "message", []roomMentionTextBlock{{text: "@Lucy 请处理 `@Lucy` @Sam @Self @outsider"}})
	if len(mentions) != 1 || mentions[0].AgentID != "remote" {
		t.Fatalf("远端解析未复用代码块、歧义、自身过滤: %+v", mentions)
	}
	if len(round.Context.Members) != 0 || len(round.Context.MemberAgents) != 0 {
		t.Fatal("远端目录污染本机执行成员")
	}
	message := protocol.Message{"message_id": "message", "role": "assistant", "is_complete": true,
		"content":        []map[string]any{{"type": "text", "text": "@Lucy 请处理"}},
		"agent_mentions": []protocol.AgentMention{{AgentID: "forged"}},
	}
	slot := &activeRoomSlot{AgentID: "self"}
	if err := (&Service{}).annotatePublicAssistantMessage(round, slot, message); err != nil {
		t.Fatal(err)
	}
	derived := protocolAgentMentions(message["agent_mentions"])
	if len(derived) != 1 || derived[0].AgentID != "remote" {
		t.Fatalf("在线消息未重新派生标注: %+v", derived)
	}
	if wakes := publicMentionWakesFromMessage(round, slot, message, "@Lucy 请处理", nil); len(wakes) != 0 {
		t.Fatal("在线标注生成了本机唤醒")
	}
}

func (s *systemOnlyRoomContextStore) GetConversationContext(
	context.Context,
	string,
) (*protocol.ConversationContextAggregate, error) {
	s.userCalls++
	return nil, errors.New("request-scoped lookup must not be used by startup recovery")
}

func (s *systemOnlyRoomContextStore) GetConversationContextForSystem(
	context.Context,
	string,
) (*protocol.ConversationContextAggregate, error) {
	s.systemCalls++
	return s.contextValue, nil
}

func (*systemOnlyRoomContextStore) UpdateSessionRuntimeIdentity(
	context.Context,
	string,
	string,
	string,
) error {
	return nil
}

func (*systemOnlyRoomContextStore) TouchConversationActivity(
	context.Context,
	string,
	time.Time,
) error {
	return nil
}

func (s *systemOnlyRoomContextStore) MarkConversationStarted(
	_ context.Context,
	conversationID string,
	_ time.Time,
) error {
	s.startedCalls++
	s.startedID = conversationID
	return nil
}

func (*systemOnlyRoomContextStore) BuildRoomSkillPrompt(
	context.Context,
	[]string,
) (string, error) {
	return "", nil
}

func TestPublicHandoffReconcilerRestoresGoalDirectedWakeAfterQueueDispatchCrash(t *testing.T) {
	const (
		ownerUserID    = "owner-goal-directed-recovery"
		conversationID = "conversation-goal-directed-recovery"
		roomID         = "room-goal-directed-recovery"
		sourceAgentID  = "agent-goal-lead"
		targetAgentID  = "agent-goal-peer"
	)
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	root := appfs.UsersRoot()
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	runtimeSessionKey := protocol.BuildRoomAgentSessionKey(
		conversationID,
		targetAgentID,
		protocol.RoomTypeGroup,
	)
	workspacePath := filepath.Join(appfs.UserWorkspaceRoot(ownerUserID), targetAgentID)
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			ID: roomID, OwnerUserID: ownerUserID, RoomType: protocol.RoomTypeGroup,
			PrivateMessagesEnabled: true,
		},
		Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: roomID},
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: sourceAgentID},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: targetAgentID},
		},
		MemberAgents: []protocol.Agent{
			{AgentID: sourceAgentID},
			{AgentID: targetAgentID, WorkspacePath: workspacePath},
		},
		Sessions: []protocol.SessionRecord{{
			ID: "session-goal-directed-target", ConversationID: conversationID, AgentID: targetAgentID,
		}},
	}
	message := protocol.RoomDirectedMessageRecord{
		MessageID: "directed-goal-source", RoomID: roomID, ConversationID: conversationID,
		SourceAgentID: sourceAgentID, Recipients: []string{targetAgentID},
		WakeTargets: []string{targetAgentID}, Content: "private Goal evidence",
		WakePolicy:  protocol.RoomWakePolicyImmediate,
		ReplyRoute:  protocol.RoomReplyRoute{Mode: protocol.RoomReplyRoutePrivate, Recipients: []string{sourceAgentID}},
		RootRoundID: "goal-root", CausedByRoundID: "goal-source-round",
		GoalCollaborationBinding: &protocol.GoalCollaborationBinding{GoalID: "goal-room", ObjectiveRevision: 4},
		Timestamp:                time.Now().UnixMilli(),
	}
	directed := workspacestore.NewRoomDirectedMessageStore(root)
	if err := directed.AppendMessage(ownerUserID, message); err != nil {
		t.Fatal(err)
	}
	handoffs := workspacestore.NewRoomPublicHandoffStore(root)
	handoffID := roomDirectedGoalHandoffID(conversationID, message.MessageID, targetAgentID)
	if _, _, err := handoffs.Detect(ownerUserID, workspacestore.RoomPublicHandoff{
		HandoffID: handoffID, ConversationID: conversationID, RoomID: roomID,
		RootRoundID: message.RootRoundID, SourceAgentRoundID: message.CausedByRoundID,
		SourceMessageID: message.MessageID, SourceAgentID: sourceAgentID,
		TargetAgentID: targetAgentID, Content: roomDirectedMessageWakePrompt,
		ReplyRoute: message.ReplyRoute, QueueSource: protocol.InputQueueSourceAgentRoomMessage,
		GoalCollaborationBinding: message.GoalCollaborationBinding,
	}); err != nil {
		t.Fatal(err)
	}
	if err := handoffs.MarkSourceFinished(ownerUserID, conversationID, handoffID); err != nil {
		t.Fatal(err)
	}
	if err := handoffs.MarkQueued(ownerUserID, conversationID, handoffID, "lost-queue-item"); err != nil {
		t.Fatal(err)
	}
	rooms := &systemOnlyRoomContextStore{contextValue: contextValue}
	busySlot := &activeRoomSlot{
		AgentID:           targetAgentID,
		AgentRoundID:      "busy-goal-directed-round",
		RuntimeSessionKey: runtimeSessionKey,
		WorkspacePath:     workspacePath,
	}
	busySlot.setStatus("running")
	wakeTimers := newRoomWakeTimerRegistry()
	defer wakeTimers.Stop()
	service := &Service{
		rooms: rooms, directedMessages: directed, publicHandoffs: handoffs,
		Host:       runtimehost.Host{InputQueue: workspacestore.NewInputQueueStore(root), Permission: permissionctx.NewContext()},
		wakeTimers: wakeTimers,
		goals: &fakeRoomGoalContextProvider{runtimeGoals: map[string]*protocol.Goal{
			sharedSessionKey: {
				ID: "goal-room", SessionKey: sharedSessionKey, Status: protocol.GoalStatusActive,
				Metadata: map[string]any{protocol.GoalMetadataObjectiveRevision: int64(4)},
			},
		}},
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"busy-goal-directed": {
				SessionKey: sharedSessionKey, RoomID: roomID,
				ConversationID: conversationID, RootRoundID: "busy-goal-root",
				Slots: map[string]*activeRoomSlot{"busy": busySlot},
			},
		}),
	}
	if _, err := service.StartPublicHandoffReconciler(context.Background()); err != nil {
		t.Fatal(err)
	}
	location := workspacestore.InputQueueLocation{
		OwnerUserID: ownerUserID, Scope: protocol.InputQueueScopeRoom,
		WorkspacePath: workspacePath,
		SessionKey:    runtimeSessionKey,
		RoomID:        roomID, ConversationID: conversationID,
	}
	items, err := service.InputQueue.Snapshot(location)
	if err != nil || len(items) != 1 || items[0].HandoffID != handoffID ||
		items[0].GoalCollaborationBinding == nil ||
		items[0].GoalCollaborationBinding.ObjectiveRevision != 4 {
		t.Fatalf("recovered Goal directed queue = %+v err=%v", items, err)
	}
}

func TestPublicHandoffReconcilerRepairsLegacySuppressedGoalRootFromHostFacts(t *testing.T) {
	const (
		ownerUserID        = "owner-legacy-goal-handback"
		conversationID     = "conversation-legacy-goal-handback"
		roomID             = "room-legacy-goal-handback"
		goalID             = "goal-legacy-goal-handback"
		leadAgentID        = "agent-legacy-goal-lead"
		targetAgentID      = "agent-legacy-goal-peer"
		rootRoundID        = "goal_continuation_legacy_root"
		sourceAgentRoundID = "agent-round-legacy-source"
		targetAgentRoundID = "agent-round-legacy-target"
	)
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	root := appfs.UsersRoot()
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			ID: roomID, OwnerUserID: ownerUserID, RoomType: protocol.RoomTypeGroup,
		},
		Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: roomID},
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: leadAgentID},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: targetAgentID},
		},
	}
	handoffs := workspacestore.NewRoomPublicHandoffStore(root)
	handoff := workspacestore.RoomPublicHandoff{
		HandoffID: "handoff-legacy-goal-handback", ConversationID: conversationID,
		RoomID: roomID, RootRoundID: rootRoundID,
		SourceAgentRoundID: sourceAgentRoundID,
		SourceMessageID:    "message-legacy-goal-source", SourceAgentID: leadAgentID,
		TargetAgentID: targetAgentID, Content: "legacy collaborator check",
		QueueSource: protocol.InputQueueSourceAgentPublicMention,
	}
	if _, _, err := handoffs.Detect(ownerUserID, handoff); err != nil {
		t.Fatal(err)
	}
	if err := handoffs.MarkSourceFinished(ownerUserID, conversationID, handoff.HandoffID); err != nil {
		t.Fatal(err)
	}
	if _, claimed, err := handoffs.Claim(ownerUserID, conversationID, handoff.HandoffID); err != nil || !claimed {
		t.Fatalf("claim=%t err=%v", claimed, err)
	}
	if err := handoffs.MarkStarted(ownerUserID, conversationID, handoff.HandoffID, "legacy-target-root"); err != nil {
		t.Fatal(err)
	}
	if err := handoffs.MarkTerminal(ownerUserID, conversationID, handoff.HandoffID, "finished"); err != nil {
		t.Fatal(err)
	}
	history := workspacestore.NewRoomHistoryStore(root)
	if err := history.AppendInlineMessage(ownerUserID, conversationID, protocol.Message{
		"message_id": "assistant-legacy-target", "role": "assistant",
		"round_id": rootRoundID, "agent_round_id": targetAgentRoundID,
		"agent_id": targetAgentID, "is_complete": true,
		"content":   []map[string]any{{"type": "text", "text": "核对完成，可以继续收尾。"}},
		"timestamp": int64(200),
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	provider := &fakeRoomGoalContextProvider{
		runtimeGoals: map[string]*protocol.Goal{
			sessionKey: {
				ID: goalID, SessionKey: sessionKey, Status: protocol.GoalStatusActive,
				EmptyProgressCount: 1,
				Metadata: map[string]any{
					protocol.GoalMetadataObjectiveRevision:             int64(2),
					protocol.GoalMetadataRoomGoalLeadAgentID:           leadAgentID,
					protocol.GoalMetadataRoomGoalCollaborationRequired: true,
				},
			},
		},
		events: []protocol.GoalEvent{
			{ID: "event-created", GoalID: goalID, EventType: "created", CreatedAt: now.Add(-time.Minute)},
			{ID: "event-suppressed", GoalID: goalID, EventType: "continuation_suppressed", RoundID: sourceAgentRoundID, CreatedAt: now},
		},
	}
	service := &Service{
		rooms:          &systemOnlyRoomContextStore{contextValue: contextValue},
		publicHandoffs: handoffs, roomHistory: history,
		directedMessages: workspacestore.NewRoomDirectedMessageStore(root),
		goals:            provider,
	}
	if _, err := service.StartPublicHandoffReconciler(context.Background()); err != nil {
		t.Fatal(err)
	}
	provider.mu.Lock()
	if len(provider.collabEvidence) != 1 ||
		provider.collabEvidence[0] != targetAgentRoundID+":"+targetAgentID ||
		len(provider.handbacks) != 1 {
		provider.mu.Unlock()
		t.Fatalf("evidence=%+v handbacks=%+v", provider.collabEvidence, provider.handbacks)
	}
	provider.mu.Unlock()
	stored, exists, err := handoffs.Get(ownerUserID, conversationID, handoff.HandoffID)
	if err != nil || !exists || stored.GoalCollaborationBinding == nil ||
		stored.GoalCollaborationBinding.GoalID != goalID ||
		stored.GoalCollaborationBinding.ObjectiveRevision != 2 ||
		!stored.GoalHandbackSettled {
		t.Fatalf("stored=%+v exists=%t err=%v", stored, exists, err)
	}
}

func TestGoalDirectedWakeConsumesRetargetedRevisionWithoutStartingAgent(t *testing.T) {
	const (
		ownerUserID    = "owner-goal-wake-stale"
		conversationID = "conversation-goal-wake-stale"
		roomID         = "room-goal-wake-stale"
		sourceAgentID  = "agent-goal-wake-source"
		targetAgentID  = "agent-goal-wake-target"
	)
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	root := appfs.UsersRoot()
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			ID: roomID, OwnerUserID: ownerUserID, RoomType: protocol.RoomTypeGroup,
			PrivateMessagesEnabled: true,
		},
		Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: roomID},
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: sourceAgentID},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: targetAgentID},
		},
	}
	message := protocol.RoomDirectedMessageRecord{
		MessageID: "directed-goal-wake-stale", RoomID: roomID,
		ConversationID: conversationID, SourceAgentID: sourceAgentID,
		Recipients: []string{targetAgentID}, WakeTargets: []string{targetAgentID},
		WakePolicy: protocol.RoomWakePolicyImmediate,
		GoalCollaborationBinding: &protocol.GoalCollaborationBinding{
			GoalID: "goal-wake-stale", ObjectiveRevision: 1,
		},
		Timestamp: time.Now().UnixMilli(),
	}
	handoffs := workspacestore.NewRoomPublicHandoffStore(root)
	handoffID := roomDirectedGoalHandoffID(conversationID, message.MessageID, targetAgentID)
	if _, _, err := handoffs.Detect(ownerUserID, workspacestore.RoomPublicHandoff{
		HandoffID: handoffID, ConversationID: conversationID, RoomID: roomID,
		RootRoundID: message.MessageID, SourceAgentRoundID: message.MessageID,
		SourceMessageID: message.MessageID, SourceAgentID: sourceAgentID,
		TargetAgentID: targetAgentID, Content: roomDirectedMessageWakePrompt,
		QueueSource:              protocol.InputQueueSourceAgentRoomMessage,
		GoalCollaborationBinding: message.GoalCollaborationBinding,
	}); err != nil {
		t.Fatal(err)
	}
	wakes := workspacestore.NewRoomDirectedMessageWakeStore(root)
	wake := workspacestore.RoomDirectedMessageWake{
		WakeID: message.MessageID, OwnerUserID: ownerUserID, Message: message,
		DueAt: time.Now().UnixMilli(), CreatedAt: time.Now().UnixMilli(),
	}
	if err := wakes.Schedule(wake); err != nil {
		t.Fatal(err)
	}
	wakeTimers := newRoomWakeTimerRegistry()
	defer wakeTimers.Stop()
	service := &Service{
		rooms:          &authorityFenceRoomStore{contextValue: contextValue},
		directedWakes:  wakes,
		publicHandoffs: handoffs,
		wakeTimers:     wakeTimers,
		goals: &fakeRoomGoalContextProvider{runtimeGoals: map[string]*protocol.Goal{
			sessionKey: {
				ID: "goal-wake-stale", SessionKey: sessionKey, Status: protocol.GoalStatusActive,
				Metadata: map[string]any{protocol.GoalMetadataObjectiveRevision: int64(2)},
			},
		}},
	}
	service.executePersistedRoomDirectedWake(wake)
	pending, err := wakes.Pending(ownerUserID)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending stale wake = %+v err=%v, want consumed", pending, err)
	}
	handoff, exists, err := handoffs.Get(ownerUserID, conversationID, handoffID)
	if err != nil || !exists || handoff.Status != "interrupted" {
		t.Fatalf("stale wake handoff = %+v exists=%t err=%v", handoff, exists, err)
	}
}

func TestPublicHandoffReconcilerDeletesRetargetedGoalQueueItem(t *testing.T) {
	const (
		ownerUserID    = "owner-goal-reconcile-stale"
		conversationID = "conversation-goal-reconcile-stale"
		roomID         = "room-goal-reconcile-stale"
		targetAgentID  = "agent-goal-reconcile-target"
	)
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	root := appfs.UsersRoot()
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	workspacePath := filepath.Join(appfs.UserWorkspaceRoot(ownerUserID), targetAgentID)
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			ID: roomID, OwnerUserID: ownerUserID, RoomType: protocol.RoomTypeGroup,
		},
		Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: roomID},
		Members: []protocol.MemberRecord{{
			MemberType: protocol.MemberTypeAgent, MemberAgentID: targetAgentID,
		}},
		MemberAgents: []protocol.Agent{{AgentID: targetAgentID, WorkspacePath: workspacePath}},
	}
	handoffs := workspacestore.NewRoomPublicHandoffStore(root)
	handoff := workspacestore.RoomPublicHandoff{
		HandoffID: "handoff-goal-reconcile-stale", ConversationID: conversationID,
		RoomID: roomID, RootRoundID: "root-goal-reconcile-stale",
		SourceMessageID: "source-goal-reconcile-stale", SourceAgentID: "agent-source",
		TargetAgentID: targetAgentID, Content: "old queued goal work",
		QueueSource: protocol.InputQueueSourceAgentRoomMessage,
		GoalCollaborationBinding: &protocol.GoalCollaborationBinding{
			GoalID: "goal-reconcile-stale", ObjectiveRevision: 1,
		},
	}
	if _, _, err := handoffs.Detect(ownerUserID, handoff); err != nil {
		t.Fatal(err)
	}
	if err := handoffs.MarkSourceFinished(ownerUserID, conversationID, handoff.HandoffID); err != nil {
		t.Fatal(err)
	}
	queue := workspacestore.NewInputQueueStore(root)
	location := workspacestore.InputQueueLocation{
		OwnerUserID: ownerUserID, Scope: protocol.InputQueueScopeRoom,
		WorkspacePath: workspacePath,
		SessionKey:    protocol.BuildRoomAgentSessionKey(conversationID, targetAgentID, protocol.RoomTypeGroup),
		RoomID:        roomID, ConversationID: conversationID,
	}
	item := protocol.InputQueueItem{
		ID: "queue-goal-reconcile-stale", AgentID: targetAgentID,
		TargetAgentIDs: []string{targetAgentID}, Source: protocol.InputQueueSourceAgentRoomMessage,
		SourceAgentID: handoff.SourceAgentID, SourceMessageID: handoff.SourceMessageID,
		HandoffID: handoff.HandoffID, Content: handoff.Content,
		DeliveryPolicy:           protocol.ChatDeliveryPolicyQueue,
		GoalCollaborationBinding: handoff.GoalCollaborationBinding,
	}
	if _, err := queue.Enqueue(location, item); err != nil {
		t.Fatal(err)
	}
	if err := handoffs.MarkQueued(ownerUserID, conversationID, handoff.HandoffID, item.ID); err != nil {
		t.Fatal(err)
	}
	service := withConstructorDefaults(t, &Service{
		rooms:          &systemOnlyRoomContextStore{contextValue: contextValue},
		publicHandoffs: handoffs, Host: runtimehost.Host{InputQueue: queue},

		goals: &fakeRoomGoalContextProvider{runtimeGoals: map[string]*protocol.Goal{
			sessionKey: {
				ID: "goal-reconcile-stale", SessionKey: sessionKey, Status: protocol.GoalStatusActive,
				Metadata: map[string]any{protocol.GoalMetadataObjectiveRevision: int64(2)},
			},
		}},
	})
	if _, err := service.StartPublicHandoffReconciler(context.Background()); err != nil {
		t.Fatal(err)
	}
	items, err := queue.Snapshot(location)
	if err != nil || len(items) != 0 {
		t.Fatalf("stale startup queue = %+v err=%v, want deleted", items, err)
	}
	stored, exists, err := handoffs.Get(ownerUserID, conversationID, handoff.HandoffID)
	if err != nil || !exists || stored.Status != "interrupted" {
		t.Fatalf("stale startup handoff = %+v exists=%t err=%v", stored, exists, err)
	}
}

// 公共移交意图测试。

func TestBuildPublicMessageMentionAnnotationsCreatesEveryHandoffAndDedupesTargets(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Conversation: protocol.ConversationRecord{ID: "conversation-public-message"},
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-source"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-amy"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-devin"},
		},
		MemberAgents: []protocol.Agent{
			{AgentID: "agent-source", Name: "Source"},
			{AgentID: "agent-amy", Name: "Amy"},
			{AgentID: "agent-devin", Name: "Devin"},
		},
	}
	mentions := buildPublicMessageMentionAnnotations(
		contextValue,
		"agent-source",
		"message-public",
		"@Amy 处理接口，@Devin 检查测试，@Amy 汇总结论；Source 不需要接手。",
	)
	if len(mentions) != 3 {
		t.Fatalf("主动发布消息的每个有效 mention 都应被标注: %+v", mentions)
	}
	for _, mention := range mentions {
		if mention.HandoffID == "" {
			t.Fatalf("主动发布消息的每个有效 mention 都应创建 handoff: %+v", mentions)
		}
	}
	if targets := handoffTargetAgentIDs(mentions); !slices.Equal(
		targets,
		[]string{"agent-amy", "agent-devin"},
	) {
		t.Fatalf("重复目标只应唤醒一次且保留首次出现顺序: %+v", targets)
	}
}

func TestPublicHandoffAdmissionRejectsRootOverflow(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	t.Setenv("NEXUS_CONFIG_DIR", root)
	conversationID := "conversation-admission-guard"
	store := workspacestore.NewRoomPublicHandoffStore(root)
	detect := func(handoff workspacestore.RoomPublicHandoff) {
		t.Helper()
		if _, _, err := store.Detect("owner", handoff); err != nil {
			t.Fatal(err)
		}
		if err := store.MarkSourceFinished("owner", conversationID, handoff.HandoffID); err != nil {
			t.Fatal(err)
		}
	}
	service := &Service{publicHandoffs: store}
	for index := 0; index < roomMaxRootHandoffs; index++ {
		detect(workspacestore.RoomPublicHandoff{
			HandoffID:       "rh-overflow-" + string(rune('a'+index)),
			ConversationID:  conversationID,
			RootRoundID:     "root-overflow",
			SourceMessageID: "message-overflow-" + string(rune('a'+index)),
			SourceAgentID:   "agent-source-" + string(rune('a'+index)),
			TargetAgentID:   "agent-target-" + string(rune('a'+index)),
		})
	}
	overflowID := "rh-overflow-new"
	detect(workspacestore.RoomPublicHandoff{
		HandoffID: overflowID, ConversationID: conversationID, RootRoundID: "root-overflow",
		SourceMessageID: "message-overflow-new", SourceAgentID: "agent-source-new", TargetAgentID: "agent-target-new",
	})
	accepted, err := service.admitPublicMentionWakes(context.Background(), &activeRoomRound{
		ConversationID: conversationID, RootRoundID: "root-overflow", OwnerUserID: "owner",
	}, []publicMentionWake{{
		HandoffID: overflowID, QueueSource: protocol.InputQueueSourceAgentPublicMention,
		SourceAgentID: "agent-source-new", TargetAgentID: "agent-target-new",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(accepted) != 0 {
		t.Fatalf("root handoff 超限后不应继续接受新边: %+v", accepted)
	}
}

// 公共提及队列测试。

func TestQueueBusyPublicMentionWakesGuidesEachBusyRootAndLeavesIdleTargetReady(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-public-mention-mixed-roots"
	roomID := "room-public-mention-mixed-roots"
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	agents := []protocol.Agent{
		{AgentID: "agent-a", WorkspacePath: filepath.Join(root, "agent-a")},
		{AgentID: "agent-b", WorkspacePath: filepath.Join(root, "agent-b")},
		{AgentID: "agent-c", WorkspacePath: filepath.Join(root, "agent-c")},
	}
	members := make([]protocol.MemberRecord, 0, len(agents))
	for _, agentValue := range agents {
		members = append(members, protocol.MemberRecord{
			RoomID:        roomID,
			MemberType:    protocol.MemberTypeAgent,
			MemberAgentID: agentValue.AgentID,
		})
	}
	contextValue := &protocol.ConversationContextAggregate{
		Room:         protocol.RoomRecord{ID: roomID, RoomType: protocol.RoomTypeGroup},
		Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: roomID},
		Members:      members,
		MemberAgents: agents,
	}
	newSlot := func(agent protocol.Agent, agentRoundID string) *activeRoomSlot {
		slot := &activeRoomSlot{
			AgentID:           agent.AgentID,
			AgentRoundID:      agentRoundID,
			RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(conversationID, agent.AgentID, protocol.RoomTypeGroup),
			WorkspacePath:     agent.WorkspacePath,
		}
		slot.setStatus("running")
		return slot
	}
	slotA := newSlot(agents[0], "agent-round-a")
	slotB := newSlot(agents[1], "agent-round-b")
	store := workspacestore.NewInputQueueStore(root)
	runtimeManager := runtimectx.NewManagerWithFactory(roomGuidanceRuntimeFactory{
		client: &permissionModeTestClient{hookResponseAck: true},
	})
	for _, slot := range []*activeRoomSlot{slotA, slotB} {
		if _, err := runtimeManager.GetOrCreate(context.Background(), slot.RuntimeSessionKey, agentclient.Options{}); err != nil {
			t.Fatalf("创建 ACK runtime 失败: %v", err)
		}
	}
	service := &Service{
		Host: runtimehost.Host{InputQueue: store, Runtime: runtimeManager, Permission: permissionctx.NewContext()},
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"root-a": {
				SessionKey: sharedSessionKey, ConversationID: conversationID, RootRoundID: "root-a",
				Slots: map[string]*activeRoomSlot{slotA.AgentID: slotA},
			},
			"root-b": {
				SessionKey: sharedSessionKey, ConversationID: conversationID, RootRoundID: "root-b",
				Slots: map[string]*activeRoomSlot{slotB.AgentID: slotB},
			},
		}),
	}
	parentRound := &activeRoomRound{
		SessionKey: sharedSessionKey, RoomID: roomID, ConversationID: conversationID,
		RootRoundID: "parent-root", Context: contextValue, OwnerUserID: "owner",
	}
	wakes := []publicMentionWake{
		{SourceAgentID: "source", TargetAgentID: agents[0].AgentID, MessageID: "message-a", Content: "@A"},
		{SourceAgentID: "source", TargetAgentID: agents[1].AgentID, MessageID: "message-b", Content: "@B"},
		{SourceAgentID: "source", TargetAgentID: agents[2].AgentID, MessageID: "message-c", Content: "@C"},
	}

	ready, err := service.queueBusyPublicMentionWakes(context.Background(), parentRound, sharedSessionKey, wakes)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 || ready[0].TargetAgentID != agents[2].AgentID {
		t.Fatalf("只有空闲目标应立即启动: %+v", ready)
	}
	for _, slot := range []*activeRoomSlot{slotA, slotB} {
		location := workspacestore.InputQueueLocation{
			Scope: protocol.InputQueueScopeRoom, WorkspacePath: slot.WorkspacePath,
			SessionKey: slot.RuntimeSessionKey, RoomID: roomID, ConversationID: conversationID,
		}
		items, snapshotErr := store.Snapshot(location)
		if snapshotErr != nil || len(items) != 1 || items[0].AgentID != slot.AgentID {
			t.Fatalf("不同 root 的忙碌目标都必须进入自己的队列: agent=%s items=%+v err=%v", slot.AgentID, items, snapshotErr)
		}
		if items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide || items[0].RootRoundID != slot.AgentRoundID {
			t.Fatalf("busy 公区 @ 必须绑定目标当前 slot 的 guide: agent=%s item=%+v", slot.AgentID, items[0])
		}
	}

	locationA := workspacestore.InputQueueLocation{
		Scope: protocol.InputQueueScopeRoom, WorkspacePath: slotA.WorkspacePath,
		SessionKey: slotA.RuntimeSessionKey, RoomID: roomID, ConversationID: conversationID,
	}
	output, err := service.roomSlotGuidanceHook(service.rounds.findByRoundID(sharedSessionKey, "root-a"), slotA, locationA)(
		context.Background(),
		sdkhook.Input{EventName: sdkhook.EventPostToolUse},
		"tool-before-public-mention",
	)
	if err != nil {
		t.Fatalf("busy 目标消费公区 @ guide 失败: %v", err)
	}
	if output.SpecificOutput == nil || !strings.Contains(output.SpecificOutput.AdditionalContext, "@A") {
		t.Fatalf("当前 slot 未收到公区 @ additionalContext: %+v", output)
	}
	if output.OnApplied == nil {
		t.Fatal("公区 @ guide 缺少 runtime applied ACK 回调")
	}
	items, err := store.Snapshot(locationA)
	if err != nil || len(items) != 1 {
		t.Fatalf("applied ACK 前必须保留 durable 公区 @: items=%+v err=%v", items, err)
	}
	output.OnApplied(sdkhook.AppliedAck{RequestID: "public-mention-applied"})
	items, err = store.Snapshot(locationA)
	if err != nil || len(items) != 0 {
		t.Fatalf("applied ACK 后应只消费已注入的公区 @: items=%+v err=%v", items, err)
	}
}

func TestLatestActiveRootRoundAgentIDsPrefersRegistrationSequence(t *testing.T) {
	service := &Service{rounds: newRoomRoundRegistry()}
	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-latest-root")
	register := func(roundID string, agentID string) {
		slot := &activeRoomSlot{
			AgentID:      agentID,
			AgentRoundID: roundID + "-agent",
			TimestampMS:  100,
		}
		slot.setStatus("running")
		service.registerRound(&activeRoomRound{
			SessionKey:     sessionKey,
			ConversationID: "conversation-latest-root",
			RoundID:        roundID,
			RootRoundID:    roundID,
			Slots:          map[string]*activeRoomSlot{agentID: slot},
		})
	}

	// root id 的字典序故意与注册顺序相反，确保并列时间戳不参与主判定。
	register("a-earlier-root", "agent-earlier")
	register("z-later-root", "agent-later")

	got := service.latestActiveRootRoundAgentIDs(sessionKey, "conversation-latest-root")
	if want := []string{"agent-later"}; !slices.Equal(got, want) {
		t.Fatalf("最近活跃 root 目标 = %+v, want %+v", got, want)
	}
}

func TestResolveChatTargetAgentIDsRejectsNonMemberTarget(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-amy"},
		},
	}
	_, _, err := resolveChatTargetAgentIDs(
		ChatRequest{Content: "整理一下", TargetAgentIDs: []string{"agent-outsider"}},
		contextValue,
		map[string]string{"agent-amy": "Amy"},
	)
	if err == nil || !strings.Contains(err.Error(), "not a room member") {
		t.Fatalf("非成员目标应被拒绝: %v", err)
	}
}

func TestBrowserRoomDefaultTargetsRespectHostSettings(t *testing.T) {
	for _, test := range []struct {
		name     string
		host     string
		enabled  bool
		members  map[string]string
		explicit []string
		want     []string
	}{
		{name: "enabled host", host: "lead", enabled: true, want: []string{"lead"}},
		{name: "disabled host", host: "lead"},
		{name: "missing host", enabled: true},
		{name: "removed host", host: "removed", enabled: true},
		{name: "explicit member wins", host: "lead", enabled: true, explicit: []string{"peer"}, want: []string{"peer"}},
		{name: "single group member requires opt in", host: "lead", members: map[string]string{"lead": "Lead"}},
		{name: "single group host opted in", host: "lead", enabled: true, members: map[string]string{"lead": "Lead"}, want: []string{"lead"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			members := test.members
			if members == nil {
				members = map[string]string{"lead": "Lead", "peer": "Peer"}
			}
			room := &protocol.ConversationContextAggregate{Room: protocol.RoomRecord{
				RoomType: protocol.RoomTypeGroup, HostAgentID: test.host, HostAutoReplyEnabled: test.enabled,
			}}
			targets, _ := resolveDefaultRoomTargets(room, members, test.explicit, "explicit_target", true)
			if !slices.Equal(targets, test.want) {
				t.Fatalf("targets = %v, want %v", targets, test.want)
			}
		})
	}
}

func TestRoomRuntimeDiagnosticsLogsStderrWhenEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		var output bytes.Buffer
		logger := logx.New(logx.Options{Output: &output, Format: "json"})
		value := "0"
		if enabled {
			value = "1"
		}
		options := runtimehost.WithRuntimeDiagnosticsLogger(agentclient.Options{
			Env: map[string]string{runtimectx.AgentSDKDiagnosticsEnvName: value},
		}, logger)
		options.Callbacks.Stderr("startup error")
		if got := strings.Contains(output.String(), "startup error"); got != enabled {
			t.Fatalf("enabled=%t, stderr logged=%t", enabled, got)
		}
	}
}

func TestDirectedMessageAgentIDsCoversSourceAndRecipientsOnce(t *testing.T) {
	got := directedMessageAgentIDs(protocol.RoomDirectedMessageRecord{
		SourceAgentID: " agent-b ",
		Recipients:    []string{"agent-a", "agent-b", "", "agent-c"},
	})
	want := []string{"agent-a", "agent-b", "agent-c"}
	if !slices.Equal(got, want) {
		t.Fatalf("私域失效应覆盖发送方与接收方且去重: got=%v want=%v", got, want)
	}
}

type authorityFenceRoomStore struct {
	mu                  sync.RWMutex
	contextValue        *protocol.ConversationContextAggregate
	respectCancellation bool
}

func (s *authorityFenceRoomStore) GetConversationContext(
	ctx context.Context,
	_ string,
) (*protocol.ConversationContextAggregate, error) {
	if s.respectCancellation {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return s.snapshot(), nil
}

func (s *authorityFenceRoomStore) GetConversationContextForSystem(
	ctx context.Context,
	_ string,
) (*protocol.ConversationContextAggregate, error) {
	if s.respectCancellation {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return s.snapshot(), nil
}

func (*authorityFenceRoomStore) UpdateSessionRuntimeIdentity(context.Context, string, string, string) error {
	return nil
}

func (*authorityFenceRoomStore) TouchConversationActivity(context.Context, string, time.Time) error {
	return nil
}

func (*authorityFenceRoomStore) MarkConversationStarted(context.Context, string, time.Time) error {
	return nil
}

func (*authorityFenceRoomStore) BuildRoomSkillPrompt(context.Context, []string) (string, error) {
	return "", nil
}

func (s *authorityFenceRoomStore) snapshot() *protocol.ConversationContextAggregate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneAuthorityFenceContext(s.contextValue)
}

func (s *authorityFenceRoomStore) update(
	update func(*protocol.ConversationContextAggregate),
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	update(s.contextValue)
}

func cloneAuthorityFenceContext(
	contextValue *protocol.ConversationContextAggregate,
) *protocol.ConversationContextAggregate {
	if contextValue == nil {
		return nil
	}
	cloned := *contextValue
	cloned.Room.SkillNames = append([]string(nil), contextValue.Room.SkillNames...)
	cloned.Members = append([]protocol.MemberRecord(nil), contextValue.Members...)
	cloned.MemberAgents = append([]protocol.Agent(nil), contextValue.MemberAgents...)
	cloned.Sessions = append([]protocol.SessionRecord(nil), contextValue.Sessions...)
	return &cloned
}

func newAuthorityFenceContext() *protocol.ConversationContextAggregate {
	return &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{
			ID:                     "room-a",
			RoomType:               protocol.RoomTypeGroup,
			PrivateMessagesEnabled: true,
			AuthorityEpoch:         1,
		},
		Conversation: protocol.ConversationRecord{ID: "conversation-a", RoomID: "room-a"},
		Members: []protocol.MemberRecord{
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-a"},
			{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-b"},
		},
		MemberAgents: []protocol.Agent{{AgentID: "agent-a"}, {AgentID: "agent-b"}},
	}
}

func newAuthorityFenceRound(
	contextValue *protocol.ConversationContextAggregate,
) *activeRoomRound {
	return &activeRoomRound{
		SessionKey:     protocol.BuildRoomSharedSessionKey("conversation-a"),
		RoomID:         "room-a",
		ConversationID: "conversation-a",
		Context:        cloneAuthorityFenceContext(contextValue),
		AuthorityEpoch: contextValue.Room.AuthorityEpoch,
		RootRoundID:    "round-a",
		RoundID:        "round-a",
	}
}

type authorityFenceBroadcaster struct {
	mu     sync.Mutex
	events []protocol.EventMessage
}

func (b *authorityFenceBroadcaster) Broadcast(
	context.Context,
	string,
	protocol.EventMessage,
) []error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, protocol.EventMessage{})
	return nil
}

func (b *authorityFenceBroadcaster) eventCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

func TestRoomIdleSubagentDropsDurableAndEventsAfterRevocation(t *testing.T) {
	initialContext := newAuthorityFenceContext()
	store := &authorityFenceRoomStore{contextValue: initialContext}
	root := t.TempDir()
	broadcaster := &authorityFenceBroadcaster{}
	service := &Service{
		rooms:       store,
		Host:        runtimehost.Host{History: workspacestore.NewAgentHistoryStore(root)},
		roomHistory: workspacestore.NewRoomHistoryStore(root),
		broadcaster: broadcaster,
	}
	roundValue := newAuthorityFenceRound(initialContext)
	slot := &activeRoomSlot{
		AgentID:           "agent-a",
		AgentRoundID:      "agent-round-a",
		MsgID:             "slot-message-a",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey("conversation-a", "agent-a", protocol.RoomTypeGroup),
		WorkspacePath:     root + "/agent-a",
	}
	incoming := authorityFenceAssistantMessage()
	probeMapper := newAuthorityFenceMapper(roundValue, slot)
	events, durableMessages, _, err := probeMapper.Map(incoming)
	if err != nil || len(events) == 0 || len(durableMessages) == 0 {
		t.Fatalf("idle fixture must contain event and durable output: events=%d durable=%d err=%v", len(events), len(durableMessages), err)
	}
	store.update(func(contextValue *protocol.ConversationContextAggregate) {
		contextValue.Room.AuthorityEpoch++
	})

	if keepDraining := service.handleIdleSubagentMessage(
		t.Context(),
		roundValue,
		slot,
		newAuthorityFenceMapper(roundValue, slot),
		authorityFenceAssistantMessage(),
	); keepDraining {
		t.Fatal("revoked idle drain should stop")
	}
	assertAuthorityFenceHistoriesEmpty(t, service, roundValue, slot)
	if broadcaster.eventCount() != 0 {
		t.Fatalf("revoked idle output emitted %d events", broadcaster.eventCount())
	}
}

func newAuthorityFenceMapper(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) *roomdomain.SlotMessageMapper {
	return roomdomain.NewSlotMessageMapper(
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		roundValue.RootRoundID,
		slot.AgentRoundID,
		slot.WorkspacePath,
	)
}

func authorityFenceAssistantMessage() sdkprotocol.ReceivedMessage {
	return sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "idle-assistant-a",
				StopReason: "end_turn",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: "revoked idle output"},
				},
			},
		},
	}
}

func assertAuthorityFenceHistoriesEmpty(
	t *testing.T,
	service *Service,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) {
	t.Helper()
	sharedMessages, err := service.roomHistory.ReadMessages(
		authctx.SystemUserID,
		roundValue.ConversationID,
		nil,
	)
	if err != nil {
		t.Fatalf("read shared history: %v", err)
	}
	if len(sharedMessages) != 0 {
		t.Fatalf("revoked output reached shared history: %+v", sharedMessages)
	}
	privateMessages, err := service.History.ReadMessages(slot.WorkspacePath, protocol.Session{
		SessionKey: slot.RuntimeSessionKey,
		AgentID:    slot.AgentID,
	}, nil)
	if err != nil {
		t.Fatalf("read private history: %v", err)
	}
	if len(privateMessages) != 0 {
		t.Fatalf("revoked output reached private history: %+v", privateMessages)
	}
}
