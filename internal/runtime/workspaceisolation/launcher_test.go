package workspaceisolation

import (
	"context"
	"strings"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestApplyFailsClosedWhenAuthenticatedRuntimeIsNotEnforced(t *testing.T) {
	for _, mode := range []Mode{ModeOff, ModeAudit} {
		t.Run(string(mode), func(t *testing.T) {
			_, err := Apply(
				context.Background(),
				agentclient.Options{},
				Config{Mode: mode},
				Input{RequireEnforce: true},
			)
			if err == nil ||
				!strings.Contains(err.Error(), "requires runtime isolation enforce") {
				t.Fatalf("认证部署未启用 enforce 应拒绝 Agent runtime，err=%v", err)
			}
		})
	}
}

func TestApplyOffStillInstallsRawNexusctlDeny(t *testing.T) {
	options, err := Apply(
		context.Background(),
		agentclient.Options{},
		Config{Mode: ModeOff},
		Input{},
	)
	if err != nil {
		t.Fatal(err)
	}
	matchers := options.Hooks.Matchers[sdkhook.EventPreToolUse]
	if len(matchers) != 1 || len(matchers[0].Hooks) != 1 {
		t.Fatalf("off mode raw nexusctl hooks = %#v", matchers)
	}
	output, err := matchers[0].Hooks[0](context.Background(), sdkhook.Input{
		EventName: sdkhook.EventPreToolUse,
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "/usr/local/bin/nexusctl agent list"},
	}, "raw-cli")
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput == nil ||
		output.SpecificOutput.PermissionDecision != sdkpermission.BehaviorDeny ||
		output.Continue == nil ||
		*output.Continue {
		t.Fatalf("off mode raw nexusctl decision = %#v", output)
	}
}
