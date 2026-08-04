// INPUT: runtime user/tool_result 消息、同轮 tool_use 索引与可选 structured output。
// OUTPUT: 保留原始结果、错误分类及显式 mutation 语义的 durable Assistant 快照。
// POS: Provider 工具结果到 DM/Room/Goal/WorkGraph 消费面的统一消息投影边界。
package message

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

var taskListToolNames = map[string]struct{}{
	"TaskCreate": {},
	"TaskList":   {},
	"TaskUpdate": {},
}

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
	structuredOutput := taskToolStructuredOutput(message, len(content))
	enrichedBlocks := make([]map[string]any, 0, len(content))
	for _, block := range content {
		if !p.shouldKeepToolResultBlock(block) {
			continue
		}
		enrichedBlock := p.enrichToolResultBlock(block, structuredOutput)
		enrichedBlocks = append(enrichedBlocks, enrichedBlock)
		enrichedBlocks = append(enrichedBlocks, p.workspaceFileArtifactsForToolResult(enrichedBlock)...)
	}
	if len(enrichedBlocks) == 0 {
		return nil
	}
	p.segment.AppendToolResults(enrichedBlocks)
	return p.buildAssistantDurableMessage(
		true,
		true,
		firstNonEmpty(normalizePointerString(message.User.ParentToolUseID), p.parentToolUseID),
	)
}

func (p *Processor) shouldKeepToolResultBlock(block map[string]any) bool {
	toolUseID := normalizeString(block["tool_use_id"])
	if p.segment.HasToolUse(toolUseID) {
		return true
	}
	// 成功结果只是 tool_use 的附属状态；没有匹配工具时不要把它物化成独立内容块。
	return boolValue(block["is_error"])
}

func (p *Processor) enrichToolResultBlock(
	block map[string]any,
	structuredOutput map[string]any,
) map[string]any {
	enriched := cloneMap(block)
	if len(enriched) == 0 {
		enriched = map[string]any{"type": "tool_result"}
	}
	p.attachTaskToolStructuredOutput(enriched, structuredOutput)
	attachMutationResultMetadata(enriched, structuredOutput)
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

// attachMutationResultMetadata 只缓存显式 mutation envelope 的紧凑语义。
// 原始 provider content 保持不变，Agent 仍可依据完整结果自主修正或重试。
func attachMutationResultMetadata(
	block map[string]any,
	structuredOutput map[string]any,
) {
	result, ok := protocol.ParseMutationResultEnvelope(
		structuredOutput,
		block["structured_output"],
		block["content"],
	)
	if !ok {
		return
	}
	metadata := mapValue(block["metadata"])
	if metadata == nil {
		metadata = make(map[string]any, 3)
	}
	metadata[protocol.MutationOutcomeMetadataKey] = string(result.Outcome)
	if result.Message != "" {
		metadata[protocol.MutationMessageMetadataKey] = result.Message
	}
	if result.ReasonCode != "" {
		metadata[protocol.MutationReasonCodeMetadataKey] = result.ReasonCode
	}
	block["metadata"] = metadata
}

// attachTaskToolStructuredOutput 只保留任务列表工具的机器可读结果，避免前端解析展示文案。
func (p *Processor) attachTaskToolStructuredOutput(
	block map[string]any,
	structuredOutput map[string]any,
) {
	if len(structuredOutput) == 0 || block["structured_output"] != nil {
		return
	}
	toolUseID := normalizeString(block["tool_use_id"])
	if _, ok := taskListToolNames[p.segment.FindToolName(toolUseID)]; !ok {
		return
	}
	block["structured_output"] = cloneMap(structuredOutput)
}

// taskToolStructuredOutput 兼容实时 stream-json 与 Claude Code transcript 的字段命名。
func taskToolStructuredOutput(
	message sdkprotocol.ReceivedMessage,
	blockCount int,
) map[string]any {
	if message.User == nil || blockCount != 1 {
		return nil
	}
	value := message.User.ToolUseResult
	if value == nil {
		value = message.Raw["toolUseResult"]
	}
	return mapValue(value)
}

func boolValue(value any) bool {
	typed, ok := value.(bool)
	if !ok {
		return false
	}
	return typed
}
