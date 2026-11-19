package dm

import (
	"context"
	"errors"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
)

// 唤醒说明来自最近一次顶层后台任务通知，交接后清空，避免下一次唤醒复用旧摘要。
func TestRuntimeWakeTriggerUsesLatestTaskNotification(t *testing.T) {
	runner := &roundRunner{}
	runner.observeIdleTaskNotification(sdkprotocol.ReceivedMessage{
		Type:             sdkprotocol.MessageTypeSystem,
		TaskNotification: &sdkprotocol.TaskNotificationMessage{TaskID: "b1", Status: "completed", Summary: "first"},
	})
	runner.observeIdleTaskNotification(sdkprotocol.ReceivedMessage{
		Type:   sdkprotocol.MessageTypeSystem,
		System: &sdkprotocol.SystemMessage{TaskNotification: &sdkprotocol.TaskNotificationMessage{TaskID: "b2", Status: "failed", Summary: "second"}},
	})
	trigger := runner.idleWakeTrigger
	if trigger.TaskID != "b2" || trigger.Content() != "second" {
		t.Fatalf("trigger = %+v", trigger)
	}
	metadata := trigger.Metadata()
	if metadata["input_kind"] != "runtime_wake" || metadata["task_status"] != "failed" || metadata["trigger_kind"] != "background_task" {
		t.Fatalf("metadata = %+v", metadata)
	}
}

// 交接期间普通输入让路并在交接结束后放行；自唤醒 round 自身不受闸门影响。
func TestRuntimeWakeGateYieldsOrdinaryInputUntilHandoffEnds(t *testing.T) {
	service := &Service{}
	release := service.RuntimeWakeGates.Begin("agent:a:ws:dm:c")
	ordinary := &dmChatExecution{service: service, sessionKey: "agent:a:ws:dm:c"}
	err := ordinary.yieldToRuntimeWake()
	var pending *runtimeWakePendingError
	if !errors.As(err, &pending) || !errors.Is(err, runtimehost.ErrRuntimeWakeHandoffPending) {
		t.Fatalf("ordinary yield err = %v", err)
	}
	wake := &dmChatExecution{service: service, sessionKey: "agent:a:ws:dm:c", request: Request{runtimeWake: &runtimeWakeStart{}}}
	if err := wake.yieldToRuntimeWake(); err != nil {
		t.Fatalf("wake round must pass its own gate: %v", err)
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		release()
		release()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := pending.wait(ctx); err != nil {
		t.Fatalf("wait err = %v", err)
	}
	if err := ordinary.yieldToRuntimeWake(); err != nil {
		t.Fatalf("after release err = %v", err)
	}
}
