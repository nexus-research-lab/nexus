// INPUT: 历史 Goal/Execution CLI 输出或当前结构化 command result。
// OUTPUT: 可与宿主 typed receipt 精确对账的 command identity。
// POS: Runtime Graph 的兼容读取边界；不参与当前 nexus_runtime 授权。
package protocol

import (
	"encoding/json"
	"strings"
)

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
