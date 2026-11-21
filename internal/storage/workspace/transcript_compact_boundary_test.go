package workspace

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// 自动压缩后 compact_boundary 的 parentUuid 为 null；主链必须沿 logicalParentUuid
// 续接压缩前的历史，且压缩摘要不能成为用户轮次。
func TestPrimaryTranscriptChainCrossesCompactBoundary(t *testing.T) {
	entries := []transcriptEntry{
		{Index: 0, Data: map[string]any{
			"type": "user", "uuid": "user-1", "timestamp": "2026-11-19T01:00:00Z",
			"message": map[string]any{"role": "user", "content": "生成 PPT"},
		}},
		{Index: 1, Data: map[string]any{
			"type": "assistant", "uuid": "assistant-1", "parentUuid": "user-1",
			"message": map[string]any{"id": "msg-1", "role": "assistant", "content": []any{
				map[string]any{"type": "text", "text": "请打开 127.0.0.1 确认"},
			}},
		}},
		{Index: 2, Data: map[string]any{
			"type": "system", "subtype": "compact_boundary", "uuid": "boundary",
			"parentUuid": nil, "logicalParentUuid": "assistant-1",
			"content": "Conversation compacted",
		}},
		{Index: 3, Data: map[string]any{
			"type": "user", "uuid": "summary", "parentUuid": "boundary", "isCompactSummary": true,
			"isVisibleInTranscriptOnly": true,
			"message":                   map[string]any{"role": "user", "content": "This session is being continued from a previous conversation..."},
		}},
		{Index: 4, Data: map[string]any{
			"type": "assistant", "uuid": "assistant-2", "parentUuid": "summary",
			"message": map[string]any{"id": "msg-2", "role": "assistant", "content": []any{
				map[string]any{"type": "text", "text": "PPT 已完成"},
			}},
		}},
	}

	chain := buildPrimaryTranscriptChain(entries)
	uuids := make([]string, 0, len(chain))
	for _, entry := range chain {
		uuids = append(uuids, stringFromAny(entry.Data["uuid"]))
	}
	if got := strings.Join(uuids, ","); got != "user-1,assistant-1,boundary,summary,assistant-2" {
		t.Fatalf("chain = %s, 压缩前历史被截断", got)
	}

	turns := collectTranscriptUserTurns(chain, shouldSkipTranscriptEntry)
	if len(turns) != 1 || turns[0].Content != transcriptUserContentKey("生成 PPT") {
		t.Fatalf("user turns = %+v, 压缩摘要不应成为用户轮次", turns)
	}

	rows := projectTranscriptChain("/workspace", "agent:a:ws:dm:c", "a", chain, nil)
	var texts []string
	for _, row := range rows {
		if row["role"] == "user" && strings.Contains(stringFromAny(row["content"]), "continued from a previous") {
			t.Fatalf("压缩摘要被投影为用户消息: %+v", row)
		}
		for _, block := range normalizeMessageContentBlocks(row["content"]) {
			if block["type"] == "text" {
				texts = append(texts, stringFromAny(block["text"]))
			}
		}
	}
	joined := strings.Join(texts, "|")
	if !strings.Contains(joined, "127.0.0.1") || !strings.Contains(joined, "PPT 已完成") {
		t.Fatalf("texts = %q, 需要同时保留压缩前后的正文", joined)
	}
}

// transcript 与 overlay 对同一 message_id 的快照合并时，不同正文的 text 块不能互相覆盖。
func TestCompactMessagesKeepsDistinctTextBlocks(t *testing.T) {
	first := protocol.Message{
		"message_id": "msg-1", "role": "assistant", "timestamp": int64(1),
		"content": []any{
			map[string]any{"type": "text", "text": "确认页面已打开：127.0.0.1"},
			map[string]any{"type": "tool_use", "id": "tool-1", "name": "Bash"},
		},
	}
	second := protocol.Message{
		"message_id": "msg-1", "role": "assistant", "timestamp": int64(2),
		"content": []any{
			map[string]any{"type": "text", "text": "确认页面已打开：127.0.0.1"},
			map[string]any{"type": "tool_use", "id": "tool-1", "name": "Bash"},
			map[string]any{"type": "text", "text": "如果页面没打开，也可以直接回复"},
		},
	}

	compacted := compactMessages([]protocol.Message{first, second})
	if len(compacted) != 1 {
		t.Fatalf("compacted = %+v", compacted)
	}
	var texts []string
	for _, block := range normalizeMessageContentBlocks(compacted[0]["content"]) {
		if block["type"] == "text" {
			texts = append(texts, stringFromAny(block["text"]))
		}
	}
	if len(texts) != 2 || !strings.Contains(texts[0], "127.0.0.1") {
		t.Fatalf("texts = %q, 先前正文被后续 text 块覆盖", texts)
	}
}
