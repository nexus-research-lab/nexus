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
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
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

// 公共移交意图测试。

// 公共提及队列测试。

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
