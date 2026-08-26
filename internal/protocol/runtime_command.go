// INPUT: 当前 nexus MCP 工具名与输入，或历史 Goal/Execution CLI 输出。
// OUTPUT: 可与宿主 typed receipt 精确对账的 command identity。
// POS: MCP 工具身份与 Runtime Graph 历史兼容读取的协议真相源。
package protocol

import (
	"encoding/json"
	"slices"
	"strings"
)

const (
	NexusMCPServerName           = "nexus"
	NexusGoalReadToolName        = "goal_read"
	NexusGoalWriteToolName       = "goal_write"
	NexusExecutionReadToolName   = "execution_read"
	NexusExecutionWriteToolName  = "execution_write"
	NexusAutomationReadToolName  = "automation_read"
	NexusAutomationPlanToolName  = "automation_plan"
	NexusAutomationApplyToolName = "automation_apply"
)

var nexusManagedToolNames = []string{
	NexusGoalReadToolName,
	NexusGoalWriteToolName,
	NexusExecutionReadToolName,
	NexusExecutionWriteToolName,
	NexusAutomationReadToolName,
	NexusAutomationPlanToolName,
	NexusAutomationApplyToolName,
}

// NexusManagedToolNames 返回顺序稳定的当前 MCP 工具叶子名。
func NexusManagedToolNames() []string {
	return slices.Clone(nexusManagedToolNames)
}

// IsNexusManagedToolName 判断工具名是否属于宿主管理的 nexus MCP surface。
func IsNexusManagedToolName(name string) bool {
	leaf, ok := nexusManagedToolLeaf(name)
	return ok && slices.Contains(nexusManagedToolNames, leaf)
}

// ParseNexusManagedToolIdentity 从当前 MCP 工具身份恢复领域语义。
func ParseNexusManagedToolIdentity(
	name string,
	input map[string]any,
) (RuntimeCommandResultIdentity, bool) {
	leaf, ok := nexusManagedToolLeaf(name)
	if !ok || !slices.Contains(nexusManagedToolNames, leaf) {
		return RuntimeCommandResultIdentity{}, false
	}
	operation := strings.TrimSpace(runtimeCommandResultString(input["operation"]))
	identity := RuntimeCommandResultIdentity{}
	switch leaf {
	case NexusGoalReadToolName:
		identity = RuntimeCommandResultIdentity{Domain: "goal", Action: "inspect", Operation: "get_goal"}
	case NexusGoalWriteToolName:
		identity = RuntimeCommandResultIdentity{Domain: "goal", Action: "invoke", Operation: operation}
	case NexusExecutionReadToolName:
		identity = RuntimeCommandResultIdentity{Domain: "execution", Action: "inspect", Operation: operation}
	case NexusExecutionWriteToolName:
		identity = RuntimeCommandResultIdentity{Domain: "execution", Action: "invoke", Operation: operation}
	case NexusAutomationReadToolName:
		identity = RuntimeCommandResultIdentity{Domain: "automation", Action: "inspect", Operation: operation}
	case NexusAutomationPlanToolName:
		identity = RuntimeCommandResultIdentity{Domain: "automation", Action: "plan", Operation: operation}
	case NexusAutomationApplyToolName:
		identity = RuntimeCommandResultIdentity{Domain: "automation", Action: "apply", Operation: operation}
	}
	if identity.Operation == "" {
		return RuntimeCommandResultIdentity{}, false
	}
	return identity, true
}

// ParseLegacyNexusCommandToolIdentity 只供历史轨迹读取旧 command envelope；
// 当前权限和 MCP discovery 不得调用它恢复旧入口。
func ParseLegacyNexusCommandToolIdentity(
	name string,
	input map[string]any,
) (RuntimeCommandResultIdentity, bool) {
	if !isLegacyNexusCommandToolName(name) {
		return RuntimeCommandResultIdentity{}, false
	}
	identity := RuntimeCommandResultIdentity{
		Domain:    strings.TrimSpace(runtimeCommandResultString(input["domain"])),
		Action:    strings.TrimSpace(runtimeCommandResultString(input["action"])),
		Operation: strings.TrimSpace(runtimeCommandResultString(input["operation"])),
		RequestID: strings.TrimSpace(runtimeCommandResultString(input["request_id"])),
	}
	if identity.Domain != "goal" && identity.Domain != "execution" {
		return RuntimeCommandResultIdentity{}, false
	}
	switch identity.Action {
	case "inspect":
		if identity.Operation == "" {
			identity.Operation = "get_execution"
			if identity.Domain == "goal" {
				identity.Operation = "get_goal"
			}
		}
	case "invoke":
		if identity.Operation == "" || identity.RequestID == "" {
			return RuntimeCommandResultIdentity{}, false
		}
	default:
		return RuntimeCommandResultIdentity{}, false
	}
	return identity, true
}

func isLegacyNexusCommandToolName(name string) bool {
	switch strings.TrimSpace(name) {
	case "mcp__nexus__command", "nexus__command", "nexus.command", "nexus/command":
		return true
	default:
		return false
	}
}

func nexusManagedToolLeaf(name string) (string, bool) {
	name = strings.TrimSpace(name)
	for _, prefix := range []string{
		"mcp__" + NexusMCPServerName + "__",
		NexusMCPServerName + "__",
		NexusMCPServerName + ".",
		NexusMCPServerName + "/",
	} {
		if strings.HasPrefix(name, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(name, prefix)), true
		}
	}
	if slices.Contains(nexusManagedToolNames, name) {
		return name, true
	}
	return "", false
}

// RuntimeCommandResultIdentity 是历史 nexus CLI invoke 输出中可与宿主 typed receipt
// 精确对账的候选身份。它本身不授权，也不能替代宿主 receipt。
type RuntimeCommandResultIdentity struct {
	Domain    string
	Action    string
	Operation string
	RequestID string
}

// ParseRuntimeCommandResultIdentity 只识别历史 CLI 的顶层 JSON envelope。调用方必须
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
