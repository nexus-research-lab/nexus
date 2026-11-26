package realtime

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func roomWakeTrigger() messagepkg.RuntimeWakeTrigger {
	return messagepkg.RuntimeWakeTrigger{TaskID: "b1", Status: "completed", Summary: "build done"}
}

// 公区 slot 的自唤醒：触发行可见、ack 广播、slot 公开输出。
func TestRoomRuntimeWakePublicSlotShowsTriggerRow(t *testing.T) {
	wake := &roomRuntimeWakeStart{trigger: roomWakeTrigger(), replyRoute: protocol.RoomReplyRoute{Mode: protocol.RoomReplyRoutePublic}}
	request := ChatRequest{Internal: true, runtimeWake: wake}
	row := decorateRoomRuntimeWakeUserMessage(request, protocol.Message{"hidden_from_user": true, "content": "x"})
	if _, hidden := row["hidden_from_user"]; hidden || row["content"] != "build done" {
		t.Fatalf("row = %+v", row)
	}
	if !shouldBroadcastRoomChatAck(request) {
		t.Fatal("public wake must broadcast ack")
	}
	slot := &activeRoomSlot{HiddenFromUser: true}
	applyRuntimeWakeSlot(request, slot)
	if slot.HiddenFromUser || slot.runtimeWake != wake || !roomSlotPublishesPublicOutput(slot) {
		t.Fatalf("slot = %+v", slot)
	}
}

// 私域 slot 的自唤醒：沿用私域路由，触发行与输出都不进入公区。
func TestRoomRuntimeWakePrivateSlotStaysPrivate(t *testing.T) {
	route := protocol.RoomReplyRoute{Mode: protocol.RoomReplyRoutePrivate, Recipients: []string{"agent-b"}}
	wake := &roomRuntimeWakeStart{trigger: roomWakeTrigger(), replyRoute: route}
	request := ChatRequest{Internal: true, runtimeWake: wake}
	row := decorateRoomRuntimeWakeUserMessage(request, protocol.Message{"content": "x"})
	if row["hidden_from_user"] != true {
		t.Fatalf("private wake row must stay hidden: %+v", row)
	}
	if shouldBroadcastRoomChatAck(request) {
		t.Fatal("private wake must not broadcast ack")
	}
	slot := &activeRoomSlot{}
	applyRuntimeWakeSlot(request, slot)
	if !slot.HiddenFromUser || roomSlotPublishesPublicOutput(slot) {
		t.Fatalf("slot must keep private route: %+v", slot.replyRoute())
	}
}

// 普通输入在交接期间让路；自唤醒请求自身不被闸门拦住。
func TestRoomRuntimeWakeGateOnlyBlocksOrdinaryInput(t *testing.T) {
	service := &Service{}
	release := service.RuntimeWakeGates.Begin("conv-1")
	if service.yieldToRoomRuntimeWake(ChatRequest{}, "conv-1") == nil {
		t.Fatal("ordinary input must wait for handoff")
	}
	if service.yieldToRoomRuntimeWake(ChatRequest{runtimeWake: &roomRuntimeWakeStart{}}, "conv-1") != nil {
		t.Fatal("wake request must pass its own gate")
	}
	if service.yieldToRoomRuntimeWake(ChatRequest{}, "conv-2") != nil {
		t.Fatal("other conversations are unaffected")
	}
	release()
	if service.yieldToRoomRuntimeWake(ChatRequest{}, "conv-1") != nil {
		t.Fatal("gate must open after release")
	}
}

// drain 观察到的通知只用于下一次唤醒，取出后清空。
func TestRoomIdleWakeObserverTakesLatestTopLevelNotification(t *testing.T) {
	parent := "toolu_parent"
	observer := &roomIdleWakeObserver{}
	observer.observe(sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem, TaskNotification: &sdkprotocol.TaskNotificationMessage{TaskID: "b1", Summary: "first"}})
	observer.observe(sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem, ParentToolUseID: &parent, TaskNotification: &sdkprotocol.TaskNotificationMessage{TaskID: "child"}})
	if got := observer.take(); got.TaskID != "b1" {
		t.Fatalf("take = %+v", got)
	}
	if got := observer.take(); got.TaskID != "" {
		t.Fatalf("second take must be empty: %+v", got)
	}
}
