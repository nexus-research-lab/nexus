package dm

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBuildRewriteRuntimeHistoryContextStopsBeforeTargetRound(t *testing.T) {
	contextValue := buildRewriteRuntimeHistoryContext([]protocol.Message{
		{
			"role":      "user",
			"round_id":  "round-1",
			"content":   "第一问",
			"timestamp": int64(1000),
		},
		{
			"role":     "assistant",
			"round_id": "round-1",
			"content": []map[string]any{
				{"type": "text", "text": "第一答"},
			},
			"timestamp": int64(1100),
		},
		{
			"role":      "user",
			"round_id":  "round-2",
			"content":   "旧问题",
			"timestamp": int64(2000),
		},
		{
			"role":     "assistant",
			"round_id": "round-2",
			"content": []map[string]any{
				{"type": "text", "text": "旧回答"},
			},
			"timestamp": int64(2100),
		},
	}, "round-2")

	if !strings.Contains(contextValue, "第一问") || !strings.Contains(contextValue, "第一答") {
		t.Fatalf("重写历史上下文应保留目标轮次之前的内容: %s", contextValue)
	}
	if strings.Contains(contextValue, "旧问题") || strings.Contains(contextValue, "旧回答") {
		t.Fatalf("重写历史上下文不应包含被编辑旧轮次: %s", contextValue)
	}
}
