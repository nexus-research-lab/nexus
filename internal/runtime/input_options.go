package runtime

import (
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// RuntimeInputOptionsForPurpose 剥离只属于本地队列/历史层的控制字段。
func RuntimeInputOptionsForPurpose(options sdkprotocol.OutboundMessageOptions, purpose string) sdkprotocol.OutboundMessageOptions {
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
