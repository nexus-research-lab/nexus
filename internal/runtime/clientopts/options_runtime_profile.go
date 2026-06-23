package clientopts

import (
	"strings"

	runtimeprovider "github.com/nexus-research-lab/nexus/internal/runtime/provider"
)

const nexusNXSCommandPathEnvName = "NEXUS_NXS_COMMAND_PATH"
const nexusAgentRuntimeKindEnvName = "NEXUS_AGENT_RUNTIME_KIND"
const nexusAgentRuntimeEnvName = "NEXUS_AGENT_RUNTIME"
const runtimeKindClaude = runtimeprovider.RuntimeKindClaude
const runtimeKindNXS = runtimeprovider.RuntimeKindNXS

type runtimeProfile struct {
	kind string
}

func resolveRuntimeProfile(runtimeKind string, getenv func(string) string) runtimeProfile {
	return runtimeProfileForKind(resolveRuntimeKind(runtimeKind, getenv))
}

func runtimeProfileForKind(runtimeKind string) runtimeProfile {
	if runtimeKind == runtimeKindNXS {
		return runtimeProfile{kind: runtimeKindNXS}
	}
	return runtimeProfile{kind: runtimeKindClaude}
}

func (p runtimeProfile) isNXS() bool {
	return p.kind == runtimeKindNXS
}

func (p runtimeProfile) supportsAPIFormat(apiFormat string) bool {
	return runtimeprovider.SupportsAPIFormat(p.kind, apiFormat)
}

func resolveRuntimeKind(runtimeKind string, getenv func(string) string) string {
	for _, value := range []string{
		getenv(nexusAgentRuntimeKindEnvName),
		getenv(nexusAgentRuntimeEnvName),
		runtimeKind,
	} {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case runtimeKindNXS, "go", "go-native", "gonative":
			return runtimeKindNXS
		case runtimeKindClaude, "claude-code", "claudecode":
			return runtimeKindClaude
		case "":
			continue
		}
	}
	return runtimeKindNXS
}
