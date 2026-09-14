package permission

import (
	"context"
	"strings"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestSandboxApprovalKeepsScopeAndDisallowsPersistentRules(t *testing.T) {
	permissions := NewContext()
	review := &sdkpermission.Review{Status: "needs_approval", Rationale: "请确认目标路径"}
	request := sdkpermission.Request{
		Boundary: sdkpermission.BoundarySandboxEscape, ToolName: "Bash", ToolUseID: "escape-call",
		Input: map[string]any{"command": "pwd", "dangerouslyDisableSandbox": true}, Review: review,
		DecisionReason:        "自动审核：请确认目标路径",
		PermissionSuggestions: []sdkpermission.Update{{Type: "setMode", Mode: sdkpermission.ModeBypassPermissions}},
	}
	pending := permissions.newPendingRequest("session-a", request)
	review.Rationale = "mutated after admission"
	if len(pending.Suggestions) != 0 || pending.Review == nil || pending.Review.Rationale != "请确认目标路径" {
		t.Fatal("pending request lost scope or aliased evidence")
	}
	payload := buildPermissionPayload(pending)
	if payload["permission_boundary"] != "sandbox_escape" || payload["risk_label"] != "沙箱外执行" || !strings.Contains(payload["summary"].(string), "批准仅对本次调用生效") {
		t.Fatalf("approval scope not visible: %#v", payload)
	}
	if len(payload["suggestions"].([]map[string]any)) != 0 {
		t.Fatal("UI exposed persistent scope")
	}
	decision := permissions.buildPermissionDecision(context.Background(), pending, map[string]any{"decision": "allow", "updated_permissions": []any{map[string]any{"type": "setMode", "mode": "bypassPermissions"}}})
	if decision.Behavior != sdkpermission.BehaviorDeny || len(decision.UpdatedPermissions) != 0 {
		t.Fatalf("forged persistent grant accepted: %#v", decision)
	}
	decision = permissions.buildPermissionDecision(context.Background(), pending, map[string]any{"decision": "allow"})
	if decision.Behavior != sdkpermission.BehaviorAllow || decision.UpdatedInput["command"] != "pwd" || len(decision.UpdatedPermissions) != 0 {
		t.Fatalf("one-time approval failed: %#v", decision)
	}
}

func TestUnknownApprovalBoundaryFailsBeforeCreatingPending(t *testing.T) {
	permissions := NewContext()
	decision, id, err := permissions.RequestPermissionWithID(context.Background(), "session-a", sdkpermission.Request{ToolName: "Bash", Boundary: "unknown_boundary"})
	if err != nil || decision.Behavior != sdkpermission.BehaviorDeny || id != "" || len(permissions.pendingRequests) != 0 {
		t.Fatalf("unknown boundary admitted: %v %q %#v", err, id, decision)
	}
}

func TestCancelledApprovalDoesNotCreateNewPending(t *testing.T) {
	permissions := NewContext()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	decision, id, err := permissions.RequestPermissionWithID(ctx, "session-a", sdkpermission.Request{ToolName: "Bash", Boundary: sdkpermission.BoundarySandboxEscape})
	if err != nil || decision.Behavior != sdkpermission.BehaviorDeny || id != "" || len(permissions.pendingRequests) != 0 {
		t.Fatalf("cancelled request reappeared: %v %q %#v", err, id, decision)
	}
}
