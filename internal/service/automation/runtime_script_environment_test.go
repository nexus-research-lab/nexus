package automation

import (
	"strings"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

func TestScriptProcessEnvironmentScrubsHostControlPlaneAndCredentials(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "secret")
	t.Setenv("NEXUSCTL_USER_ID", "owner")
	t.Setenv("DATABASE_URL", "host-database")
	t.Setenv("CONNECTOR_CREDENTIALS_KEY", "connector-secret")
	t.Setenv("HOME", "/host/home")
	t.Setenv("PATH", "/safe/bin")

	environment := scriptProcessEnvironment("/workspace/agent-a", automationdomain.ScheduledTask{
		JobID:   "job-1",
		AgentID: "agent-a",
	}, "run-1")
	values := make(map[string]string, len(environment))
	for _, item := range environment {
		name, value, ok := strings.Cut(item, "=")
		if ok {
			values[name] = value
		}
	}
	for _, forbidden := range []string{
		"OPENAI_API_KEY",
		"NEXUSCTL_USER_ID",
		"DATABASE_URL",
		"CONNECTOR_CREDENTIALS_KEY",
	} {
		if _, ok := values[forbidden]; ok {
			t.Fatalf("script environment leaked %s: %+v", forbidden, values)
		}
	}
	if values["HOME"] != "/workspace/agent-a" ||
		values["USERPROFILE"] != "/workspace/agent-a" ||
		values["PATH"] != "/safe/bin" {
		t.Fatalf("script environment roots are not scoped: %+v", values)
	}
	if values["NEXUS_AUTOMATION_JOB_ID"] != "job-1" ||
		values["NEXUS_AUTOMATION_RUN_ID"] != "run-1" ||
		values["NEXUS_AUTOMATION_AGENT_ID"] != "agent-a" ||
		values["NEXUS_AUTOMATION_EXECUTION"] != "script" {
		t.Fatalf("script execution metadata is incomplete: %+v", values)
	}
}
