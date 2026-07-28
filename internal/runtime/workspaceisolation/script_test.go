package workspaceisolation

import (
	"context"
	"strings"
	"testing"
)

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
