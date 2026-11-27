// input_queue_test.go 覆盖工作区持久化输入队列、恢复与 WorkBinding 测试。
package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestInputQueueStoreRejectsAnotherOwnerWorkspace(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	location := InputQueueLocation{
		OwnerUserID:   "user-a",
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, "user-b"), "agent-b"),
		SessionKey:    "agent:agent-b:ws:dm:cross-owner",
	}
	if _, err := NewInputQueueStore("").Enqueue(location, protocol.InputQueueItem{
		ID:      "item-cross-owner",
		Content: "不应写入另一用户 workspace",
	}); err == nil {
		t.Fatal("跨 owner workspace 的队列位置必须被拒绝")
	}
}

func TestInputQueueStoreEnqueueBatchRollsBackEarlierFiles(t *testing.T) {
	root := t.TempDir()
	store := NewInputQueueStore(root)
	validLocation := InputQueueLocation{
		Scope:         protocol.InputQueueScopeRoom,
		WorkspacePath: filepath.Join(root, "agent-a"),
		SessionKey:    "agent:agent-a:ws:group:conversation-batch",
	}
	blockedWorkspace := filepath.Join(root, "blocked")
	if err := os.WriteFile(blockedWorkspace, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := store.EnqueueBatch([]InputQueueEnqueue{
		{Location: validLocation, Item: protocol.InputQueueItem{ID: "guide-a", Content: "first"}},
		{
			Location: InputQueueLocation{
				Scope:         protocol.InputQueueScopeRoom,
				WorkspacePath: blockedWorkspace,
				SessionKey:    "agent:agent-b:ws:group:conversation-batch",
			},
			Item: protocol.InputQueueItem{ID: "guide-b", Content: "second"},
		},
	})
	if err == nil {
		t.Fatal("第二个队列文件不可写时批量登记应失败")
	}
	items, snapshotErr := NewInputQueueStore(root).Snapshot(validLocation)
	if snapshotErr != nil || len(items) != 0 {
		t.Fatalf("批量写入失败后第一目标必须回滚: items=%+v err=%v", items, snapshotErr)
	}
}

func TestInputQueueStoreBoundedEnqueueDeduplicatesAndExpires(t *testing.T) {
	root := t.TempDir()
	location := InputQueueLocation{
		Scope:         protocol.InputQueueScopeRoom,
		WorkspacePath: filepath.Join(root, "agent"),
		SessionKey:    "room:conversation-1:agent-1",
	}
	store := NewInputQueueStore(root)
	item := protocol.InputQueueItem{
		AgentID:         "agent-1",
		Source:          protocol.InputQueueSourceAgentRoomMessage,
		SourceMessageID: "message-1",
		Content:         "runtime trigger",
		DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
		ExpiresAt:       time.Now().Add(time.Hour).UnixMilli(),
	}
	items, inserted, err := store.EnqueueBounded(location, item, 1)
	if err != nil || !inserted || len(items) != 1 {
		t.Fatalf("有界入队失败: items=%+v inserted=%v err=%v", items, inserted, err)
	}
	items, inserted, err = store.EnqueueBounded(location, item, 1)
	if err != nil || inserted || len(items) != 1 {
		t.Fatalf("重复自动唤醒应被折叠: items=%+v inserted=%v err=%v", items, inserted, err)
	}
	item.SourceMessageID = "message-2"
	if _, _, err = store.EnqueueBounded(location, item, 1); !errors.Is(err, ErrInputQueueCapacity) {
		t.Fatalf("超出容量应返回 ErrInputQueueCapacity: %v", err)
	}
	if _, err = store.DispatchMany(location, []string{items[0].ID}); err != nil {
		t.Fatalf("消费有界队列失败: %v", err)
	}
	item.SourceMessageID = "message-expired"
	item.ExpiresAt = time.Now().Add(-time.Second).UnixMilli()
	if _, _, err = store.EnqueueBounded(location, item, 1); err != nil {
		t.Fatalf("写入过期队列项失败: %v", err)
	}
	items, err = store.Snapshot(location)
	if err != nil || len(items) != 0 {
		t.Fatalf("过期队列项不应出现在快照中: items=%+v err=%v", items, err)
	}
}

func TestInputQueueStoreGuidanceDispatchDoesNotReadAfterCommit(t *testing.T) {
	root := t.TempDir()
	location := InputQueueLocation{
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: filepath.Join(root, "agent"),
		SessionKey:    "agent:alpha:ws:dm:no-post-commit-read",
	}
	store := NewInputQueueStore(root)
	for _, item := range []protocol.InputQueueItem{
		{
			ID:             "guide-a",
			Content:        "需要确认的引导",
			DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
			RootRoundID:    "round-running",
		},
		{
			ID:             "queue-b",
			Content:        "仍应留在队列",
			DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		},
	} {
		if _, err := store.Enqueue(location, item); err != nil {
			t.Fatal(err)
		}
	}
	current, err := store.Snapshot(location)
	if err != nil {
		t.Fatal(err)
	}
	guidanceItems := matchingGuidanceItems(current, []string{"round-running"})
	if len(guidanceItems) != 1 || guidanceItems[0].ID != "guide-a" {
		t.Fatalf("unexpected prepared guidance: %+v", guidanceItems)
	}

	path, err := store.pathForLocation(location)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, 0o200); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(path, 0o600) }()
	if file, openErr := os.Open(path); openErr == nil {
		_ = file.Close()
		t.Skip("platform does not enforce write-only test permissions")
	}

	store.mu.Lock()
	claimed, next, err := store.dispatchGuidanceItemsLocked(location, current, guidanceItems)
	store.mu.Unlock()
	if err != nil {
		t.Fatalf("dispatch commit must not depend on a post-commit read: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != "guide-a" || len(next) != 1 || next[0].ID != "queue-b" {
		t.Fatalf("unexpected dispatch result: claimed=%+v next=%+v", claimed, next)
	}

	if err = os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	replayed, err := NewInputQueueStore(root).Snapshot(location)
	if err != nil || len(replayed) != 1 || replayed[0].ID != "queue-b" {
		t.Fatalf("returned snapshot must match durable replay: items=%+v err=%v", replayed, err)
	}
}

func TestInputQueueStoreUntargetedGuidanceWaitsForAnyRound(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "agent")
	sessionKey := "agent:alpha:ws:dm:test"
	store := NewInputQueueStore(root)
	location := InputQueueLocation{
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: workspacePath,
		SessionKey:    sessionKey,
	}

	if _, err := store.Enqueue(location, protocol.InputQueueItem{
		ID:             "item-guide",
		Content:        "后续工具回来时补充这句",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		Source:         protocol.InputQueueSourceUser,
		RootRoundID:    "stale-round",
	}); err != nil {
		t.Fatalf("写入待引导队列失败: %v", err)
	}
	items, err := store.UpdateDeliveryPolicy(location, "item-guide", protocol.ChatDeliveryPolicyGuide)
	if err != nil {
		t.Fatalf("标记无绑定引导队列失败: %v", err)
	}
	if len(items) != 1 || items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide || items[0].RootRoundID != "" {
		t.Fatalf("无绑定引导不应保留旧 root_round_id: %+v", items)
	}

	dispatched, items, err := store.DispatchFirstDispatchable(location)
	if err != nil {
		t.Fatalf("派发普通队列失败: %v", err)
	}
	if dispatched != nil || len(items) != 1 {
		t.Fatalf("无绑定引导仍应等待 hook 消费: dispatched=%+v items=%+v", dispatched, items)
	}

	guidanceItems, items, err := store.DispatchGuidance(location, "future-round")
	if err != nil {
		t.Fatalf("未来 round 派发无绑定引导失败: %v", err)
	}
	if len(guidanceItems) != 1 || guidanceItems[0].ID != "item-guide" || len(items) != 0 {
		t.Fatalf("无绑定引导应被后续任意 round 的 hook 消费: guidance=%+v items=%+v", guidanceItems, items)
	}
}

func TestBoundedDirectedInputRemainsAcceptedAfterDispatch(t *testing.T) {
	root := t.TempDir()
	location := InputQueueLocation{Scope: protocol.InputQueueScopeRoom, WorkspacePath: filepath.Join(root, "agent"), SessionKey: "room:conversation:agent"}
	item := protocol.InputQueueItem{AgentID: "agent", Source: protocol.InputQueueSourceAgentRoomMessage, SourceMessageID: "message", ClientMessageID: "message", Content: "手机输入", DeliveryPolicy: protocol.ChatDeliveryPolicyQueue}
	store := NewInputQueueStore(root)
	items, inserted, err := store.EnqueueBounded(location, item, 1)
	if err != nil || !inserted {
		t.Fatalf("首次受理: %v %v", inserted, err)
	}
	if _, err := store.DispatchMany(location, []string{items[0].ID}); err != nil {
		t.Fatal(err)
	}
	// 新实例模拟重启；已经出队的输入不能因 wake 回执缺失再次执行。
	items, inserted, err = NewInputQueueStore(root).EnqueueBounded(location, item, 1)
	if err != nil || inserted || len(items) != 0 {
		t.Fatalf("恢复重放了已派发输入: %+v %v %v", items, inserted, err)
	}
}

func TestRestoreDispatchedPreservesExplicitOrderAcrossRestart(t *testing.T) {
	location := InputQueueLocation{WorkspacePath: t.TempDir(), SessionKey: "agent:a:ws:dm:restore", Scope: protocol.InputQueueScopeDM}
	store := NewInputQueueStore("")
	for _, id := range []string{"a", "b", "c"} {
		if _, err := store.Enqueue(location, protocol.InputQueueItem{ID: id, Content: id, CreatedAt: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Reorder(location, []string{"c", "a", "b"}); err != nil {
		t.Fatal(err)
	}
	item, _, err := store.DispatchFirstDispatchable(location)
	if err != nil {
		t.Fatal(err)
	}
	restarted := NewInputQueueStore("")
	for range 2 {
		items, err := restarted.RestoreDispatched(location, *item)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 3 || items[0].ID != "c" || items[1].ID != "a" || items[2].ID != "b" || items[0].QueueOrder != item.QueueOrder {
			t.Fatalf("restored order = %#v", items)
		}
	}
}

func TestQueueRecoveryPreservesClaimOrderAndStopAcrossRestart(t *testing.T) {
	root := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, root)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	location := InputQueueLocation{OwnerUserID: "owner", Scope: protocol.InputQueueScopeDM, WorkspacePath: filepath.Join(appfs.UserWorkspaceRootAt(root, "owner"), "agent-a"), SessionKey: "agent:agent-a:ws:dm:recover"}
	store := NewInputQueueStore("")
	for _, id := range []string{"first", "second"} {
		if _, err := store.Enqueue(location, protocol.InputQueueItem{ID: id, Content: id}); err != nil {
			t.Fatal(err)
		}
	}
	item, _, err := store.ClaimFirstDispatchable(location)
	if err != nil || item == nil || item.ID != "first" || item.DispatchPending {
		t.Fatalf("fresh claim: %+v %v", item, err)
	}
	store = NewInputQueueStore("")
	items, err := store.Snapshot(location)
	if err != nil || len(items) != 2 || items[0].ID != "first" || !items[0].DispatchPending {
		t.Fatalf("lost claim/order: %+v %v", items, err)
	}
	if err := store.SetDispatchPaused(location, true); err != nil {
		t.Fatal(err)
	}
	store = NewInputQueueStore("")
	item, items, err = store.ClaimFirstDispatchable(location)
	if err != nil || item != nil || !items[0].DispatchPaused {
		t.Fatalf("stop lost on restart: %+v %v", items, err)
	}
	locations, err := store.LocationsForWorkspace("owner", location.WorkspacePath)
	if err != nil || len(locations) != 1 || locations[0].SessionKey != location.SessionKey {
		t.Fatalf("queue without meta not discovered: %+v %v", locations, err)
	}
	if err := store.SetDispatchPaused(location, false); err != nil {
		t.Fatal(err)
	}
	item, _, err = store.ClaimFirstDispatchable(location)
	if err != nil || item == nil || !item.DispatchPending || item.ID != "first" {
		t.Fatalf("unknown claim treated fresh: %+v %v", item, err)
	}
	if _, err := store.Dispatch(location, item.ID); err != nil {
		t.Fatal(err)
	}
	items, err = NewInputQueueStore("").Snapshot(location)
	if err != nil || len(items) != 1 || items[0].ID != "second" {
		t.Fatalf("acceptance not committed: %+v %v", items, err)
	}
}

func TestExplicitWorkRecoveryPassesOnlyItsOwnStopGate(t *testing.T) {
	root := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, root)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	location := InputQueueLocation{OwnerUserID: "owner", Scope: protocol.InputQueueScopeDM, WorkspacePath: filepath.Join(appfs.UserWorkspaceRootAt(root, "owner"), "agent-a"), SessionKey: "agent:agent-a:ws:dm:explicit-recover"}
	store := NewInputQueueStore("")
	if _, err := store.Enqueue(location, protocol.InputQueueItem{ID: "other", Content: "other paused work"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetDispatchPaused(location, true); err != nil {
		t.Fatal(err)
	}
	target := &protocol.ExecutionRecoveryTarget{ExecutionID: "execution", WorkItemID: "work", AttemptID: "attempt"}
	request := protocol.InputQueueItem{ID: "recover", Source: protocol.InputQueueSourceHostRecovery, RecoveryExecutionID: "execution", RecoveryTarget: target, Content: "inspect exact work", DeliveryPolicy: protocol.ChatDeliveryPolicyQueue}
	revision, err := store.ContinuationPauseRevision(location)
	if err != nil {
		t.Fatal(err)
	}
	request.ContinuationPauseRevision = &revision
	request.AgentID = "agent-a"
	request.TargetAgentIDs = []string{"agent-a"}
	request.WorkBinding = &protocol.ExecutionWorkBinding{ExecutionID: "execution", PlanID: "plan", WorkItemID: "work", SpecID: "spec", AssignmentID: "assignment", AttemptID: "attempt"}
	first, err := store.EnqueueIdempotent(location, request, "request")
	if err != nil {
		t.Fatal(err)
	}
	request.ID = "ignored"
	duplicate, err := store.EnqueueIdempotent(location, request, "request-2")
	if err != nil || !duplicate.Duplicate || duplicate.Item.ID != first.Item.ID {
		t.Fatalf("coalesce=%+v %v", duplicate, err)
	}
	store = NewInputQueueStore("")
	item, items, err := store.ClaimFirstDispatchable(location)
	if err != nil || item == nil || item.ID != "recover" || !items[0].DispatchPaused {
		t.Fatalf("recovery admission=%+v %+v %v", item, items, err)
	}
	if paused, err := store.DispatchPaused(location); err != nil || !paused {
		t.Fatalf("other queue gate resumed: %v %v", paused, err)
	}
	if err := store.SetDispatchPaused(location, true); err != nil {
		t.Fatal(err)
	}
	delayed := request
	delayed.ID = "delayed"
	copied := *request.RecoveryTarget
	copied.AttemptID = "delayed-attempt"
	delayed.RecoveryTarget = &copied
	binding := *request.WorkBinding
	binding.AttemptID = copied.AttemptID
	delayed.WorkBinding = &binding
	if _, err = store.EnqueueIdempotent(location, delayed, "delayed-request"); !errors.Is(err, ErrContinuationStopped) {
		t.Fatalf("delayed delivery revived stopped work: %v", err)
	}
	// Replaying the same accepted request cannot undo a later user stop.
	if _, err := store.EnqueueIdempotent(location, request, "request"); err != nil {
		t.Fatal(err)
	}
	item, _, err = NewInputQueueStore("").ClaimFirstDispatchable(location)
	if err != nil || item != nil {
		t.Fatalf("later user stop lost: %+v %v", item, err)
	}
	accepted, ok, err := store.FindAcceptedEnqueue(location, "request")
	if err != nil || !ok || accepted.RecoveryTarget == nil || *accepted.RecoveryTarget != *target {
		t.Fatalf("lost durable target: %+v %v", accepted, err)
	}
}

func TestInputQueueStoreRejectsInvalidCapabilityEnvelopeAcrossEnqueuePaths(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	location := InputQueueLocation{
		OwnerUserID:    "owner-1",
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, "owner-1"), "agent-worker"),
		SessionKey:     "room:agent:conversation-1:agent-worker",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
	}
	invalid := protocol.InputQueueItem{
		ID:             "invalid-bound-guide",
		AgentID:        "agent-worker",
		TargetAgentIDs: []string{"agent-worker"},
		Source:         protocol.InputQueueSourceAgentRoomMessage,
		Content:        "must not enter an active round as guidance",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
		WorkBinding: &protocol.ExecutionWorkBinding{
			ExecutionID:  "execution-1",
			PlanID:       "plan-1",
			WorkItemID:   "work-1",
			SpecID:       "spec-1",
			AssignmentID: "assignment-1",
			AttemptID:    "attempt-1",
			DispatchID:   "dispatch-1",
		},
	}
	store := NewInputQueueStore("")
	assertRejected := func(name string, err error) {
		t.Helper()
		if !errors.Is(err, protocol.ErrInvalidInputQueueCapabilityEnvelope) {
			t.Fatalf("%s error = %v, want capability envelope error", name, err)
		}
	}
	_, err := store.Enqueue(location, invalid)
	assertRejected("enqueue", err)
	_, err = store.EnqueueIdempotent(location, invalid, "client-message-1")
	assertRejected("idempotent enqueue", err)
	_, _, err = store.EnqueueBounded(location, invalid, 8)
	assertRejected("bounded enqueue", err)
	_, err = store.EnqueueBatchWithItems([]InputQueueEnqueue{{Location: location, Item: invalid}})
	assertRejected("batch enqueue", err)
}

func TestInputQueuePersistsIndependentExecutionReviewBinding(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	location := InputQueueLocation{
		OwnerUserID:    "owner-1",
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, "owner-1"), "agent-lead"),
		SessionKey:     "room:agent:conversation-1:agent-lead",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
	}
	binding := &protocol.ExecutionReviewBinding{
		ExecutionID:      "execution-1",
		PlanID:           "plan-1",
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		SubmissionID:     "submission-1",
		ReviewDispatchID: "review-dispatch-1",
		TargetAgentID:    "agent-lead",
	}
	store := NewInputQueueStore("")
	if _, err := store.Enqueue(location, protocol.InputQueueItem{
		ID:              "execution_review_dispatch_review-dispatch-1",
		AgentID:         "agent-lead",
		SourceAgentID:   "agent-worker",
		SourceMessageID: "execution_review_dispatch_review-dispatch-1",
		TargetAgentIDs:  []string{"agent-lead"},
		Source:          protocol.InputQueueSourceAgentRoomMessage,
		Content:         "review the submitted evidence",
		DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
		ReviewBinding:   binding,
	}); err != nil {
		t.Fatal(err)
	}
	replayed, err := NewInputQueueStore("").Snapshot(location)
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed) != 1 ||
		replayed[0].WorkBinding != nil ||
		!reflect.DeepEqual(replayed[0].ReviewBinding, binding) {
		t.Fatalf("replayed ReviewBinding = %+v, want %+v", replayed, binding)
	}
	different := replayed[0]
	cloned := *binding
	cloned.SubmissionID = "submission-stale"
	different.ReviewBinding = &cloned
	if MatchesInputQueueEnqueueIntent(replayed[0], different) {
		t.Fatal("different Submission review binding matched the same enqueue intent")
	}
}
