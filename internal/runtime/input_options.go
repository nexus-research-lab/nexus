package runtime

import (
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// InternalInputOptionsForPurpose 保留内部续跑语义，让 runtime 不把控制输入当普通用户消息。
func InternalInputOptionsForPurpose(options sdkprotocol.OutboundMessageOptions, purpose string) sdkprotocol.OutboundMessageOptions {
	if strings.TrimSpace(options.Purpose) != strings.TrimSpace(purpose) {
		return options
	}
	options.Meta = true
	options.Synthetic = true
	options.HiddenFromUser = true
	if strings.TrimSpace(options.Priority) == "" {
		options.Priority = "internal"
	}
	return options
}
