package automation

import "strings"

// HeartbeatFilterResult 表示 heartbeat 回复是否应该继续外发。
type HeartbeatFilterResult struct {
	ShouldDeliver bool
	Text          string
}

// FilterHeartbeatResponse 过滤纯 ACK 回复，保留需要提醒用户的异常文本。
func FilterHeartbeatResponse(text string, ackMaxChars int) HeartbeatFilterResult {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return HeartbeatFilterResult{ShouldDeliver: false, Text: ""}
	}
	if normalized == "HEARTBEAT_OK" {
		return HeartbeatFilterResult{ShouldDeliver: false, Text: ""}
	}

	stripped := normalized
	prefixRemoved := false
	suffixRemoved := false
	if strings.HasPrefix(stripped, "HEARTBEAT_OK") {
		stripped = strings.TrimSpace(strings.TrimPrefix(stripped, "HEARTBEAT_OK"))
		prefixRemoved = true
	}
	if strings.HasSuffix(stripped, "HEARTBEAT_OK") {
		stripped = strings.TrimSpace(strings.TrimSuffix(stripped, "HEARTBEAT_OK"))
		suffixRemoved = true
	}
	if !prefixRemoved && !suffixRemoved {
		return HeartbeatFilterResult{ShouldDeliver: true, Text: text}
	}
	if len([]rune(stripped)) <= ackMaxChars {
		return HeartbeatFilterResult{ShouldDeliver: false, Text: ""}
	}
	return HeartbeatFilterResult{ShouldDeliver: true, Text: stripped}
}
