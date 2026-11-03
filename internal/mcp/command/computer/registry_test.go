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

func TestComputerImagesUseNativeMCPBlocks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result func(map[string]any) command.Result
	}{
		{"observe", structured},
		{"perform_action", applied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pixels := strings.Repeat("screenshot-bytes", 4096)
			input := map[string]any{
				"observation_id": "obs-current", "image_data": pixels, "image_mime": "image/png",
			}
			result := tc.result(input)
			if len(result.Content) != 2 || result.Content[1]["type"] != "image" ||
				result.Content[1]["data"] != pixels || result.Content[1]["mimeType"] != "image/png" {
				t.Fatal("screenshot was not delivered as a native MCP image block")
			}
			metadata, err := json.Marshal(result.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			for _, value := range []string{string(metadata), result.Content[0]["text"].(string)} {
				if strings.Contains(value, "screenshot-bytes") || strings.Contains(value, "image_data") || strings.Contains(value, "image_mime") {
					t.Fatal("screenshot bytes inflated the text or structured result")
				}
				if !strings.Contains(value, "obs-current") {
					t.Fatal("image extraction lost the observation metadata")
				}
			}
			if input["image_data"] != pixels || input["image_mime"] != "image/png" {
				t.Fatal("image extraction changed the caller's result")
			}
		})
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

func TestRejectedPreservesCUARecoveryFields(t *testing.T) {
	recovery := "retry_with_same_request_id"
	result := rejected(&nexuscua.CUAError{
		Code:           nexuscua.ErrorInternal,
		Message:        "mutation outcome is unknown",
		MutationStatus: nexuscua.MutationIndeterminate,
		RecoveryAction: &recovery,
	})
	if result.StructuredContent["error_code"] != nexuscua.ErrorInternal {
		t.Fatalf("error_code = %#v", result.StructuredContent["error_code"])
	}
	if result.StructuredContent["mutation_status"] != nexuscua.MutationIndeterminate {
		t.Fatalf("mutation_status = %#v", result.StructuredContent["mutation_status"])
	}
	if result.StructuredContent["recovery_action"] != recovery {
		t.Fatalf("recovery_action = %#v", result.StructuredContent["recovery_action"])
	}
}
