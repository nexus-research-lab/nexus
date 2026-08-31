// INPUT: Agent 创建/更新/删除的请求校验、提交前失败和提交后同步失败证据。
// OUTPUT: 稳定 HTTP 状态、FailureCore code/effect 和恢复动作断言。
// POS: Agent Handler 失败映射回归；请求体校验不得进入 Agent 服务。
package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
)

func TestHandleCreateAgentRejectsMalformedJSONBeforeSideEffects(t *testing.T) {
	handler := &Handlers{api: handlershared.NewAPI(nil)}
	request := httptest.NewRequest(http.MethodPost, "/agents", strings.NewReader(`{"name":`))
	response := httptest.NewRecorder()

	handler.HandleCreateAgent(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	var payload struct {
		Data struct {
			Failure protocol.FailureCore `json:"failure"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.Failure.Code != "agent.creation_request_invalid" ||
		payload.Data.Failure.Effect != protocol.FailureEffectNotApplied {
		t.Fatalf("failure = %#v", payload.Data.Failure)
	}
}

func TestAgentDeleteFailureUsesDomainCommitEvidence(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		status   int
		code     string
		effect   protocol.FailureEffect
		recovery string
	}{
		{
			name:     "target already absent",
			err:      agentpkg.ErrAgentNotFound,
			status:   http.StatusNotFound,
			code:     "agent.not_found",
			effect:   protocol.FailureEffectNotApplied,
			recovery: "agent.refresh_directory",
		},
		{
			name:     "unclassified persistence outcome",
			err:      errors.New("database unavailable"),
			status:   http.StatusInternalServerError,
			code:     "agent.deletion_outcome_unknown",
			effect:   protocol.FailureEffectUnknown,
			recovery: "agent.refresh_directory",
		},
		{
			name:     "concurrent version conflict",
			err:      agentpkg.ErrRuntimeVersionConflict,
			status:   http.StatusConflict,
			code:     "agent.deletion_conflict",
			effect:   protocol.FailureEffectNotApplied,
			recovery: "agent.refresh_directory",
		},
		{
			name:     "committed cleanup failure",
			err:      new(agentpkg.DeletionReconcileError),
			status:   http.StatusInternalServerError,
			code:     "agent.deletion_cleanup_incomplete",
			effect:   protocol.FailureEffectCommitted,
			recovery: "agent.refresh_directory",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, spec := agentDeleteFailure(test.err)
			if status != test.status || spec.Code != test.code || spec.Effect != test.effect {
				t.Fatalf("agentDeleteFailure() = status %d, code %q, effect %q", status, spec.Code, spec.Effect)
			}
			if spec.Resolution == nil || spec.Resolution.Action != test.recovery {
				t.Fatalf("resolution = %#v, want action %q", spec.Resolution, test.recovery)
			}
		})
	}
}

func TestAgentDeleteFailureUsesStableControlPlaneSentinel(t *testing.T) {
	status, spec := agentDeleteFailure(agentpkg.ErrAgentDeletionNotAllowed)
	if status != http.StatusBadRequest || spec.Code != "agent.deletion_not_allowed" ||
		spec.Effect != protocol.FailureEffectNotApplied {
		t.Fatalf(
			"agentDeleteFailure() = status %d, code %q, effect %q",
			status,
			spec.Code,
			spec.Effect,
		)
	}
	if spec.Resolution != nil {
		t.Fatalf("control-plane rejection must not advertise retry: %#v", spec.Resolution)
	}
}

func TestAgentUpdateFailurePreservesCommitEvidence(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		status   int
		code     string
		effect   protocol.FailureEffect
		recovery string
	}{
		{
			name:     "validation rejected before write",
			err:      agentpkg.ErrMainAgentNameImmutable,
			status:   http.StatusBadRequest,
			code:     "agent.update_rejected",
			effect:   protocol.FailureEffectNotApplied,
			recovery: "agent.review_settings",
		},
		{
			name:     "concurrent version conflict",
			err:      agentpkg.ErrRuntimeVersionConflict,
			status:   http.StatusConflict,
			code:     "agent.update_conflict",
			effect:   protocol.FailureEffectNotApplied,
			recovery: "agent.refresh_directory",
		},
		{
			name:     "post commit projection failure",
			err:      new(agentpkg.UpdateReconcileError),
			status:   http.StatusInternalServerError,
			code:     "agent.update_projection_incomplete",
			effect:   protocol.FailureEffectCommitted,
			recovery: "agent.refresh_directory",
		},
		{
			name:     "unclassified repository result",
			err:      errors.New("database connection lost"),
			status:   http.StatusInternalServerError,
			code:     "agent.update_outcome_unknown",
			effect:   protocol.FailureEffectUnknown,
			recovery: "agent.refresh_directory",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, spec := agentUpdateFailure(test.err)
			if status != test.status || spec.Code != test.code || spec.Effect != test.effect {
				t.Fatalf("agentUpdateFailure() = status %d, code %q, effect %q", status, spec.Code, spec.Effect)
			}
			if spec.Resolution == nil || spec.Resolution.Action != test.recovery {
				t.Fatalf("resolution = %#v, want action %q", spec.Resolution, test.recovery)
			}
		})
	}
}

func TestAgentPermissionModeSyncFailureIsCommitted(t *testing.T) {
	spec := agentPermissionModeSyncFailure(errors.New("runtime sync failed"))
	if spec.Code != "agent.permission_mode_sync_incomplete" ||
		spec.Effect != protocol.FailureEffectCommitted {
		t.Fatalf("permission sync failure = code %q, effect %q", spec.Code, spec.Effect)
	}
	if spec.Resolution == nil || spec.Resolution.Action != "agent.refresh_directory" {
		t.Fatalf("resolution = %#v", spec.Resolution)
	}
}

func TestAgentCreateFailureUsesDurableRequestEvidence(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		status   int
		code     string
		effect   protocol.FailureEffect
		recovery string
	}{
		{
			name:     "same request different intent",
			err:      agentpkg.ErrAgentCreationRequestConflict,
			status:   http.StatusConflict,
			code:     "agent.creation_request_conflict",
			effect:   protocol.FailureEffectNotApplied,
			recovery: "agent.check_creation_request",
		},
		{
			name:     "durable request still running",
			err:      agentpkg.ErrAgentCreationPending,
			status:   http.StatusConflict,
			code:     "agent.creation_in_progress",
			effect:   protocol.FailureEffectAccepted,
			recovery: "agent.check_creation_request",
		},
		{
			name:     "deleted result never recreated",
			err:      agentpkg.ErrAgentCreationResultDeleted,
			status:   http.StatusGone,
			code:     "agent.creation_result_deleted",
			effect:   protocol.FailureEffectNotApplied,
			recovery: "agent.start_new_creation",
		},
		{
			name:     "committed projection failure",
			err:      &agentpkg.CreationReconcileError{},
			status:   http.StatusInternalServerError,
			code:     "agent.creation_projection_incomplete",
			effect:   protocol.FailureEffectCommitted,
			recovery: "agent.check_creation_request",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, spec := agentCreateFailure(test.err)
			if status != test.status || spec.Code != test.code || spec.Effect != test.effect {
				t.Fatalf("agentCreateFailure() = status %d, code %q, effect %q", status, spec.Code, spec.Effect)
			}
			if spec.Resolution == nil || spec.Resolution.Action != test.recovery {
				t.Fatalf("resolution = %#v, want %q", spec.Resolution, test.recovery)
			}
			if errors.Is(test.err, agentpkg.ErrAgentCreationPending) && spec.RetryAfter != 0 {
				t.Fatalf("pending create must be reconciled explicitly, RetryAfter = %v", spec.RetryAfter)
			}
		})
	}
}
