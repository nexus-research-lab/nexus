//go:build linux

package runtimeidentity

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizedRuntimeEnvironmentNeverRestoresNexusCLI(t *testing.T) {
	stateRoot := filepath.Join(string(filepath.Separator), "srv", "nexus")
	homeRoot := filepath.Join(stateRoot, "runtime-home")
	policy := preparedPolicy{
		OwnerUserID: "owner-a",
		CWD:         filepath.Join(stateRoot, "users", "owner-a", "workspace", "agent-a"),
		EnvironmentNames: []string{
			"NEXUSCTL_COMMAND_PATH",
			"NEXUSCTL_USER_ID",
			"NEXUS_CLI_COMMAND_PATH",
			"NEXUS_CLI",
		},
		Identity: preparedIdentity{
			Username: "nexus-owner-a",
			HomeDir:  homeRoot,
			TempDir:  filepath.Join(homeRoot, "tmp"),
		},
	}
	result := sanitizedRuntimeEnvironment([]string{
		"NEXUSCTL_COMMAND_PATH=/host/nexusctl",
		"NEXUSCTL_USER_ID=forged-owner",
		"NEXUS_CLI_COMMAND_PATH=/host/nexusctl",
		"NEXUS_CLI=enabled",
		"PATH=/host/bin",
	}, launcherConfig{StateRoot: stateRoot}, policy)
	values := runtimeEnvironmentMap(result)

	for name := range values {
		if name == "NEXUSCTL" || name == "NEXUS_CLI" ||
			strings.HasPrefix(name, "NEXUSCTL_") ||
			strings.HasPrefix(name, "NEXUS_CLI_") {
			t.Fatalf("runtime 环境泄漏原始 Nexus CLI capability: %s=%q", name, values[name])
		}
	}
	if strings.Contains(values["PATH"], filepath.Join(stateRoot, "app", ".agents", "bin")) {
		t.Fatalf("runtime PATH 仍包含旧 Agent CLI shim 目录: %q", values["PATH"])
	}
	if !strings.HasPrefix(values["PATH"], filepath.Join(homeRoot, ".local", "bin")+":") {
		t.Fatalf("runtime PATH 未从 owner 私有 bin 开始: %q", values["PATH"])
	}
}

func runtimeEnvironmentMap(environment []string) map[string]string {
	result := make(map[string]string, len(environment))
	for _, entry := range environment {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			result[name] = value
		}
	}
	return result
}
