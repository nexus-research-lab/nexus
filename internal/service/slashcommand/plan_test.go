// INPUT: Exact user and internal Slash requests.
// OUTPUT: Only explicit /plan narrows runtime permission and enters the approval flow.
// POS: Planning command boundary regression.
package slashcommand

import (
	"strings"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestPlanCommandRequiresExactExplicitInput(t *testing.T) {
	for _, tc := range []struct {
		content        string
		internal, plan bool
	}{
		{"/plan", false, true}, {" /PLAN design search", false, true},
		{"/plan design search", true, false}, {"please /plan", false, false},
		{"/planning", false, false}, {"/compact", false, false},
	} {
		got := PlanRequestPermissionMode(tc.content, tc.internal, sdkpermission.ModeBypassPermissions)
		want := sdkpermission.ModeBypassPermissions
		if tc.plan {
			want = sdkpermission.ModePlan
		}
		if got != want {
			t.Errorf("%q internal=%t: got %q want %q", tc.content, tc.internal, got, want)
		}
	}
	for _, content := range []string{"/plan", "/plan design search"} {
		prompt := ExpandProductPrompt(content)
		if !strings.Contains(prompt, "ExitPlanMode") || !strings.Contains(prompt, "before approval") || strings.HasPrefix(prompt, "/plan") {
			t.Fatalf("plan did not expand into the approval workflow: %q", prompt)
		}
	}
}
