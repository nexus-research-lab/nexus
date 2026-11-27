// agent_handler_test.go 覆盖 Agent Handler 失败处理、通信错误与权限模式测试。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
	sessionpkg "github.com/nexus-research-lab/nexus/internal/service/session"
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
		name   string
		err    error
		status int
		code   string
		effect protocol.FailureEffect
	}{
		{
			name:   "target already absent",
			err:    agentpkg.ErrAgentNotFound,
			status: http.StatusNotFound,
			code:   "agent.not_found",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "unclassified persistence outcome",
			err:    errors.New("database unavailable"),
			status: http.StatusInternalServerError,
			code:   "agent.deletion_outcome_unknown",
			effect: protocol.FailureEffectUnknown,
		},
		{
			name:   "concurrent version conflict",
			err:    agentpkg.ErrRuntimeVersionConflict,
			status: http.StatusConflict,
			code:   "agent.deletion_conflict",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "committed cleanup failure",
			err:    new(agentpkg.DeletionReconcileError),
			status: http.StatusInternalServerError,
			code:   "agent.deletion_cleanup_incomplete",
			effect: protocol.FailureEffectCommitted,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, spec := agentDeleteFailure(test.err)
			if status != test.status || spec.Code != test.code || spec.Effect != test.effect {
				t.Fatalf("agentDeleteFailure() = status %d, code %q, effect %q", status, spec.Code, spec.Effect)
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
}

func TestAgentUpdateFailurePreservesCommitEvidence(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
		effect protocol.FailureEffect
	}{
		{
			name:   "validation rejected before write",
			err:    agentpkg.ErrMainAgentNameImmutable,
			status: http.StatusBadRequest,
			code:   "agent.update_rejected",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "concurrent version conflict",
			err:    agentpkg.ErrRuntimeVersionConflict,
			status: http.StatusConflict,
			code:   "agent.update_conflict",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "post commit projection failure",
			err:    new(agentpkg.UpdateReconcileError),
			status: http.StatusInternalServerError,
			code:   "agent.update_projection_incomplete",
			effect: protocol.FailureEffectCommitted,
		},
		{
			name:   "unclassified repository result",
			err:    errors.New("database connection lost"),
			status: http.StatusInternalServerError,
			code:   "agent.update_outcome_unknown",
			effect: protocol.FailureEffectUnknown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, spec := agentUpdateFailure(test.err)
			if status != test.status || spec.Code != test.code || spec.Effect != test.effect {
				t.Fatalf("agentUpdateFailure() = status %d, code %q, effect %q", status, spec.Code, spec.Effect)
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
}

func TestAgentCreateFailureUsesDurableRequestEvidence(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
		effect protocol.FailureEffect
	}{
		{
			name:   "same request different intent",
			err:    agentpkg.ErrAgentCreationRequestConflict,
			status: http.StatusConflict,
			code:   "agent.creation_request_conflict",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "durable request still running",
			err:    agentpkg.ErrAgentCreationPending,
			status: http.StatusConflict,
			code:   "agent.creation_in_progress",
			effect: protocol.FailureEffectAccepted,
		},
		{
			name:   "deleted result never recreated",
			err:    agentpkg.ErrAgentCreationResultDeleted,
			status: http.StatusGone,
			code:   "agent.creation_result_deleted",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "committed projection failure",
			err:    &agentpkg.CreationReconcileError{},
			status: http.StatusInternalServerError,
			code:   "agent.creation_projection_incomplete",
			effect: protocol.FailureEffectCommitted,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, spec := agentCreateFailure(test.err)
			if status != test.status || spec.Code != test.code || spec.Effect != test.effect {
				t.Fatalf("agentCreateFailure() = status %d, code %q, effect %q", status, spec.Code, spec.Effect)
			}
		})
	}
}

// TestRuntimeSettingsValidationIsNotApplied 校验阶段拒绝不能被前端当作提交结果未知。
func TestRuntimeSettingsValidationIsNotApplied(t *testing.T) {
	handler := &Handlers{api: handlershared.NewAPI(nil)}
	request := httptest.NewRequest(http.MethodPut, "/sessions/example/runtime-settings", nil)
	response := httptest.NewRecorder()
	handler.writeSessionRuntimeSettingsError(response, request, sessionpkg.ErrInvalidRuntimeSettings)
	var payload struct {
		Data struct {
			Failure protocol.FailureCore `json:"failure"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadRequest || payload.Data.Failure.Effect != protocol.FailureEffectNotApplied {
		t.Fatalf("status=%d failure=%+v", response.Code, payload.Data.Failure)
	}
}

func TestCommunicationFailureUsesTypedCommitEvidence(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		effect protocol.FailureEffect
	}{
		{name: "typed input rejection", err: new(communicationsvc.InputError), status: http.StatusBadRequest, effect: protocol.FailureEffectNotApplied},
		{name: "target absent", err: agentpkg.ErrAgentContactNotFound, status: http.StatusNotFound, effect: protocol.FailureEffectNotApplied},
		{name: "unclassified send result", err: errors.New("storage result unavailable"), status: http.StatusInternalServerError, effect: protocol.FailureEffectUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := &Handlers{api: handlershared.NewAPI(nil)}
			request := httptest.NewRequest(http.MethodPost, "/communications/messages", nil)
			response := httptest.NewRecorder()
			handler.writeCommunicationFailure(
				response,
				request,
				"communication.send_failed",
				"消息没有发送完成",
				test.err,
			)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			var payload struct {
				Data struct {
					Failure protocol.FailureCore `json:"failure"`
				} `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if payload.Data.Failure.Effect != test.effect {
				t.Fatalf("effect = %q, want %q", payload.Data.Failure.Effect, test.effect)
			}
		})
	}
}

type permissionModeRecorder struct {
	ids   []string
	modes []sdkpermission.Mode
	err   error
}

func (r *permissionModeRecorder) SetPermissionModeForAgent(_ context.Context, id string, mode sdkpermission.Mode) error {
	r.ids = append(r.ids, id)
	r.modes = append(r.modes, mode)
	return r.err
}

func TestPermissionModeUpdateContinuesAfterDomainFailure(t *testing.T) {
	dmError, roomError := errors.New("DM cleanup pending"), errors.New("Room cleanup pending")
	dm, room := &permissionModeRecorder{err: dmError}, &permissionModeRecorder{err: roomError}
	err := syncAgentPermissionMode(context.Background(), "agent-a", sdkpermission.ModeDefault, dm, room)
	if !errors.Is(err, dmError) || !errors.Is(err, roomError) {
		t.Fatalf("domain failure was lost: %v", err)
	}
	for _, setter := range []*permissionModeRecorder{dm, room} {
		if len(setter.ids) != 1 || setter.ids[0] != "agent-a" || setter.modes[0] != sdkpermission.ModeDefault {
			t.Fatalf("domain missed mode change: %+v", setter)
		}
	}
}
