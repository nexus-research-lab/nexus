// INPUT: 带嵌套大内容引用的消息页。
// OUTPUT: Session detail 引用注入的不可变性与完整递归覆盖证明。
// POS: Session detail wire 装饰器的纯单元回归测试。
package session

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAttachMessageDetailSessionKeyClonesReferencedContent(t *testing.T) {
	tool := map[string]any{
		"type":       "tool_result",
		"detail_ref": "tool-ref",
		"content": []any{map[string]any{
			"type":       "image",
			"detail_ref": "image-ref",
		}},
	}
	original := []protocol.Message{{
		"message_id": "message-1",
		"content":    []any{tool},
	}}
	projected := attachMessageDetailSessionKey(original, "agent:a:ws:dm:one")

	if original[0]["content"].([]any)[0].(map[string]any)["detail_session_key"] != nil {
		t.Fatal("decorating the wire page mutated canonical message content")
	}
	projectedTool := projected[0]["content"].([]any)[0].(map[string]any)
	if projectedTool["detail_session_key"] != "agent:a:ws:dm:one" {
		t.Fatalf("tool detail session key = %#v", projectedTool["detail_session_key"])
	}
	projectedImage := projectedTool["content"].([]any)[0].(map[string]any)
	if projectedImage["detail_session_key"] != "agent:a:ws:dm:one" {
		t.Fatalf("nested image detail session key = %#v", projectedImage["detail_session_key"])
	}
}
