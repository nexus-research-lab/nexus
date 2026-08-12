package automation

import (
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

func TestScheduledTaskReferencedSessionKeysUsesExecutionAndDeliveryDependencies(t *testing.T) {
	const (
		boundKey    = "agent:agent-a:ws:dm:bound"
		deliveryKey = "agent:agent-a:weixin-personal:dm:acct:account-a:contact-a"
		legacyKey   = "agent:agent-a:tg:dm:legacy"
		sourceKey   = "agent:agent-a:fs:dm:source"
	)
	task := automationdomain.ScheduledTask{
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: boundKey,
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:       automationdomain.DeliveryModeLast,
			SessionKey: deliveryKey,
			To:         legacyKey,
		},
		Source: automationdomain.Source{SessionKey: sourceKey},
	}
	keys := map[string]struct{}{
		boundKey:    {},
		deliveryKey: {},
		legacyKey:   {},
		sourceKey:   {},
	}

	result := scheduledTaskReferencedSessionKeys(task, keys)
	for _, dependency := range []string{boundKey, deliveryKey, legacyKey} {
		if _, exists := result[dependency]; !exists {
			t.Fatalf("缺少任务 Session 依赖 %q: %+v", dependency, result)
		}
	}
	if _, exists := result[sourceKey]; exists {
		t.Fatalf("创建来源只属于 provenance，不应阻止历史 Session 删除: %+v", result)
	}
}

func TestScheduledTaskReferencedSessionKeysCountsOneTaskOncePerSession(t *testing.T) {
	const sessionKey = "agent:agent-a:tg:dm:chat-a"
	task := automationdomain.ScheduledTask{
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: sessionKey,
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:       automationdomain.DeliveryModeLast,
			SessionKey: sessionKey,
			To:         sessionKey,
		},
	}
	result := scheduledTaskReferencedSessionKeys(
		task,
		map[string]struct{}{sessionKey: {}},
	)
	if len(result) != 1 {
		t.Fatalf("同一任务重复引用同一 Session 时应只计一次: %+v", result)
	}
}
