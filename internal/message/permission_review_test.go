package message

import (
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"testing"
)

// TestPermissionReviewProjection 保证结构化审批事件生成独立审计消息。
func TestPermissionReviewProjection(t *testing.T) {
	p := &Processor{}
	message := sdkprotocol.SystemMessage{Subtype: "permission_review", Data: map[string]any{"tool_use_id": "tool-1", "review": map[string]any{"status": "approved", "risk": "low", "authorization": "high", "rationale": "已授权"}}}
	result := p.projectPermissionReviewSystemMessage(message)
	if result == nil || (*result)["content"] != "已自动批准：已授权" {
		t.Fatalf("message=%v", result)
	}
	if got := p.projectPermissionReviewSystemMessage(sdkprotocol.SystemMessage{Subtype: "permission_review"}); got != nil {
		t.Fatal("malformed review became a message")
	}
}
