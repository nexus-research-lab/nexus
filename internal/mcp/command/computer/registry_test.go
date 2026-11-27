package computer

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
)

func TestKeyInputNormalizesCommonModelSpellings(t *testing.T) {
	input := actionInput{Kind: "press_keys", Keys: []string{" Mod ", "Return", "ESC", "a", "UnknownKey"}}
	if err := input.validate(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(input.action().Keys, []string{"mod", "enter", "escape", "a", "unknownkey"}) {
		t.Fatalf("unexpected normalized keys: %v", input.action().Keys)
	}
	// Unknown names remain subject to the runtime's closed key validation.
	approval := computerActionApprovalInput("keys-1", input)
	if !slices.Equal(approval["keys"].([]string), input.action().Keys) {
		t.Fatal("approval and dispatch keys differ")
	}
}

type permissionStub struct {
	decision sdkpermission.Decision
	request  sdkpermission.Request
	session  string
}

func (s *permissionStub) RequestPermissionWithID(_ context.Context, session string, request sdkpermission.Request) (sdkpermission.Decision, string, error) {
	s.session = session
	s.request = request
	return s.decision, "perm-test", nil
}

func TestComputerActionApprovalFailsClosedWithoutHostChannel(t *testing.T) {
	r := registry{sessionKey: "runtime-session-1"}
	if err := r.requestActionApproval(context.Background(), "action-20261013-1", actionInput{Kind: "click_point"}); err == nil {
		t.Fatal("missing host permission channel unexpectedly allowed action")
	}

	permissions := &permissionStub{decision: sdkpermission.Deny("declined", false)}
	r.permissions = permissions
	if err := r.requestActionApproval(context.Background(), "action-20261013-2", actionInput{Kind: "click_point"}); err == nil || err.Error() != "declined" {
		t.Fatalf("denied action error = %v", err)
	}
}

func TestComputerOperationContractIsClosedAndStable(t *testing.T) {
	operations := BuildAll(nil, "owner-a", "round-a")
	contract, err := command.BuildContract(command.DomainComputer, "get_computer", "perform_action", operations)
	if err != nil {
		t.Fatal(err)
	}
	if len(contract.Operations) != 1 || contract.Operations[0].Name != "perform_action" {
		t.Fatalf("selected contract = %#v", contract.Operations)
	}
	if contract.Operations[0].Kind != "mutation" || !contract.Operations[0].Idempotent {
		t.Fatalf("perform_action metadata = %#v", contract.Operations[0])
	}
	if contract.Operations[0].InputSchema["additionalProperties"] != false {
		t.Fatalf("perform_action schema is not closed: %#v", contract.Operations[0].InputSchema)
	}
	properties, ok := contract.Operations[0].InputSchema["properties"].(map[string]any)
	if !ok || properties["kind"] == nil {
		t.Fatalf("perform_action schema has no kind property: %#v", contract.Operations[0].InputSchema)
	}
}

func TestRecoverableModelHintsDoNotOverrideUnknownMutation(t *testing.T) {
	for _, tc := range []struct {
		code           nexuscua.ErrorCode
		recovery, want string
	}{
		{nexuscua.ErrorStaleObservation, "observe_window", "target grant has not expired"},
		{nexuscua.ErrorInvalidRequest, "discover_applications", "Do not repeat launch_application"},
	} {
		for _, status := range []nexuscua.MutationStatus{nexuscua.MutationNotDispatched, nexuscua.MutationIndeterminate} {
			result := rejected(&nexuscua.CUAError{Code: tc.code, MutationStatus: status, RecoveryAction: &tc.recovery})
			var model map[string]any
			if err := json.Unmarshal([]byte(result.Content[0]["text"].(string)), &model); err != nil {
				t.Fatal(err)
			}
			hint, _ := model["recovery_hint"].(string)
			if status == nexuscua.MutationNotDispatched && !strings.Contains(hint, tc.want) {
				t.Fatalf("missing bounded recovery hint: %s", hint)
			}
			if status == nexuscua.MutationIndeterminate && !strings.Contains(hint, "once to reconcile without redispatch") {
				t.Fatal("ordinary recovery overrode unknown mutation guidance")
			}
		}
	}
}
