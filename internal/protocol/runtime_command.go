// INPUT: 宿主签发给当前 Agent runtime 的 Nexus command broker、稳定 capability 与私有输入槽。
// OUTPUT: nxs/Claude、CLI 与本机 broker 共用且不可由模型覆盖的环境变量和请求头名称。
// POS: Goal、Execution、Automation 共用的 Agent-facing Nexus CLI 跨进程最小 wire。
package protocol

import (
	"encoding/json"
	"strings"
)

const (
	NexusCommandPathEnvName            = "NEXUS_COMMAND_PATH"
	NexusCommandBrokerURLEnvName       = "NEXUS_COMMAND_BROKER_URL"
	NexusCommandCapabilityTokenEnvName = "NEXUS_COMMAND_CAPABILITY_TOKEN"
	NexusCommandInputPathEnvName       = "NEXUS_COMMAND_INPUT_PATH"
	NexusCommandCapabilityHeader       = "X-Nexus-Runtime-Command-Capability"
)

// RuntimeCommandResultIdentity 是 nexus CLI invoke 输出中可与宿主 typed receipt
// 精确对账的候选身份。它本身不授权，也不能替代 broker receipt。
type RuntimeCommandResultIdentity struct {
	Domain    string
	Action    string
	Operation string
	RequestID string
}

// ParseRuntimeCommandResultIdentity 只识别 CLI 的顶层 JSON envelope。调用方必须
// 再与当前 round 的 host-owned receipt 对账，不能信任任意 shell 输出自报的身份。
func ParseRuntimeCommandResultIdentity(values ...any) (RuntimeCommandResultIdentity, bool) {
	for _, value := range values {
		if identity, ok := parseRuntimeCommandResultIdentity(value); ok {
			return identity, true
		}
	}
	return RuntimeCommandResultIdentity{}, false
}

func parseRuntimeCommandResultIdentity(value any) (RuntimeCommandResultIdentity, bool) {
	switch typed := value.(type) {
	case map[string]any:
		identity := RuntimeCommandResultIdentity{
			Domain:    strings.TrimSpace(runtimeCommandResultString(typed["domain"])),
			Action:    strings.TrimSpace(runtimeCommandResultString(typed["action"])),
			Operation: strings.TrimSpace(runtimeCommandResultString(typed["operation"])),
			RequestID: strings.TrimSpace(runtimeCommandResultString(typed["request_id"])),
		}
		if identity.Action == "invoke" && identity.RequestID != "" &&
			(identity.Domain == "goal" || identity.Domain == "execution") &&
			identity.Operation != "" {
			return identity, true
		}
	case json.RawMessage:
		return parseRuntimeCommandResultIdentityJSON([]byte(typed))
	case []byte:
		return parseRuntimeCommandResultIdentityJSON(typed)
	case string:
		return parseRuntimeCommandResultIdentityJSON([]byte(strings.TrimSpace(typed)))
	}
	return RuntimeCommandResultIdentity{}, false
}

func parseRuntimeCommandResultIdentityJSON(raw []byte) (RuntimeCommandResultIdentity, bool) {
	if len(raw) == 0 || len(raw) > mutationResultJSONLimit {
		return RuntimeCommandResultIdentity{}, false
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return RuntimeCommandResultIdentity{}, false
	}
	return parseRuntimeCommandResultIdentity(decoded)
}

func runtimeCommandResultString(value any) string {
	result, _ := value.(string)
	return result
}
