// INPUT: 删除 Session 后的任务引用、后续部分/完整重绑与手动执行请求。
// OUTPUT: 任务保留、持久停用、失效范围收敛和完全重绑后恢复的端到端证明。
// POS: Automation service 会话绑定生命周期回归测试。
package automation

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestDeletedSessionDisablesTaskUntilEveryBindingIsReassigned(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	deletedSessionKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelWebSocket,
		"dm",
		"deleted-session",
		"",
	)
	input := automationConfigurationTaskInput("session rebind")
	input.SessionTarget = automationdomain.SessionTarget{
		Kind:            automationdomain.SessionTargetBound,
		BoundSessionKey: deletedSessionKey,
		WakeMode:        automationdomain.WakeModeNow,
	}
	input.Delivery = automationdomain.DeliveryTarget{
		Mode:       automationdomain.DeliveryModeLast,
		SessionKey: deletedSessionKey,
	}
	created, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err = service.InvalidateTasksForDeletedSessions(
		context.Background(),
		created.OwnerUserID,
		[]string{deletedSessionKey},
	); err != nil {
		t.Fatalf("InvalidateTasksForDeletedSessions: %v", err)
	}
	invalidated, err := service.GetTask(context.Background(), created.JobID)
	if err != nil || invalidated == nil {
		t.Fatalf("任务应被保留: task=%+v err=%v", invalidated, err)
	}
	if invalidated.Enabled || invalidated.SessionBindingState != automationdomain.TaskSessionBindingStateRebindRequired ||
		!reflect.DeepEqual(invalidated.SessionBindingIssues, []string{
			automationdomain.TaskSessionBindingIssueExecution,
			automationdomain.TaskSessionBindingIssueDelivery,
		}) {
		t.Fatalf("任务未进入完整待重绑状态: %+v", invalidated)
	}
	if _, err = service.RunTaskNow(context.Background(), created.JobID); !errors.Is(
		err,
		automationdomain.ErrTaskSessionRebindRequired,
	) {
		t.Fatalf("待重绑任务应拒绝运行: %v", err)
	}

	newDeliveryKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelWebSocket,
		"dm",
		"new-delivery",
		"",
	)
	updatedDelivery := automationdomain.DeliveryTarget{
		Mode:       automationdomain.DeliveryModeLast,
		SessionKey: newDeliveryKey,
	}
	partiallyRebound, err := service.UpdateTask(
		context.Background(),
		created.JobID,
		automationdomain.UpdateJobInput{Delivery: &updatedDelivery},
	)
	if err != nil {
		t.Fatalf("更新投递会话: %v", err)
	}
	if partiallyRebound.SessionBindingState != automationdomain.TaskSessionBindingStateRebindRequired ||
		!reflect.DeepEqual(partiallyRebound.SessionBindingIssues, []string{automationdomain.TaskSessionBindingIssueExecution}) {
		t.Fatalf("部分重绑不应恢复任务: %+v", partiallyRebound)
	}
	enabled := true
	if _, err = service.UpdateTask(
		context.Background(),
		created.JobID,
		automationdomain.UpdateJobInput{Enabled: &enabled},
	); !errors.Is(err, automationdomain.ErrTaskSessionRebindRequired) {
		t.Fatalf("仍有失效执行会话时应拒绝启用: %v", err)
	}

	newExecutionKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelWebSocket,
		"dm",
		"new-execution",
		"",
	)
	updatedExecution := automationdomain.SessionTarget{
		Kind:            automationdomain.SessionTargetBound,
		BoundSessionKey: newExecutionKey,
		WakeMode:        automationdomain.WakeModeNow,
	}
	rebound, err := service.UpdateTask(
		context.Background(),
		created.JobID,
		automationdomain.UpdateJobInput{
			Enabled:       &enabled,
			SessionTarget: &updatedExecution,
		},
	)
	if err != nil {
		t.Fatalf("完整重绑并启用: %v", err)
	}
	if !rebound.Enabled || rebound.SessionBindingState != automationdomain.TaskSessionBindingStateReady ||
		len(rebound.SessionBindingIssues) != 0 {
		t.Fatalf("完整重绑后任务未恢复: %+v", rebound)
	}

	events, err := service.ListTaskEvents(context.Background(), created.JobID, 20)
	if err != nil {
		t.Fatalf("ListTaskEvents: %v", err)
	}
	foundInvalidation := false
	for _, event := range events {
		if event.Action == automationdomain.TaskEventActionSessionBindingInvalidated {
			foundInvalidation = true
		}
	}
	if !foundInvalidation {
		t.Fatalf("未记录 Session 绑定失效审计: %+v", events)
	}
}

func TestDeletedSessionInvalidationStopsFrozenRunDelivery(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	deletedSessionKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelWebSocket,
		"dm",
		"deleted-delivery",
		"",
	)
	input := automationConfigurationTaskInput("frozen delivery")
	input.Delivery = automationdomain.DeliveryTarget{
		Mode:       automationdomain.DeliveryModeLast,
		SessionKey: deletedSessionKey,
	}
	created, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err = service.InvalidateTasksForDeletedSessions(
		context.Background(),
		created.OwnerUserID,
		[]string{deletedSessionKey},
	); err != nil {
		t.Fatalf("InvalidateTasksForDeletedSessions: %v", err)
	}

	result := service.deliverJobObservationToTarget(
		context.Background(),
		*created,
		created.Delivery,
		"",
		automationexec.ExecutionObservation{
			RunID:      "run-before-delete",
			Status:     automationdomain.RunStatusSucceeded,
			ResultText: "不应投递到已删除会话",
		},
	)
	if result.Status != automationdomain.DeliveryStatusFailed || result.Error == nil ||
		!strings.Contains(*result.Error, automationdomain.ErrTaskSessionRebindRequired.Error()) {
		t.Fatalf("删除会话后的在途结果应停止投递: %+v", result)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("已删除会话仍收到投递: %+v", calls)
	}
}
