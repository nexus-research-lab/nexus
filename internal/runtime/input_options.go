package runtime

import (
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// VisibleInputOptionsForPurpose 清除只应影响本地持久化/UI 的输入标记，让 runtime 按普通用户输入执行。
func VisibleInputOptionsForPurpose(options sdkprotocol.OutboundMessageOptions, purpose string) sdkprotocol.OutboundMessageOptions {
	if strings.TrimSpace(options.Purpose) != strings.TrimSpace(purpose) {
		return options
	}
	options.Meta = false
	options.Synthetic = false
	options.HiddenFromUser = false
	options.Priority = ""
	options.Purpose = ""
	options.Metadata = nil
	return options
}
