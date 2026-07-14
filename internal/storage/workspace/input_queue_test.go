package workspace

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

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
