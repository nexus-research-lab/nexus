// INPUT: Provider 归一化的 ToolUseSummary 消息。
// OUTPUT: 当前 Agent round 内收到即发布、可替换且终态即清除的自然语言执行旁白。
// POS: 执行中即时旁白的 ephemeral 消息投影边界；不得写回 durable assistant segment。
package message

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

const maxToolUseSummaryRunes = 240

func (p *Processor) projectToolUseSummary(summary sdkprotocol.ToolUseSummaryMessage) *protocol.Message {
	text := sanitizeToolUseSummary(summary.Summary)
	if text == "" {
		return nil
	}
	payload := protocol.Message(baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		toolUseSummaryMessageID(p.ctx),
		"assistant",
	))
	payload["content"] = []map[string]any{{
		"type":                   "progress_update",
		"text":                   text,
		"preceding_tool_use_ids": append([]string(nil), summary.PrecedingToolUseIDs...),
	}}
	payload["is_complete"] = false
	payload["metadata"] = map[string]any{
		"subtype": "tool_use_summary",
	}
	return &payload
}

func toolUseSummaryMessageID(ctx MessageContext) string {
	identity := strings.Join([]string{
		strings.TrimSpace(ctx.RoundID),
		strings.TrimSpace(ctx.AgentRoundID),
		strings.TrimSpace(ctx.AgentID),
	}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return "msg_assistant_progress_" + hex.EncodeToString(digest[:12])
}

func sanitizeToolUseSummary(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) > maxToolUseSummaryRunes {
		value = string([]rune(value)[:maxToolUseSummaryRunes-1]) + "…"
	}
	return strings.TrimSpace(value)
}
