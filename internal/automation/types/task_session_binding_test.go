package types

import (
	"reflect"
	"testing"
)

func TestInvalidateScheduledTaskSessionsRequiresEveryBindingToBeReassigned(t *testing.T) {
	deletedKey := "agent:agent-1:weixin-personal:dm:account-a:peer-a"
	task := ScheduledTask{
		Enabled: true,
		SessionTarget: SessionTarget{
			Kind:            SessionTargetBound,
			BoundSessionKey: deletedKey,
		},
		Delivery: DeliveryTarget{
			Mode:       DeliveryModeLast,
			SessionKey: deletedKey,
		},
	}

	invalidated, changed := InvalidateScheduledTaskSessions(task, []string{deletedKey})
	if !changed || invalidated.Enabled ||
		invalidated.SessionBindingState != TaskSessionBindingStateRebindRequired {
		t.Fatalf("失效状态不正确: %+v", invalidated)
	}
	if !reflect.DeepEqual(
		invalidated.SessionBindingIssues,
		[]string{TaskSessionBindingIssueExecution, TaskSessionBindingIssueDelivery},
	) {
		t.Fatalf("失效范围 = %v", invalidated.SessionBindingIssues)
	}

	invalidated.Delivery.SessionKey = "agent:agent-1:websocket:dm:new-delivery"
	partiallyRebound := NormalizeScheduledTaskSessionBinding(invalidated)
	if partiallyRebound.SessionBindingState != TaskSessionBindingStateRebindRequired ||
		!reflect.DeepEqual(partiallyRebound.SessionBindingIssues, []string{TaskSessionBindingIssueExecution}) {
		t.Fatalf("只修复投递后不应恢复: %+v", partiallyRebound)
	}

	partiallyRebound.SessionTarget.BoundSessionKey = "agent:agent-1:websocket:dm:new-execution"
	rebound := NormalizeScheduledTaskSessionBinding(partiallyRebound)
	if rebound.SessionBindingState != TaskSessionBindingStateReady ||
		len(rebound.InvalidatedSessionKeys) != 0 || len(rebound.SessionBindingIssues) != 0 {
		t.Fatalf("全部重绑后应恢复 ready: %+v", rebound)
	}
}

func TestInvalidateScheduledTaskSessionsDoesNotTreatSourceAsLiveBinding(t *testing.T) {
	deletedKey := "agent:agent-1:weixin-personal:dm:account-a:peer-a"
	task := ScheduledTask{
		Enabled:       true,
		SessionTarget: SessionTarget{Kind: SessionTargetIsolated},
		Delivery:      DeliveryTarget{Mode: DeliveryModeNone},
		Source:        Source{SessionKey: deletedKey},
	}

	result, changed := InvalidateScheduledTaskSessions(task, []string{deletedKey})
	if changed || !result.Enabled || result.SessionBindingState != TaskSessionBindingStateReady {
		t.Fatalf("来源 Session 仅作 provenance，不应暂停任务: %+v", result)
	}
}
