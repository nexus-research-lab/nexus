// INPUT: 已认证 WebSocket 对某个 runtime tool permission request 的人工允许。
// OUTPUT: 可由业务控制面记录的一次性批准意图，不包含可由模型生成的 bearer token。
// POS: runtime permission 与高风险业务写入之间的最小可信桥接契约。
package permission

import (
	"context"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/secretinput"
)

// HumanToolApproval 是服务端从 pending request 固化的人工批准上下文。
type HumanToolApproval struct {
	PermissionRequestID      string
	ToolName                 string
	ToolInput                map[string]any
	ConfigurationSecrets     map[string]string
	ConfigurationSecretSlots []secretinput.Slot
	RuntimeSessionKey        string
	DispatchSessionKey       string
	Route                    RouteContext
	ExpiresAt                time.Time
}

// HumanToolApprovalRecorder 在 runtime 返回 allow 之前记录业务侧的一次性批准。
type HumanToolApprovalRecorder interface {
	RecordHumanToolApproval(context.Context, HumanToolApproval) error
}

func isRecordedHumanApprovalTool(toolName string, toolInput map[string]any) bool {
	if matchesToolLeaf(toolName, "apply_nexus_configuration_change") {
		return true
	}
	return matchesToolLeaf(toolName, "connector_authorization") &&
		normalizeString(toolInput["action"]) == "start"
}

func matchesToolLeaf(toolName string, leaf string) bool {
	if toolName == leaf {
		return true
	}
	for _, separator := range []string{"__", ".", "/"} {
		if len(toolName) > len(leaf)+len(separator) &&
			toolName[len(toolName)-len(leaf)-len(separator):] == separator+leaf {
			return true
		}
	}
	return false
}
