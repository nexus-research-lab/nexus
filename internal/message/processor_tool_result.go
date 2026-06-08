package message

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func (p *Processor) processToolResultMessage(message sdkprotocol.ReceivedMessage) *protocol.Message {
	if message.User == nil {
		return nil
	}
	content := normalizeContentBlocks(message.User.Message.Content)
	if len(content) == 0 {
		return nil
	}
	for _, block := range content {
		if normalizeString(block["type"]) != "tool_result" {
			return nil
		}
	}
	enrichedBlocks := make([]map[string]any, 0, len(content))
	for _, block := range content {
		if !p.shouldKeepToolResultBlock(block) {
			continue
		}
		enrichedBlock := p.enrichToolResultBlock(block)
		enrichedBlocks = append(enrichedBlocks, enrichedBlock)
		enrichedBlocks = append(enrichedBlocks, p.workspaceFileArtifactsForToolResult(enrichedBlock)...)
	}
	if len(enrichedBlocks) == 0 {
		return nil
	}
	p.segment.AppendToolResults(enrichedBlocks)
	return p.buildAssistantDurableMessage(true, true, "")
}

func (p *Processor) shouldKeepToolResultBlock(block map[string]any) bool {
	toolUseID := normalizeString(block["tool_use_id"])
	if p.segment.HasToolUse(toolUseID) {
		return true
	}
	// 成功结果只是 tool_use 的附属状态；没有匹配工具时不要把它物化成独立内容块。
	return boolValue(block["is_error"])
}

func (p *Processor) enrichToolResultBlock(block map[string]any) map[string]any {
	enriched := cloneMap(block)
	if len(enriched) == 0 {
		enriched = map[string]any{"type": "tool_result"}
	}
	if boolValue(enriched["is_error"]) {
		toolUseID := normalizeString(enriched["tool_use_id"])
		if toolUseID != "" {
			toolName := p.segment.FindToolName(toolUseID)
			errorCode := inferPermissionErrorCode(toolName, normalizeString(enriched["content"]))
			if errorCode != "" {
				enriched["error_code"] = errorCode
			}
		}
	}
	return enriched
}

func boolValue(value any) bool {
	typed, ok := value.(bool)
	if !ok {
		return false
	}
	return typed
}
