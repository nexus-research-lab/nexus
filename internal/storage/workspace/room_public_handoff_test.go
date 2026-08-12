package workspace

import (
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomPublicHandoffStoreIsDurableAndIdempotent(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-handoff"
	handoff := RoomPublicHandoff{
		HandoffID:          "rh_test-1",
		ConversationID:     conversationID,
		RoomID:             "room-1",
		RootRoundID:        "round-root",
		SourceAgentRoundID: "agent-round-a",
		SourceMessageID:    "message-a",
		SourceAgentID:      "agent-a",
		TargetAgentID:      "agent-b",
		Content:            "请 @AgentB 继续",
		QueueSource:        protocol.InputQueueSourceAgentPublicMention,
		GoalCollaborationBinding: &protocol.GoalCollaborationBinding{
			GoalID:            "goal-room",
			ObjectiveRevision: 3,
		},
	}

	store := NewRoomPublicHandoffStore(root)
	store.paths.StateRoot = root
	first, inserted, err := store.Detect(testRoomOwnerUserID, handoff)
	if err != nil || !inserted || first.Status != roomPublicHandoffActionDetected {
		t.Fatalf("首次检测应落盘: value=%+v inserted=%v err=%v", first, inserted, err)
	}
	second, inserted, err := store.Detect(testRoomOwnerUserID, handoff)
	if err != nil || inserted || second.HandoffID != handoff.HandoffID {
		t.Fatalf("重复检测必须幂等: value=%+v inserted=%v err=%v", second, inserted, err)
	}
	if err := store.MarkSourceFinished(testRoomOwnerUserID, conversationID, handoff.HandoffID); err != nil {
		t.Fatal(err)
	}
	claimed, ok, err := store.Claim(testRoomOwnerUserID, conversationID, handoff.HandoffID)
	if err != nil || !ok || claimed.Status != roomPublicHandoffActionClaimed {
		t.Fatalf("source 完成后应只允许一次 claim: value=%+v claimed=%v err=%v", claimed, ok, err)
	}
	if _, ok, err := store.Claim(testRoomOwnerUserID, conversationID, handoff.HandoffID); err != nil || ok {
		t.Fatalf("重复 claim 不应再次成功: ok=%v err=%v", ok, err)
	}
	if err := store.MarkStarted(testRoomOwnerUserID, conversationID, handoff.HandoffID, "target-round"); err != nil {
		t.Fatal(err)
	}

	reloaded := NewRoomPublicHandoffStore(root)
	reloaded.paths.StateRoot = root
	pending, err := reloaded.Pending(testRoomOwnerUserID, conversationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].HandoffID != handoff.HandoffID {
		t.Fatalf("Goal-attributed started handoff 应保持可恢复: %+v", pending)
	}
	stored, ok, err := reloaded.Get(testRoomOwnerUserID, conversationID, handoff.HandoffID)
	if err != nil || !ok || stored.GoalCollaborationBinding == nil ||
		stored.GoalCollaborationBinding.GoalID != "goal-room" ||
		stored.GoalCollaborationBinding.ObjectiveRevision != 3 {
		t.Fatalf("Goal collaboration binding was not restored: value=%+v ok=%v err=%v", stored, ok, err)
	}
	path := store.paths.RoomPublicHandoffsPath(testRoomOwnerUserID, conversationID)
	if _, err := reloaded.files.readJSONL(path); err != nil {
		t.Fatalf("handoff ledger 应写入 workspace JSONL: %v", err)
	}
}

func TestRoomPublicHandoffRejectsInvalidGoalCollaborationBinding(t *testing.T) {
	store := NewRoomPublicHandoffStore(t.TempDir())
	_, _, err := store.Detect(testRoomOwnerUserID, RoomPublicHandoff{
		HandoffID:       "rh_invalid_goal_binding",
		ConversationID:  "conversation-handoff",
		SourceMessageID: "message-invalid",
		SourceAgentID:   "agent-lead",
		TargetAgentID:   "agent-worker",
		GoalCollaborationBinding: &protocol.GoalCollaborationBinding{
			GoalID: "goal-room",
		},
	})
	if err == nil {
		t.Fatal("incomplete Goal collaboration binding must be rejected")
	}
}

func TestRoomPublicHandoffRejectsGoalBindingWithoutQueueSource(t *testing.T) {
	store := NewRoomPublicHandoffStore(t.TempDir())
	_, _, err := store.Detect(testRoomOwnerUserID, RoomPublicHandoff{
		HandoffID: "handoff-goal-source", ConversationID: "conversation-goal-source",
		SourceMessageID: "message-goal-source", SourceAgentID: "agent-source",
		TargetAgentID: "agent-target", Content: "check",
		GoalCollaborationBinding: &protocol.GoalCollaborationBinding{
			GoalID: "goal-room", ObjectiveRevision: 1,
		},
	})
	if err == nil {
		t.Fatal("Goal collaboration handoff without a queue source must fail closed")
	}
}

func TestRoomPublicHandoffGoalCollaborationInFlightUsesExactRevision(t *testing.T) {
	root := t.TempDir()
	store := NewRoomPublicHandoffStore(root)
	store.paths.StateRoot = root
	conversationID := "conversation-goal-fence"
	ownerUserID := testRoomOwnerUserID
	handoff := RoomPublicHandoff{
		HandoffID:       "rh_goal_fence",
		ConversationID:  conversationID,
		SourceMessageID: "message-goal-fence",
		SourceAgentID:   "agent-lead",
		TargetAgentID:   "agent-peer",
		QueueSource:     protocol.InputQueueSourceAgentPublicMention,
		GoalCollaborationBinding: &protocol.GoalCollaborationBinding{
			GoalID:            "goal-room",
			ObjectiveRevision: 2,
		},
	}
	if _, _, err := store.Detect(ownerUserID, handoff); err != nil {
		t.Fatal(err)
	}
	inFlight, err := store.GoalCollaborationInFlight(
		ownerUserID,
		conversationID,
		*handoff.GoalCollaborationBinding,
	)
	if err != nil || !inFlight {
		t.Fatalf("inFlight = %v err=%v, want exact revision fenced", inFlight, err)
	}
	inFlight, err = store.GoalCollaborationInFlight(
		ownerUserID,
		conversationID,
		protocol.GoalCollaborationBinding{GoalID: "goal-room", ObjectiveRevision: 3},
	)
	if err != nil || inFlight {
		t.Fatalf("inFlight = %v err=%v, want newer revision independent", inFlight, err)
	}
	if err := store.MarkTerminal(ownerUserID, conversationID, handoff.HandoffID, "finished"); err != nil {
		t.Fatal(err)
	}
	inFlight, err = store.GoalCollaborationInFlight(
		ownerUserID,
		conversationID,
		*handoff.GoalCollaborationBinding,
	)
	if err != nil || inFlight {
		t.Fatalf("inFlight = %v err=%v, want terminal edge released", inFlight, err)
	}
}

func TestRoomPublicHandoffTerminalGoalHandbackSurvivesRestartExactlyOnce(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	t.Setenv("NEXUS_CONFIG_DIR", root)
	const conversationID = "conversation-goal-handback-recovery"
	handoff := RoomPublicHandoff{
		HandoffID:       "rh_goal_handback_recovery",
		ConversationID:  conversationID,
		SourceMessageID: "message-goal-handback-recovery",
		SourceAgentID:   "agent-lead",
		TargetAgentID:   "agent-peer",
		QueueSource:     protocol.InputQueueSourceAgentPublicMention,
		GoalCollaborationBinding: &protocol.GoalCollaborationBinding{
			GoalID: "goal-room", ObjectiveRevision: 2,
		},
	}
	store := NewRoomPublicHandoffStore(root)
	if _, _, err := store.Detect(testRoomOwnerUserID, handoff); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkSourceFinished(testRoomOwnerUserID, conversationID, handoff.HandoffID); err != nil {
		t.Fatal(err)
	}
	if _, claimed, err := store.Claim(testRoomOwnerUserID, conversationID, handoff.HandoffID); err != nil || !claimed {
		t.Fatalf("claim=%t err=%v", claimed, err)
	}
	if err := store.MarkStarted(testRoomOwnerUserID, conversationID, handoff.HandoffID, "target-root-round"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkTerminalWithGoalOutcome(
		testRoomOwnerUserID,
		conversationID,
		handoff.HandoffID,
		"finished",
		"target-agent-round",
		true,
		true,
	); err != nil {
		t.Fatal(err)
	}

	pending, err := NewRoomPublicHandoffStore(root).PendingAll()
	if err != nil || len(pending) != 1 || pending[0].HandoffID != handoff.HandoffID ||
		pending[0].TargetRoundID != "target-root-round" ||
		pending[0].TargetAgentRoundID != "target-agent-round" ||
		!pending[0].GoalSubstantiveOutput || !pending[0].GoalPublicEvidence ||
		!pending[0].GoalHandbackRequired ||
		pending[0].GoalHandbackSettled {
		t.Fatalf("pending terminal handback=%+v err=%v", pending, err)
	}
	if err := store.MarkGoalHandbackSettled(testRoomOwnerUserID, conversationID, handoff.HandoffID); err != nil {
		t.Fatal(err)
	}
	pending, err = NewRoomPublicHandoffStore(root).PendingAll()
	if err != nil || len(pending) != 0 {
		t.Fatalf("settled handback must not replay: pending=%+v err=%v", pending, err)
	}
}

func TestRoomPublicHandoffGenericCancellationDoesNotScheduleGoalHandback(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	t.Setenv("NEXUS_CONFIG_DIR", root)
	const conversationID = "conversation-goal-cancelled"
	handoff := RoomPublicHandoff{
		HandoffID:       "rh_goal_cancelled",
		ConversationID:  conversationID,
		RootRoundID:     "root-goal-cancelled",
		SourceMessageID: "message-goal-cancelled",
		SourceAgentID:   "agent-lead",
		TargetAgentID:   "agent-peer",
		QueueSource:     protocol.InputQueueSourceAgentPublicMention,
		GoalCollaborationBinding: &protocol.GoalCollaborationBinding{
			GoalID: "goal-room", ObjectiveRevision: 2,
		},
	}
	store := NewRoomPublicHandoffStore(root)
	if _, _, err := store.Detect(testRoomOwnerUserID, handoff); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkSourceFinished(testRoomOwnerUserID, conversationID, handoff.HandoffID); err != nil {
		t.Fatal(err)
	}
	if err := store.CancelForRoot(testRoomOwnerUserID, conversationID, handoff.RootRoundID, "interrupted"); err != nil {
		t.Fatal(err)
	}
	pending, err := NewRoomPublicHandoffStore(root).PendingAll()
	if err != nil || len(pending) != 0 {
		t.Fatalf("generic cancellation must not schedule Goal handback: pending=%+v err=%v", pending, err)
	}
}

func TestRoomPublicHandoffPendingAllRecoversFreshClaimAfterRestart(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	t.Setenv("NEXUS_CONFIG_DIR", root)
	handoff := RoomPublicHandoff{
		HandoffID:       "rh-fresh-claim-restart",
		ConversationID:  "conversation-fresh-claim-restart",
		SourceMessageID: "message-fresh-claim-restart",
		SourceAgentID:   "agent-a",
		TargetAgentID:   "agent-b",
	}
	store := NewRoomPublicHandoffStore(root)
	if _, _, err := store.Detect(testRoomOwnerUserID, handoff); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkSourceFinished(testRoomOwnerUserID, handoff.ConversationID, handoff.HandoffID); err != nil {
		t.Fatal(err)
	}
	if _, claimed, err := store.Claim(testRoomOwnerUserID, handoff.ConversationID, handoff.HandoffID); err != nil || !claimed {
		t.Fatalf("claim = %t err=%v", claimed, err)
	}
	pending, err := NewRoomPublicHandoffStore(root).PendingAll()
	if err != nil || len(pending) != 1 || pending[0].Status != roomPublicHandoffActionClaimed {
		t.Fatalf("fresh prior-process claim must recover immediately: pending=%+v err=%v", pending, err)
	}
}

func TestRoomPublicHandoffRejectsIncompleteResponsibilityBinding(t *testing.T) {
	store := NewRoomPublicHandoffStore(t.TempDir())
	_, _, err := store.Detect(testRoomOwnerUserID, RoomPublicHandoff{
		HandoffID:       "execution_dispatch_incomplete",
		ConversationID:  "conversation-handoff",
		SourceMessageID: "execution_dispatch_incomplete",
		SourceAgentID:   "agent-lead",
		TargetAgentID:   "agent-worker",
		Content:         "deliver the result",
		WorkBinding: &protocol.ExecutionWorkBinding{
			ExecutionID:  "execution-1",
			PlanID:       "plan-1",
			WorkItemID:   "work-1",
			SpecID:       "spec-1",
			AssignmentID: "assignment-1",
			DispatchID:   "dispatch-1",
		},
	})
	if !errors.Is(err, protocol.ErrInvalidInputQueueCapabilityEnvelope) {
		t.Fatalf("error = %v, want capability envelope error", err)
	}
}

func TestRoomPublicHandoffStoreReleasesClaimAfterRestartWindow(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-retry"
	handoff := RoomPublicHandoff{
		HandoffID:       "rh_test-retry",
		ConversationID:  conversationID,
		SourceMessageID: "message-retry",
		SourceAgentID:   "agent-a",
		TargetAgentID:   "agent-b",
	}
	store := NewRoomPublicHandoffStore(root)
	store.paths.StateRoot = root
	if _, _, err := store.Detect(testRoomOwnerUserID, handoff); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkSourceFinished(testRoomOwnerUserID, conversationID, handoff.HandoffID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.Claim(testRoomOwnerUserID, conversationID, handoff.HandoffID); err != nil || !ok {
		t.Fatalf("首次 claim 失败: ok=%v err=%v", ok, err)
	}
	// 手工追加一个过期 claim，验证重启扫描会重新暴露它。
	value, ok, err := store.Claim(testRoomOwnerUserID, conversationID, handoff.HandoffID)
	if err != nil || ok {
		t.Fatalf("未过期 claim 不应重复领取: value=%+v ok=%v err=%v", value, ok, err)
	}
	value.ClaimedAt = 1
	value.UpdatedAt = 1
	if err := store.appendLocked(testRoomOwnerUserID, conversationID, roomPublicHandoffActionClaimed, value); err != nil {
		t.Fatal(err)
	}
	reloaded := NewRoomPublicHandoffStore(root)
	reloaded.paths.StateRoot = root
	pending, err := reloaded.Pending(testRoomOwnerUserID, conversationID)
	if err != nil || len(pending) != 1 || pending[0].HandoffID != handoff.HandoffID {
		t.Fatalf("过期 claim 应进入恢复列表: pending=%+v err=%v", pending, err)
	}
}

func TestRoomPublicHandoffStoreClaimsQueuedDelivery(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-queued"
	handoff := RoomPublicHandoff{
		HandoffID:       "rh_test-queued",
		ConversationID:  conversationID,
		SourceMessageID: "message-queued",
		SourceAgentID:   "agent-a",
		TargetAgentID:   "agent-b",
	}
	store := NewRoomPublicHandoffStore(root)
	store.paths.StateRoot = root
	if _, _, err := store.Detect(testRoomOwnerUserID, handoff); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkSourceFinished(testRoomOwnerUserID, conversationID, handoff.HandoffID); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkQueued(testRoomOwnerUserID, conversationID, handoff.HandoffID, "queue-1"); err != nil {
		t.Fatal(err)
	}
	claimed, ok, err := store.Claim(testRoomOwnerUserID, conversationID, handoff.HandoffID)
	if err != nil || !ok || claimed.Status != roomPublicHandoffActionClaimed {
		t.Fatalf("queued handoff 应可被 dispatcher claim: value=%+v ok=%v err=%v", claimed, ok, err)
	}
	if err := store.MarkTerminal(testRoomOwnerUserID, conversationID, handoff.HandoffID, "finished"); err != nil {
		t.Fatal(err)
	}
	pending, err := store.Pending(testRoomOwnerUserID, conversationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("terminal handoff 不应继续 pending: %+v", pending)
	}
}

func TestRoomPublicHandoffStoreListsAndCancelsRoot(t *testing.T) {
	root := t.TempDir()
	conversationID := "conversation-root-cancel"
	store := NewRoomPublicHandoffStore(root)
	store.paths.StateRoot = root
	for _, handoff := range []RoomPublicHandoff{
		{
			HandoffID: "rh-root-a", ConversationID: conversationID, RootRoundID: "root-1",
			SourceMessageID: "message-a", SourceAgentID: "agent-a", TargetAgentID: "agent-b",
		},
		{
			HandoffID: "rh-root-b", ConversationID: conversationID, RootRoundID: "root-1",
			SourceMessageID: "message-b", SourceAgentID: "agent-b", TargetAgentID: "agent-c",
		},
		{
			HandoffID: "rh-other", ConversationID: conversationID, RootRoundID: "root-2",
			SourceMessageID: "message-c", SourceAgentID: "agent-a", TargetAgentID: "agent-d",
		},
	} {
		if _, _, err := store.Detect(testRoomOwnerUserID, handoff); err != nil {
			t.Fatal(err)
		}
		if err := store.MarkSourceFinished(testRoomOwnerUserID, conversationID, handoff.HandoffID); err != nil {
			t.Fatal(err)
		}
	}
	edges, err := store.ListRoot(testRoomOwnerUserID, conversationID, "root-1")
	if err != nil || len(edges) != 2 {
		t.Fatalf("root snapshot 不正确: edges=%+v err=%v", edges, err)
	}
	if err := store.CancelForRoot(testRoomOwnerUserID, conversationID, "root-1", "interrupted"); err != nil {
		t.Fatal(err)
	}
	pending, err := store.Pending(testRoomOwnerUserID, conversationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].HandoffID != "rh-other" {
		t.Fatalf("取消 root 后只应保留其他 root: %+v", pending)
	}
}

func TestRoomPublicHandoffStorePendingAllPreservesOriginalOwner(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	t.Setenv("NEXUS_CONFIG_DIR", root)
	const ownerUserID = "owner/with/slash"
	handoff := RoomPublicHandoff{
		HandoffID:       "rh-unsafe-owner",
		ConversationID:  "conversation-unsafe-owner",
		SourceMessageID: "message-unsafe-owner",
		SourceAgentID:   "agent-a",
		TargetAgentID:   "agent-b",
	}
	store := NewRoomPublicHandoffStore(root)
	if _, _, err := store.Detect(ownerUserID, handoff); err != nil {
		t.Fatalf("写入特殊 owner handoff 失败: %v", err)
	}
	if err := store.MarkSourceFinished(ownerUserID, handoff.ConversationID, handoff.HandoffID); err != nil {
		t.Fatalf("推进特殊 owner handoff 失败: %v", err)
	}

	pending, err := NewRoomPublicHandoffStore(root).PendingAll()
	if err != nil {
		t.Fatalf("扫描全部 handoff 失败: %v", err)
	}
	if len(pending) != 1 || pending[0].OwnerUserID != ownerUserID {
		t.Fatalf("恢复后 owner 身份失真: pending=%+v", pending)
	}
}

func TestRoomPublicHandoffStorePendingAllRejectsMismatchedOwner(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	t.Setenv("NEXUS_CONFIG_DIR", root)
	store := NewRoomPublicHandoffStore(root)
	const (
		pathOwnerUserID = "owner-a"
		conversationID  = "conversation-forged-owner"
	)
	forgedHandoff := RoomPublicHandoff{
		HandoffID:       "rh-forged-owner",
		OwnerUserID:     "owner-b",
		ConversationID:  conversationID,
		SourceMessageID: "message-forged-owner",
		SourceAgentID:   "agent-a",
		TargetAgentID:   "agent-b",
		Status:          roomPublicHandoffActionSourceFinished,
		CreatedAt:       1,
		UpdatedAt:       1,
	}
	if err := store.files.appendRoomJSONL(
		pathOwnerUserID,
		store.paths.RoomPublicHandoffsPath(pathOwnerUserID, conversationID),
		map[string]any{
			"action":  roomPublicHandoffActionSourceFinished,
			"handoff": forgedHandoff,
		},
	); err != nil {
		t.Fatalf("写入伪造 owner handoff 失败: %v", err)
	}

	pending, err := NewRoomPublicHandoffStore(root).PendingAll()
	if err != nil {
		t.Fatalf("扫描全部 handoff 失败: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("目录与记录 owner 不一致时必须拒绝恢复: pending=%+v", pending)
	}
}
