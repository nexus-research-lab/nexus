// agent_handler_test.go 覆盖 Agent Handler 失败处理、通信错误与权限模式测试。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	sessionpkg "github.com/nexus-research-lab/nexus/internal/service/session"
)

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
