package computer

import (
	"context"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
)

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

func TestComputerActionApprovalUsesHostDecisionAndRedactsText(t *testing.T) {
	permissions := &permissionStub{decision: sdkpermission.Allow(nil, nil)}
	r := registry{permissions: permissions, sessionKey: "runtime-session-1"}
	input := actionInput{Kind: "type_text", Text: "secret text"}

	if err := r.requestActionApproval(context.Background(), "action-20261013-1", input); err != nil {
		t.Fatal(err)
	}
	if permissions.session != "runtime-session-1" {
		t.Fatalf("permission session = %q", permissions.session)
	}
	if permissions.request.ToolName != "nexus_computer_use_action" {
		t.Fatalf("permission tool = %q", permissions.request.ToolName)
	}
	if got := permissions.request.Input["text"]; got != nil {
		t.Fatalf("raw text entered approval input: %#v", got)
	}
	if permissions.request.Input["text_redacted"] != true {
		t.Fatalf("approval input did not mark text as redacted: %#v", permissions.request.Input)
	}
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
