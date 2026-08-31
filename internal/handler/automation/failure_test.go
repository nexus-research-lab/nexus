// INPUT: Automation handler 操作与 service 返回的领域错误。
// OUTPUT: HTTP 兼容状态及有证据的数据影响分类回归。
// POS: Automation FailureCore 映射的单元测试，防止把未知副作用误报为未应用。
package automation

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
)

func TestBindAutomationJSONReportsKnownPreServiceFailure(t *testing.T) {
	t.Parallel()

	handler := &Handlers{api: handlershared.NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)))}
	request := httptest.NewRequest(http.MethodPost, "/api/automation/tasks", bytes.NewBufferString("{"))
	request.Header.Set("X-Request-ID", "transport-json-invalid")
	recorder := httptest.NewRecorder()

	handlershared.RequestContextMiddleware(slog.New(slog.NewTextHandler(io.Discard, nil)))(
		http.HandlerFunc(func(writer http.ResponseWriter, scopedRequest *http.Request) {
			if handler.bindAutomationJSON(writer, scopedRequest, &scheduledTaskCreatePayload{}, false) {
				t.Fatal("malformed JSON must stop before the service")
			}
		}),
	).ServeHTTP(recorder, request)
	var envelope struct {
		Data struct {
			Failure protocol.FailureCore `json:"failure"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode failure response: %v", err)
	}
	if recorder.Code != http.StatusBadRequest ||
		envelope.Data.Failure.Code != "automation.request_body_invalid" ||
		envelope.Data.Failure.Effect != protocol.FailureEffectNotApplied ||
		envelope.Data.Failure.TransportRequestID != "transport-json-invalid" {
		t.Fatalf("unexpected failure response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMapAutomationFailurePreservesEffectEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		operation automationFailureOperation
		err       error
		status    int
		code      string
		effect    protocol.FailureEffect
	}{
		{
			name:      "task list read has no mutation effect",
			operation: automationFailureListTasks,
			err:       errors.New("database unavailable"),
			status:    http.StatusInternalServerError,
			code:      "automation.task_list_unavailable",
			effect:    protocol.FailureEffectNotApplicable,
		},
		{
			name:      "permission list read has no mutation effect",
			operation: automationFailureListPermissions,
			err:       errors.New("database unavailable"),
			status:    http.StatusInternalServerError,
			code:      "automation.permission_list_unavailable",
			effect:    protocol.FailureEffectNotApplicable,
		},
		{
			name:      "create request identity conflict is rejected before a second create",
			operation: automationFailureReplayCreateTask,
			err:       automationdomain.ErrCreateRequestConflict,
			status:    http.StatusConflict,
			code:      "automation.task_create_conflict",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "replayable create validation after mutable preflight is unknown",
			operation: automationFailureReplayCreateTask,
			err:       errors.New("任务名称不能为空"),
			status:    http.StatusBadRequest,
			code:      "automation.task_create_invalid",
			effect:    protocol.FailureEffectUnknown,
		},
		{
			name:      "committed create whose task was deleted is explicit",
			operation: automationFailureReplayCreateTask,
			err:       automationdomain.ErrCreateRequestResultGone,
			status:    http.StatusGone,
			code:      "automation.task_create_result_gone",
			effect:    protocol.FailureEffectCommitted,
		},
		{
			name:      "create validation is known pre-write",
			operation: automationFailureCreateTask,
			err:       errors.New("任务名称不能为空"),
			status:    http.StatusBadRequest,
			code:      "automation.task_create_invalid",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "create infrastructure failure is unknown",
			operation: automationFailureCreateTask,
			err:       errors.New("database unavailable"),
			status:    http.StatusInternalServerError,
			code:      "automation.task_create_failed",
			effect:    protocol.FailureEffectUnknown,
		},
		{
			name:      "status version conflict is known pre-write",
			operation: automationFailureUpdateStatus,
			err:       automationdomain.ErrConfigurationVersionConflict,
			status:    http.StatusConflict,
			code:      "automation.configuration_conflict",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "task update is fenced after deletion claim",
			operation: automationFailureUpdateTask,
			err:       automationdomain.ErrTaskDeleting,
			status:    http.StatusConflict,
			code:      "automation.task_deletion_in_progress",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "permission stale decision is known pre-write",
			operation: automationFailureResolvePermission,
			err:       automationdomain.ErrPermissionRequestStale,
			status:    http.StatusConflict,
			code:      "automation.permission_conflict",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "permission decision is fenced after deletion claim",
			operation: automationFailureResolvePermission,
			err:       automationdomain.ErrTaskDeleting,
			status:    http.StatusConflict,
			code:      "automation.task_deletion_in_progress",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "post-commit permission stale cannot be reported as not applied",
			operation: automationFailureResolvePermission,
			err:       automationsvc.MarkPermissionDecisionCommitted(automationdomain.ErrPermissionRequestStale),
			status:    http.StatusConflict,
			code:      "automation.permission_decision_committed",
			effect:    protocol.FailureEffectCommitted,
		},
		{
			name:      "permission persistence failure is unknown",
			operation: automationFailureResolvePermission,
			err:       errors.New("database unavailable"),
			status:    http.StatusInternalServerError,
			code:      "automation.permission_decision_failed",
			effect:    protocol.FailureEffectUnknown,
		},
		{
			name:      "resume generic failure keeps conflict status but unknown effect",
			operation: automationFailureResumePermission,
			err:       errors.New("resume dispatch failed"),
			status:    http.StatusConflict,
			code:      "automation.permission_resume_failed",
			effect:    protocol.FailureEffectUnknown,
		},
		{
			name:      "permission resume is fenced after deletion claim",
			operation: automationFailureResumePermission,
			err:       automationdomain.ErrTaskDeleting,
			status:    http.StatusConflict,
			code:      "automation.task_deletion_in_progress",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "resume stale may be reported after ready state write",
			operation: automationFailureResumePermission,
			err:       automationdomain.ErrPermissionRequestStale,
			status:    http.StatusConflict,
			code:      "automation.permission_resume_conflict",
			effect:    protocol.FailureEffectUnknown,
		},
		{
			name:      "resume readable text does not prove mutation phase",
			operation: automationFailureResumePermission,
			err:       errors.New("恢复状态不一致"),
			status:    http.StatusBadRequest,
			code:      "automation.permission_resume_invalid",
			effect:    protocol.FailureEffectUnknown,
		},
		{
			name:      "run version conflict fences stale task configuration",
			operation: automationFailureRunTask,
			err:       automationdomain.ErrConfigurationVersionConflict,
			status:    http.StatusConflict,
			code:      "automation.run_configuration_conflict",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "manual run is fenced after deletion claim",
			operation: automationFailureRunTask,
			err:       automationdomain.ErrTaskDeleting,
			status:    http.StatusConflict,
			code:      "automation.task_deletion_in_progress",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "manual run request identity cannot be reused for another intent",
			operation: automationFailureRunTask,
			err:       automationdomain.ErrRuntimeCommandConflict,
			status:    http.StatusConflict,
			code:      "automation.run_request_conflict",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "run missing task is known pre-write",
			operation: automationFailureRunTask,
			err:       automationdomain.ErrJobNotFound,
			status:    http.StatusNotFound,
			code:      "automation.run_task_not_found",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "run dispatch failure may follow durable run creation",
			operation: automationFailureRunTask,
			err:       errors.New("dispatch failed"),
			status:    http.StatusInternalServerError,
			code:      "automation.run_task_failed",
			effect:    protocol.FailureEffectUnknown,
		},
		{
			name:      "recover exact run changed before atomic commit",
			operation: automationFailureRecoverTask,
			err:       automationdomain.ErrRunRecoveryConflict,
			status:    http.StatusConflict,
			code:      "automation.recover_task_conflict",
			effect:    protocol.FailureEffectUnknown,
		},
		{
			name:      "recover client error still has unknown effect",
			operation: automationFailureRecoverTask,
			err:       errors.New("run_id 不能为空"),
			status:    http.StatusBadRequest,
			code:      "automation.recover_task_conflict",
			effect:    protocol.FailureEffectUnknown,
		},
		{
			name:      "delivery version conflict fences stale route configuration",
			operation: automationFailureRetryDelivery,
			err:       automationdomain.ErrConfigurationVersionConflict,
			status:    http.StatusConflict,
			code:      "automation.delivery_configuration_conflict",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "delivery retry is fenced after deletion claim",
			operation: automationFailureRetryDelivery,
			err:       automationdomain.ErrTaskDeleting,
			status:    http.StatusConflict,
			code:      "automation.task_deletion_in_progress",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "delivery missing run stays unknown across preflight and post-effect reload",
			operation: automationFailureRetryDelivery,
			err:       automationdomain.ErrRunNotFound,
			status:    http.StatusNotFound,
			code:      "automation.delivery_retry_not_found",
			effect:    protocol.FailureEffectUnknown,
		},
		{
			name:      "delivery retry generic failure may follow external delivery",
			operation: automationFailureRetryDelivery,
			err:       errors.New("delivery receipt persistence failed"),
			status:    http.StatusInternalServerError,
			code:      "automation.delivery_retry_failed",
			effect:    protocol.FailureEffectUnknown,
		},
		{
			name:      "delivery retry claim conflict is known not applied",
			operation: automationFailureRetryDelivery,
			err:       automationdomain.ErrDeliveryRetryConflict,
			status:    http.StatusConflict,
			code:      "automation.delivery_retry_conflict",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "unverified prior delivery requires user review",
			operation: automationFailureRetryDelivery,
			err:       automationdomain.ErrDeliveryRetryUnverified,
			status:    http.StatusConflict,
			code:      "automation.delivery_retry_unverified",
			effect:    protocol.FailureEffectUnknown,
		},
		{
			name:      "delivery completion conflict follows external effect",
			operation: automationFailureRetryDelivery,
			err:       automationdomain.ErrDeliveryRetryCompletionUnconfirmed,
			status:    http.StatusConflict,
			code:      "automation.delivery_retry_completion_unconfirmed",
			effect:    protocol.FailureEffectUnknown,
		},
		{
			name:      "delete version conflict before preparation is not applied",
			operation: automationFailureDeleteTask,
			err:       automationdomain.ErrConfigurationVersionConflict,
			status:    http.StatusConflict,
			code:      "automation.delete_configuration_conflict",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "durable delete claim reports accepted cleanup",
			operation: automationFailureDeleteTask,
			err:       automationsvc.MarkTaskDeletionPrepared(automationdomain.ErrConfigurationVersionConflict),
			status:    http.StatusConflict,
			code:      "automation.delete_task_in_progress",
			effect:    protocol.FailureEffectAccepted,
		},
		{
			name:      "delete missing task is known pre-write",
			operation: automationFailureDeleteTask,
			err:       automationdomain.ErrJobNotFound,
			status:    http.StatusNotFound,
			code:      "automation.delete_task_not_found",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "unversioned delete failure is always unknown",
			operation: automationFailureDeleteTask,
			err:       errors.New("delete persistence failed"),
			status:    http.StatusInternalServerError,
			code:      "automation.delete_task_failed",
			effect:    protocol.FailureEffectUnknown,
		},
		{
			name:      "deletion stop confirmation requires an exact version",
			operation: automationFailureConfirmDeletionStopped,
			err:       errExpectedConfigurationVersionRequired,
			status:    http.StatusBadRequest,
			code:      "automation.deletion_confirmation_invalid",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "deletion stop confirmation rejects a stale review snapshot",
			operation: automationFailureConfirmDeletionStopped,
			err:       automationdomain.ErrTaskDeletionReviewConflict,
			status:    http.StatusConflict,
			code:      "automation.deletion_confirmation_not_required",
			effect:    protocol.FailureEffectNotApplied,
		},
		{
			name:      "accepted deletion stop confirmation never promises replay",
			operation: automationFailureConfirmDeletionStopped,
			err:       automationsvc.MarkTaskDeletionPrepared(automationdomain.ErrConfigurationVersionConflict),
			status:    http.StatusConflict,
			code:      "automation.deletion_confirmation_in_progress",
			effect:    protocol.FailureEffectAccepted,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			failure := mapAutomationFailure(test.operation, test.err)
			if failure.status != test.status ||
				failure.spec.Code != test.code ||
				failure.spec.Effect != test.effect {
				t.Fatalf(
					"映射不正确: status=%d code=%q effect=%q spec=%+v",
					failure.status,
					failure.spec.Code,
					failure.spec.Effect,
					failure.spec,
				)
			}
			if failure.spec.Cause != test.err {
				t.Fatalf("诊断 cause 未保留: got=%v want=%v", failure.spec.Cause, test.err)
			}
		})
	}
}

func TestDeleteTaskPreparedFailureExplainsDurableCleanup(t *testing.T) {
	t.Parallel()

	failure := mapAutomationFailure(
		automationFailureDeleteTask,
		automationsvc.MarkTaskDeletionPrepared(errors.New("cleanup unavailable")),
	)
	if failure.spec.Effect != protocol.FailureEffectAccepted {
		t.Fatalf("unexpected durable delete failure: %+v", failure.spec)
	}
	for _, phrase := range []string{"已停止接受新运行", "不会撤回", "不会自动重放", "稍后刷新"} {
		if !strings.Contains(failure.spec.Detail, phrase) {
			t.Fatalf("delete failure detail %q omitted %q", failure.spec.Detail, phrase)
		}
	}
}

func TestDeleteTaskReviewRequiredFailureDoesNotPromiseAutomaticRecovery(t *testing.T) {
	t.Parallel()

	failure := mapAutomationFailure(
		automationFailureDeleteTask,
		automationsvc.MarkTaskDeletionPrepared(automationsvc.ErrExecutionAttemptOwnershipUnconfirmed),
	)
	if failure.spec.Code != "automation.delete_task_review_required" ||
		failure.spec.Effect != protocol.FailureEffectAccepted {
		t.Fatalf("unexpected review-required failure: %+v", failure.spec)
	}
	for _, phrase := range []string{"暂时无法确认", "尚未删除", "管理员处理", "不会自动重放"} {
		if !strings.Contains(failure.spec.Detail, phrase) {
			t.Fatalf("review-required detail %q omitted %q", failure.spec.Detail, phrase)
		}
	}
}
