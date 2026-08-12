package websocket

import (
	"errors"
	"fmt"
	"testing"

	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

func TestAppServerGoalRPCErrorClassification(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   int64
		wantReason string
	}{
		{
			name:       "goal conflict",
			err:        goalsvc.ErrGoalConflict,
			wantCode:   goalappserver.AppServerRPCConflictCode,
			wantReason: goalappserver.AppServerRPCReasonConflict,
		},
		{
			name:       "version stale",
			err:        fmt.Errorf("concurrent update: %w", goalsvc.ErrGoalVersionStale),
			wantCode:   goalappserver.AppServerRPCConflictCode,
			wantReason: goalappserver.AppServerRPCReasonVersionStale,
		},
		{
			name:       "objective revision stale",
			err:        goalsvc.ErrGoalRevisionStale,
			wantCode:   goalappserver.AppServerRPCConflictCode,
			wantReason: goalappserver.AppServerRPCReasonRevisionStale,
		},
		{
			name:       "execution binding conflict",
			err:        fmt.Errorf("binding read: %w", goalsvc.ErrGoalExecutionBindingConflict),
			wantCode:   goalappserver.AppServerRPCConflictCode,
			wantReason: goalappserver.AppServerRPCReasonExecutionBindingConflict,
		},
		{
			name:     "invalid state",
			err:      goalsvc.ErrGoalInvalidState,
			wantCode: goalappserver.AppServerRPCInvalidRequestCode,
		},
		{
			name:     "unknown internal error",
			err:      errors.New("repository unavailable"),
			wantCode: goalappserver.AppServerRPCInternalErrorCode,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := appServerGoalRPCError(test.err)
			if result.Code != test.wantCode {
				t.Fatalf("code = %d, want %d", result.Code, test.wantCode)
			}
			if test.wantReason == "" {
				if result.Data != nil {
					t.Fatalf("data = %#v, want nil", result.Data)
				}
				return
			}
			data, ok := result.Data.(goalappserver.AppServerRPCErrorData)
			if !ok || data.ReasonCode != test.wantReason {
				t.Fatalf("data = %#v, want reason_code %q", result.Data, test.wantReason)
			}
		})
	}
}
