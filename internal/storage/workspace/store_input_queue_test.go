package workspace

import (
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
