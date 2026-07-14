package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func TestInputQueueStoreEnqueueBatchWithItemsReturnsCommittedVersions(t *testing.T) {
	root := t.TempDir()
	store := NewInputQueueStore(root)
	location := InputQueueLocation{
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: filepath.Join(root, "agent"),
		SessionKey:    "agent:alpha:ws:dm:batch-versions",
	}
	committed, err := store.EnqueueBatchWithItems([]InputQueueEnqueue{{
		Location: location,
		Item: protocol.InputQueueItem{
			ID:             "guide-versioned",
			Content:        "恢复后继续确认",
			DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
			CreatedAt:      1,
			UpdatedAt:      1,
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(committed) != 1 || committed[0].UpdatedAt <= 1 {
		t.Fatalf("committed items must contain the normalized CAS version: %+v", committed)
	}
	snapshot, err := store.Snapshot(location)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshot, committed) {
		t.Fatalf("returned committed items differ from durable snapshot: committed=%+v snapshot=%+v", committed, snapshot)
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

func TestInputQueueStoreCancelGuidanceRestoresDispatchableQueue(t *testing.T) {
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
		Content:        "取消后作为普通队列发送",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
		Source:         protocol.InputQueueSourceUser,
		RootRoundID:    "stale-round",
	}); err != nil {
		t.Fatalf("写入引导队列失败: %v", err)
	}
	items, err := store.UpdateDeliveryPolicy(location, "item-guide", protocol.ChatDeliveryPolicyQueue)
	if err != nil {
		t.Fatalf("取消引导失败: %v", err)
	}
	if len(items) != 1 || items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyQueue || items[0].RootRoundID != "" {
		t.Fatalf("取消引导后应恢复普通队列并清理 root_round_id: %+v", items)
	}

	dispatched, items, err := store.DispatchFirstDispatchable(location)
	if err != nil {
		t.Fatalf("派发取消引导后的队列失败: %v", err)
	}
	if dispatched == nil || dispatched.ID != "item-guide" || len(items) != 0 {
		t.Fatalf("取消引导后的队列应可正常派发: dispatched=%+v items=%+v", dispatched, items)
	}
}

func TestInputQueueStoreRoomScopeUsesAgentSessionPath(t *testing.T) {
	root := t.TempDir()
	store := NewInputQueueStore(root)
	workspacePath := filepath.Join(root, "sam")
	sessionKey := protocol.BuildRoomAgentSessionKey("conversation-1", "agent-sam", protocol.RoomTypeGroup)
	location := InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  workspacePath,
		SessionKey:     sessionKey,
		RoomID:         "room-1",
		ConversationID: "conversation-1",
	}

	if _, err := store.Enqueue(location, protocol.InputQueueItem{
		ID:             "room-item",
		Content:        "@Sam 看下这个",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		Source:         protocol.InputQueueSourceUser,
	}); err != nil {
		t.Fatalf("写入 Room 队列失败: %v", err)
	}

	items, err := NewInputQueueStore(root).Snapshot(location)
	if err != nil {
		t.Fatalf("读取 Room 队列失败: %v", err)
	}
	if len(items) != 1 || items[0].Scope != protocol.InputQueueScopeRoom || items[0].ConversationID != "conversation-1" {
		t.Fatalf("Room 队列快照不正确: %#v", items)
	}
	if items[0].SessionKey != sessionKey {
		t.Fatalf("Room 队列应归属 agent session: %#v", items[0])
	}
}
