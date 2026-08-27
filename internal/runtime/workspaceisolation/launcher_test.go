package workspaceisolation

import (
	"context"
	"runtime"
	"strings"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

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

func TestOwnerProcessReaperSkipsNonEnforceModes(t *testing.T) {
	for _, mode := range []Mode{ModeOff, ModeAudit} {
		reaper := OwnerProcessReaper{
			Mode:         mode,
			LauncherPath: "/does/not/exist",
		}
		if err := reaper.ReapOwnerProcesses(context.Background(), "owner-a"); err != nil {
			t.Fatalf("mode=%s should skip launcher: %v", mode, err)
		}
	}
}

func TestOwnerProcessReaperRejectsUnsafeOwner(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("owner cgroup 仅在 Linux enforce 生效")
	}
	reaper := OwnerProcessReaper{
		Mode:         ModeEnforce,
		LauncherPath: "/does/not/exist",
	}
	if err := reaper.ReapOwnerProcesses(context.Background(), "../owner"); err == nil {
		t.Fatal("不安全 owner 应被拒绝")
	}
}

func TestRunScriptRejectsUnisolatedServerModes(t *testing.T) {
	for _, mode := range []Mode{ModeOff, ModeAudit} {
		t.Run(string(mode), func(t *testing.T) {
			err := RunScript(
				context.Background(),
				Config{Mode: mode},
				ScriptInput{
					OwnerUserID: "owner-a",
					CWD:         t.TempDir(),
					Script:      "echo blocked",
				},
				nil,
				nil,
			)
			if err == nil || !strings.Contains(err.Error(), "requires runtime isolation enforce") {
				t.Fatalf("RunScript() error = %v", err)
			}
		})
	}
}

func TestScriptLauncherEnvironmentDoesNotAllowTicketOverride(t *testing.T) {
	environment := scriptLauncherEnvironment("trusted-ticket", map[string]string{
		LauncherTicketEnvName:     "attacker-ticket",
		"NEXUS_AUTOMATION_JOB_ID": "job-1",
	})
	joined := strings.Join(environment, "\n")
	if !strings.Contains(joined, LauncherTicketEnvName+"=trusted-ticket") {
		t.Fatalf("launcher ticket 丢失: %v", environment)
	}
	if strings.Contains(joined, "attacker-ticket") {
		t.Fatalf("调用方环境不能覆盖 launcher ticket: %v", environment)
	}
}
