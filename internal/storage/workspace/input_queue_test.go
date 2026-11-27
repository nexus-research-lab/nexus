// input_queue_test.go 覆盖工作区持久化输入队列、恢复与 WorkBinding 测试。
package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestInputQueueReplayIgnoresPersistedMismatchedOwner(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	location := InputQueueLocation{
		OwnerUserID:   "user-a",
		Scope:         protocol.InputQueueScopeRoom,
		WorkspacePath: filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, "user-a"), "agent-a"),
		SessionKey:    "agent:agent-a:ws:group:owner-replay",
	}
	store := NewInputQueueStore("")
	if err := store.appendActionLocked(location, map[string]any{
		"action": inputQueueActionEnqueue,
		"item": protocol.InputQueueItem{
			ID:          "foreign-item",
			OwnerUserID: "user-b",
			Scope:       protocol.InputQueueScopeRoom,
			Content:     "伪造恢复项",
		},
		"client_message_id": "foreign-request",
		"timestamp":         time.Now().UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
	items, err := store.Snapshot(location)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("owner 不匹配的历史队列项不应被恢复: %+v", items)
	}
	accepted, err := store.EnqueueIdempotent(location, protocol.InputQueueItem{
		Content:     "合法恢复项",
		OwnerUserID: location.OwnerUserID,
		Scope:       location.Scope,
	}, "foreign-request")
	if err != nil || accepted.Duplicate {
		t.Fatalf("owner 不匹配的历史记录不能占用幂等键: result=%+v err=%v", accepted, err)
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

func TestInputQueueStoreReplayAppendReorderDispatchAndDelete(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "agent")
	sessionKey := "agent:alpha:ws:dm:test"
	store := NewInputQueueStore(root)
	location := InputQueueLocation{
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: workspacePath,
		SessionKey:    sessionKey,
	}

	items, err := store.Enqueue(location, protocol.InputQueueItem{
		ID:             "item-a",
		Content:        "第一条",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		Source:         protocol.InputQueueSourceUser,
	})
	if err != nil {
		t.Fatalf("写入第一条队列失败: %v", err)
	}
	if len(items) != 1 || items[0].ID != "item-a" {
		t.Fatalf("第一条队列快照不正确: %#v", items)
	}

	if _, err = store.Enqueue(location, protocol.InputQueueItem{
		ID:             "item-b",
		Content:        "第二条",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
		Source:         protocol.InputQueueSourceUser,
	}); err != nil {
		t.Fatalf("写入第二条队列失败: %v", err)
	}

	items, err = store.Reorder(location, []string{"item-b", "item-a"})
	if err != nil {
		t.Fatalf("重排队列失败: %v", err)
	}
	if len(items) != 2 || items[0].ID != "item-b" || items[1].ID != "item-a" {
		t.Fatalf("重排队列快照不正确: %#v", items)
	}

	dispatched, items, err := store.DispatchNext(location)
	if err != nil {
		t.Fatalf("派发队首失败: %v", err)
	}
	if dispatched == nil || dispatched.ID != "item-b" {
		t.Fatalf("派发队首不正确: %#v", dispatched)
	}
	if len(items) != 1 || items[0].ID != "item-a" {
		t.Fatalf("派发后队列快照不正确: %#v", items)
	}

	if _, err = store.Delete(location, "item-a"); err != nil {
		t.Fatalf("删除队列项失败: %v", err)
	}
	items, err = store.Snapshot(location)
	if err != nil {
		t.Fatalf("读取队列快照失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("删除后队列应为空: %#v", items)
	}

	reloaded := NewInputQueueStore(root)
	items, err = reloaded.Snapshot(location)
	if err != nil {
		t.Fatalf("重放队列快照失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("重放删除/派发事件后队列应为空: %#v", items)
	}
}

func TestInputQueueStoreSeparatesDMAndRoomScopesAtSameSessionPath(t *testing.T) {
	root := t.TempDir()
	store := NewInputQueueStore(root)
	workspacePath := filepath.Join(root, "agent")
	sessionKey := "agent:alpha:ws:dm:shared-conversation"
	dmLocation := InputQueueLocation{
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: workspacePath,
		SessionKey:    sessionKey,
	}
	roomLocation := InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  workspacePath,
		SessionKey:     sessionKey,
		RoomID:         "room-1",
		ConversationID: "shared-conversation",
	}

	if _, err := store.Enqueue(dmLocation, protocol.InputQueueItem{
		ID:      "dm-item",
		Content: "DM 后续指令",
		Source:  protocol.InputQueueSourceUser,
	}); err != nil {
		t.Fatal(err)
	}
	roomItems, err := store.Snapshot(roomLocation)
	if err != nil {
		t.Fatal(err)
	}
	if len(roomItems) != 0 {
		t.Fatalf("Room 队列不得回放 DM 队列项: %+v", roomItems)
	}

	if _, err = store.Enqueue(roomLocation, protocol.InputQueueItem{
		ID:      "room-item",
		Scope:   protocol.InputQueueScopeDM,
		Content: "Room 后续指令",
		Source:  protocol.InputQueueSourceUser,
	}); err != nil {
		t.Fatal(err)
	}
	dmItems, err := store.Snapshot(dmLocation)
	if err != nil {
		t.Fatal(err)
	}
	if len(dmItems) != 1 || dmItems[0].ID != "dm-item" {
		t.Fatalf("DM 队列只能看到 DM 队列项: %+v", dmItems)
	}
	roomItems, err = store.Snapshot(roomLocation)
	if err != nil {
		t.Fatal(err)
	}
	if len(roomItems) != 1 || roomItems[0].ID != "room-item" {
		t.Fatalf("Room 队列只能看到 Room 队列项: %+v", roomItems)
	}
	if roomItems[0].Scope != protocol.InputQueueScopeRoom {
		t.Fatalf("队列位置 scope 必须覆盖 item scope: %+v", roomItems[0])
	}

	dispatched, remaining, err := store.DispatchNext(roomLocation)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched == nil || dispatched.ID != "room-item" || len(remaining) != 0 {
		t.Fatalf("Room 派发不得消费 DM 队列项: dispatched=%+v remaining=%+v", dispatched, remaining)
	}
	dmItems, err = store.Snapshot(dmLocation)
	if err != nil {
		t.Fatal(err)
	}
	if len(dmItems) != 1 || dmItems[0].ID != "dm-item" {
		t.Fatalf("Room 派发后 DM 队列项必须保留: %+v", dmItems)
	}
}

func TestInputQueueStoreRejectsOversizedClientMessageID(t *testing.T) {
	root := t.TempDir()
	store := NewInputQueueStore(root)
	_, err := store.EnqueueIdempotent(InputQueueLocation{
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: filepath.Join(root, "agent"),
		SessionKey:    "agent:alpha:ws:dm:oversized-id",
	}, protocol.InputQueueItem{
		Content: "bounded",
		Source:  protocol.InputQueueSourceUser,
	}, strings.Repeat("x", protocol.MaxClientMessageIDBytes+1))
	if err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("超长 client_message_id 应在写盘前拒绝: %v", err)
	}
}

func TestInputQueueStoreIdempotencyConflictDoesNotAppend(t *testing.T) {
	root := t.TempDir()
	location := InputQueueLocation{
		Scope:         protocol.InputQueueScopeRoom,
		WorkspacePath: filepath.Join(root, "agent"),
		SessionKey:    "agent:alpha:ws:group:idempotent-conflict",
	}
	store := NewInputQueueStore(root)
	original := protocol.InputQueueItem{
		Content:        "分析 M4",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		TargetAgentIDs: []string{"agent-a"},
		Source:         protocol.InputQueueSourceUser,
		OwnerUserID:    "owner-1",
	}
	if _, err := store.EnqueueIdempotent(location, original, "client-message-conflict"); err != nil {
		t.Fatal(err)
	}
	conflicting := original
	conflicting.Content = "分析 M5"
	if _, err := store.EnqueueIdempotent(location, conflicting, "client-message-conflict"); !errors.Is(err, ErrInputQueueIdempotencyConflict) {
		t.Fatalf("same key with different intent must conflict: %v", err)
	}

	path, err := store.pathForLocation(location)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := store.files.readJSONL(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("conflict must not append a log row: %+v", rows)
	}
}

func TestMatchesInputQueueEnqueueIntentIgnoresPhysicalLocation(t *testing.T) {
	existing := protocol.InputQueueItem{
		ID:             "item-a",
		Scope:          protocol.InputQueueScopeRoom,
		SessionKey:     "agent:a:ws:group:conversation",
		RoomID:         "room-a",
		ConversationID: "conversation-a",
		AgentID:        "agent-a",
		Content:        " 协作分析 ",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		TargetAgentIDs: []string{"agent-b", "agent-a"},
		Source:         protocol.InputQueueSourceUser,
		OwnerUserID:    " owner ",
		CreatedAt:      1,
		UpdatedAt:      2,
		QueueOrder:     3,
	}
	candidate := existing
	candidate.ID = "item-b"
	candidate.SessionKey = "agent:b:ws:group:conversation"
	candidate.RoomID = "room-b"
	candidate.ConversationID = "conversation-b"
	candidate.AgentID = "agent-b"
	candidate.TargetAgentIDs = []string{"agent-a", "agent-b", "agent-a"}
	candidate.CreatedAt = 100
	candidate.UpdatedAt = 200
	candidate.QueueOrder = 300

	if !MatchesInputQueueEnqueueIntent(existing, candidate) {
		t.Fatal("physical location, ID, time and target order must not change logical intent")
	}
	candidate.OwnerUserID = "other-owner"
	if MatchesInputQueueEnqueueIntent(existing, candidate) {
		t.Fatal("owner change must be an idempotency conflict")
	}
	candidate.OwnerUserID = existing.OwnerUserID
	candidate.TargetAgentIDs = []string{"agent-a"}
	if MatchesInputQueueEnqueueIntent(existing, candidate) {
		t.Fatal("resolved target set change must be an idempotency conflict")
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

func TestInputQueueStoreGuidanceWaitsForMatchingRound(t *testing.T) {
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
		ID:             "item-a",
		Content:        "普通消息",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		Source:         protocol.InputQueueSourceUser,
	}); err != nil {
		t.Fatalf("写入普通队列失败: %v", err)
	}
	if _, err := store.Enqueue(location, protocol.InputQueueItem{
		ID:             "item-b",
		Content:        "引导消息",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		Source:         protocol.InputQueueSourceUser,
	}); err != nil {
		t.Fatalf("写入引导队列失败: %v", err)
	}
	items, err := store.UpdateDeliveryPolicy(location, "item-b", protocol.ChatDeliveryPolicyGuide, "round-running")
	if err != nil {
		t.Fatalf("标记引导队列失败: %v", err)
	}
	if len(items) != 2 || items[1].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide || items[1].RootRoundID != "round-running" {
		t.Fatalf("引导队列标记不正确: %+v", items)
	}

	dispatched, items, err := store.DispatchFirstDispatchable(location)
	if err != nil {
		t.Fatalf("派发普通队列失败: %v", err)
	}
	if dispatched == nil || dispatched.ID != "item-a" || len(items) != 1 || items[0].ID != "item-b" {
		t.Fatalf("普通派发应跳过等待引导的队列项: dispatched=%+v items=%+v", dispatched, items)
	}

	guidanceItems, items, err := store.DispatchGuidance(location, "other-round")
	if err != nil {
		t.Fatalf("非匹配 round 派发引导失败: %v", err)
	}
	if len(guidanceItems) != 0 || len(items) != 1 {
		t.Fatalf("非匹配 round 不应消费引导: guidance=%+v items=%+v", guidanceItems, items)
	}

	guidanceItems, items, err = store.DispatchGuidance(location, "round-running")
	if err != nil {
		t.Fatalf("匹配 round 派发引导失败: %v", err)
	}
	if len(guidanceItems) != 1 || guidanceItems[0].ID != "item-b" || len(items) != 0 {
		t.Fatalf("匹配 round 应消费引导: guidance=%+v items=%+v", guidanceItems, items)
	}
}

func TestInputQueueStoreDispatchPreparedGuidanceIsAllOrNone(t *testing.T) {
	root := t.TempDir()
	location := InputQueueLocation{
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: filepath.Join(root, "agent"),
		SessionKey:    "agent:alpha:ws:dm:prepared-guidance",
	}
	store := NewInputQueueStore(root)
	for _, item := range []protocol.InputQueueItem{
		{
			ID:             "item-a",
			Content:        "第一条引导",
			DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
			Source:         protocol.InputQueueSourceUser,
			RootRoundID:    "round-running",
		},
		{
			ID:             "item-b",
			Content:        "第二条引导",
			DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
			Source:         protocol.InputQueueSourceUser,
			RootRoundID:    "round-running",
		},
	} {
		if _, err := store.Enqueue(location, item); err != nil {
			t.Fatalf("写入引导队列失败: %v", err)
		}
	}

	prepared, err := store.SnapshotGuidance(location, "round-running")
	if err != nil {
		t.Fatalf("预检引导队列失败: %v", err)
	}
	if len(prepared) != 2 {
		t.Fatalf("预检应返回两条引导: %+v", prepared)
	}
	if _, err = store.UpdateDeliveryPolicy(location, "item-b", protocol.ChatDeliveryPolicyQueue); err != nil {
		t.Fatalf("模拟预检后队列变化失败: %v", err)
	}

	claimed, snapshot, err := store.DispatchPreparedGuidance(location, prepared, "round-running")
	if err != nil {
		t.Fatalf("提交预检引导失败: %v", err)
	}
	if len(claimed) != 0 {
		t.Fatalf("任一预检项变化时不应消费任何引导: %+v", claimed)
	}
	if len(snapshot) != 2 || snapshot[0].ID != "item-a" || snapshot[1].ID != "item-b" {
		t.Fatalf("预检冲突后应保留整批队列项: %+v", snapshot)
	}

	reloaded, err := NewInputQueueStore(root).Snapshot(location)
	if err != nil {
		t.Fatalf("重放预检冲突后的持久队列失败: %v", err)
	}
	if len(reloaded) != 2 ||
		reloaded[0].ID != "item-a" || reloaded[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide ||
		reloaded[1].ID != "item-b" || reloaded[1].DeliveryPolicy != protocol.ChatDeliveryPolicyQueue {
		t.Fatalf("预检冲突不应留下部分消费事件: %+v", reloaded)
	}

	if _, err = store.UpdateDeliveryPolicy(location, "item-b", protocol.ChatDeliveryPolicyGuide); err != nil {
		t.Fatalf("恢复第二条引导失败: %v", err)
	}
	prepared, err = store.SnapshotGuidance(location, "round-running")
	if err != nil {
		t.Fatalf("重新预检引导失败: %v", err)
	}
	claimed, snapshot, err = store.DispatchPreparedGuidance(location, prepared, "round-running")
	if err != nil || len(claimed) != 2 || len(snapshot) != 0 {
		t.Fatalf("整批引导消费失败: claimed=%+v snapshot=%+v err=%v", claimed, snapshot, err)
	}
	reloaded, err = NewInputQueueStore(root).Snapshot(location)
	if err != nil || len(reloaded) != 0 {
		t.Fatalf("批量 dispatch 事件重放后队列应为空: items=%+v err=%v", reloaded, err)
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

func TestQueueRecoveryCoalescesPendingCoordinatorWake(t *testing.T) {
	root := t.TempDir()
	store := NewInputQueueStore(root)
	location := InputQueueLocation{Scope: protocol.InputQueueScopeDM, WorkspacePath: root, SessionKey: "agent:lead:ws:dm:recover"}
	item := protocol.InputQueueItem{Source: protocol.InputQueueSourceHostRecovery, RecoveryExecutionID: "execution", Content: "inspect current work"}
	first, err := store.EnqueueIdempotent(location, item, "startup:execution:1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.EnqueueIdempotent(location, item, "failed:attempt")
	if err != nil || !second.Duplicate || second.Item.ID != first.Item.ID || len(second.Items) != 1 {
		t.Fatalf("duplicate coordinator turn: %+v %v", second, err)
	}
	if _, err := store.Dispatch(location, first.Item.ID); err != nil {
		t.Fatal(err)
	}
	retry, err := NewInputQueueStore(root).EnqueueIdempotent(location, item, "failed:attempt")
	if err != nil || !retry.Duplicate || len(retry.Items) != 0 {
		t.Fatalf("coalesced receipt lost: %+v %v", retry, err)
	}
	next, err := store.EnqueueIdempotent(location, item, "startup:execution:2")
	if err != nil || next.Duplicate || len(next.Items) != 1 {
		t.Fatalf("new recovery suppressed: %+v %v", next, err)
	}
}

func TestQueueDiscoverySkipsUnreadableJournalAndKeepsOthers(t *testing.T) {
	root := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, root)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	workspace := filepath.Join(appfs.UserWorkspaceRootAt(root, "owner"), "agent-a")
	good := InputQueueLocation{OwnerUserID: "owner", Scope: protocol.InputQueueScopeDM, WorkspacePath: workspace, SessionKey: "agent:agent-a:ws:dm:good"}
	store := NewInputQueueStore("")
	if _, err := store.Enqueue(good, protocol.InputQueueItem{ID: "x", Content: "x"}); err != nil {
		t.Fatal(err)
	}
	badDir := filepath.Join(workspace, ".agents", "sessions", "bad-session")
	if err := os.MkdirAll(badDir, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "elsewhere.jsonl")
	if err := os.WriteFile(outside, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(badDir, "input_queue.jsonl")); err != nil {
		t.Fatal(err)
	}

	locations, err := store.LocationsForWorkspace("owner", workspace)

	if err == nil {
		t.Fatal("unreadable journal was not reported")
	}
	if len(locations) != 1 || locations[0].SessionKey != good.SessionKey {
		t.Fatalf("locations = %+v, want the readable queue still discovered", locations)
	}
}

func TestRestoreDispatchedRewindsClaimToPreDispatchState(t *testing.T) {
	root := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, root)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	location := InputQueueLocation{OwnerUserID: "owner", Scope: protocol.InputQueueScopeDM, WorkspacePath: filepath.Join(appfs.UserWorkspaceRootAt(root, "owner"), "agent-a"), SessionKey: "agent:agent-a:ws:dm:rewind"}
	store := NewInputQueueStore("")
	if _, err := store.Enqueue(location, protocol.InputQueueItem{ID: "first", Content: "first"}); err != nil {
		t.Fatal(err)
	}
	fresh, _, err := store.ClaimFirstDispatchable(location)
	if err != nil || fresh == nil || fresh.DispatchPending {
		t.Fatalf("fresh claim = %+v, %v", fresh, err)
	}
	items, err := store.RestoreDispatched(location, *fresh)
	if err != nil || len(items) != 1 || items[0].DispatchPending {
		t.Fatalf("known failure kept an unknown-outcome claim: %+v %v", items, err)
	}

	inherited, _, err := store.ClaimFirstDispatchable(location)
	if err != nil || inherited == nil {
		t.Fatal(err)
	}
	store = NewInputQueueStore("")
	survivor, _, err := store.ClaimFirstDispatchable(location)
	if err != nil || survivor == nil || !survivor.DispatchPending {
		t.Fatalf("restart lost the surviving claim: %+v %v", survivor, err)
	}
	items, err = store.RestoreDispatched(location, *survivor)
	if err != nil || len(items) != 1 || !items[0].DispatchPending {
		t.Fatalf("inherited claim lost its unknown outcome: %+v %v", items, err)
	}
}

func TestHasRoundMarkerMatchesExactRound(t *testing.T) {
	root := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, root)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	workspace := filepath.Join(appfs.UserWorkspaceRootAt(root, "owner"), "agent-a")
	history := NewAgentHistoryStore("").ForOwner("owner")
	sessionKey := "agent:agent-a:ws:dm:marker"
	if found, err := history.HasRoundMarker(workspace, sessionKey, "queue_x"); err != nil || found {
		t.Fatalf("empty history found=%v err=%v", found, err)
	}
	if err := history.AppendRoundMarkerWithOptions(workspace, sessionKey, "queue_x", "hello", 1, RoundMarkerOptions{}); err != nil {
		t.Fatal(err)
	}
	if found, err := history.HasRoundMarker(workspace, sessionKey, "queue_x"); err != nil || !found {
		t.Fatalf("persisted round found=%v err=%v", found, err)
	}
	if found, err := history.HasRoundMarker(workspace, sessionKey, "queue_y"); err != nil || found {
		t.Fatalf("other round found=%v err=%v", found, err)
	}
}

func TestInputQueueStoreRejectsGuidanceMutationForExecutionBinding(t *testing.T) {
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
	item := protocol.InputQueueItem{
		ID:              "execution_dispatch_dispatch-1",
		AgentID:         "agent-worker",
		SourceAgentID:   "agent-lead",
		SourceMessageID: "execution_dispatch_dispatch-1",
		TargetAgentIDs:  []string{"agent-worker"},
		Source:          protocol.InputQueueSourceAgentRoomMessage,
		Content:         "deliver the evidence set",
		DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
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
	if _, err := store.Enqueue(location, item); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateDeliveryPolicy(
		location,
		item.ID,
		protocol.ChatDeliveryPolicyGuide,
		"round-running",
	); !errors.Is(err, protocol.ErrInvalidInputQueueCapabilityEnvelope) {
		t.Fatalf("guide mutation error = %v, want capability envelope error", err)
	}
	items, err := store.Snapshot(location)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyQueue {
		t.Fatalf("bound item changed after rejected guidance mutation: %+v", items)
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
