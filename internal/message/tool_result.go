// INPUT: SDK user tool_result 消息与 assistant tool_use/tool_result 内容块。
// OUTPUT: durable assistant 工具结果与规范化观察。
// POS: runtime 工具结果到产品消息的统一投影；Goal/Execution 语义由宿主 command receipt 投影。
package message

import (
	"strings"
	"unicode"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// ToolResultObservation 表示 assistant 快照中一次已物化的工具结果。
type ToolResultObservation struct {
	ToolUseID string
	ToolName  string
	ErrorCode string
	IsError   bool
	// Recoverable 表示只用于模型自愈的内部工具结果，不代表真实工具执行。
	Recoverable bool
}

const (
	internalToolResultKindMetadataKey = "_nexus_internal_kind"
	malformedToolInputResultKind      = "malformed_tool_input"
)

var taskListToolNames = map[string]struct{}{
	"TaskCreate": {},
	"TaskList":   {},
	"TaskUpdate": {},
}

func (p *Processor) processToolResultMessage(
	user sdkprotocol.UserMessage,
	raw map[string]any,
) *protocol.Message {
	content := normalizeContentBlocks(user.Message.Content)
	if len(content) == 0 {
		return nil
	}
	for _, block := range content {
		if normalizeString(block["type"]) != "tool_result" {
			return nil
		}
	}
	structuredOutput := taskToolStructuredOutput(user, raw, len(content))
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
		firstNonEmpty(normalizePointerString(user.ParentToolUseID), p.parentToolUseID),
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
	p.attachTaskToolStructuredOutput(enriched, structuredOutput)
	attachMutationResultMetadata(enriched, structuredOutput)
	return enriched
}

// attachMutationResultMetadata 只缓存显式 mutation envelope 的紧凑语义。
// 原始 provider content 保持不变，Agent 仍可依据完整结果自主修正或重试。
func attachMutationResultMetadata(
	block map[string]any,
	structuredOutput map[string]any,
) {
	result, hasMutationResult := protocol.ParseMutationResultEnvelope(
		structuredOutput,
		block["structured_output"],
		block["content"],
	)
	goalStatus, hasGoalStatus := protocol.ParseGoalStatusResult(
		structuredOutput,
		block["structured_output"],
		block["content"],
	)
	goalID, hasGoalID := protocol.ParseGoalIDResult(
		structuredOutput,
		block["structured_output"],
		block["content"],
	)
	if !hasMutationResult && !hasGoalStatus && !hasGoalID {
		return
	}
	metadata := mapValue(block["metadata"])
	if metadata == nil {
		metadata = make(map[string]any, 4)
	}
	if hasMutationResult {
		metadata[protocol.MutationOutcomeMetadataKey] = string(result.Outcome)
		if result.Message != "" {
			metadata[protocol.MutationMessageMetadataKey] = result.Message
		}
		if result.ReasonCode != "" {
			metadata[protocol.MutationReasonCodeMetadataKey] = result.ReasonCode
		}
	}
	if hasGoalStatus {
		metadata[protocol.GoalStatusMetadataKey] = string(goalStatus)
	}
	if hasGoalID {
		metadata[protocol.GoalIDMetadataKey] = goalID
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
	user sdkprotocol.UserMessage,
	raw map[string]any,
	blockCount int,
) map[string]any {
	if blockCount != 1 {
		return nil
	}
	value := user.ToolUseResult
	if value == nil {
		value = raw["toolUseResult"]
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

// AssistantToolResults 从 assistant 快照里提取 tool_result，并用同快照中的 tool_use 补齐工具名。
func AssistantToolResults(message protocol.Message) []ToolResultObservation {
	if protocol.MessageRole(message) != "assistant" {
		return nil
	}
	blocks := messageContentBlocks(message["content"])
	if len(blocks) == 0 {
		return nil
	}
	toolNames := make(map[string]string)
	for _, block := range blocks {
		if normalizeString(block["type"]) != "tool_use" {
			continue
		}
		toolUseID := normalizeString(block["id"])
		if toolUseID == "" {
			continue
		}
		toolNames[toolUseID] = normalizeString(block["name"])
	}
	observations := make([]ToolResultObservation, 0)
	for _, block := range blocks {
		if normalizeString(block["type"]) != "tool_result" {
			continue
		}
		toolUseID := normalizeString(block["tool_use_id"])
		if toolUseID == "" {
			continue
		}
		metadata := mapValue(block["metadata"])
		observations = append(observations, ToolResultObservation{
			ToolUseID:   toolUseID,
			ToolName:    toolNames[toolUseID],
			ErrorCode:   normalizeString(block["error_code"]),
			IsError:     boolValue(block["is_error"]),
			Recoverable: normalizeString(metadata[internalToolResultKindMetadataKey]) == malformedToolInputResultKind,
		})
	}
	return observations
}

// AssistantMissedGoalCompletionCommand 判断 assistant 是否声称目标已完成，
// 但宿主没有收到 applied update_goal command receipt。
func AssistantMissedGoalCompletionCommand(message protocol.Message, completionRecorded bool) bool {
	if protocol.MessageRole(message) != "assistant" {
		return false
	}
	if completionRecorded {
		return false
	}
	text := normalizeCompletionSignalText(ExtractAssistantDisplayText(message))
	if text == "" {
		return false
	}
	if claimsPartialGoalWorkOrContinuation(text) {
		return false
	}
	if claimsFinalGoalWorkComplete(text) {
		return true
	}
	return mentionsGoalUpdateCommand(text) &&
		claimsGoalUpdateCommandUnavailable(text) &&
		claimsGoalWorkComplete(text)
}

func normalizeCompletionSignalText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, text)
}

func mentionsGoalUpdateCommand(text string) bool {
	for _, marker := range []string{
		"nexus goal invoke",
		"update_goal",
		"goal update command",
		"更新目标",
		"停止目标",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func claimsGoalUpdateCommandUnavailable(text string) bool {
	for _, marker := range []string{
		"not available",
		"unavailable",
		"not exposed",
		"not visible",
		"not in the command list",
		"cannot call",
		"can't call",
		"could not call",
		"unable to call",
		"no access",
		"don't see",
		"do not see",
		"missing",
		"找不到",
		"没找到",
		"没有看到",
		"没看到",
		"没有权限",
		"无法调用",
		"不能调用",
		"不可用",
		"没有这个命令",
		"没有这样的命令",
		"命令不存在",
		"未暴露",
		"没暴露",
		"不在工具",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func claimsPartialGoalWorkOrContinuation(text string) bool {
	for _, marker := range []string{
		"下一步",
		"下一个步骤",
		"下一阶段",
		"还需要",
		"仍需要",
		"仍需",
		"剩余",
		"未完成",
		"没完成",
		"尚未完成",
		"后续",
		"需要继续",
		"next step",
		"next phase",
		"remaining",
		"still need",
		"still needs",
		"not complete",
		"not completed",
		"not done",
		"unfinished",
		"follow-up",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	if strings.Contains(text, "阶段") && !strings.Contains(text, "所有阶段") && !strings.Contains(text, "全部阶段") {
		return true
	}
	if strings.Contains(text, "phase") && !strings.Contains(text, "all phases") {
		return true
	}
	if strings.Contains(text, "stage") && !strings.Contains(text, "all stages") {
		return true
	}
	return false
}

func claimsGoalWorkComplete(text string) bool {
	for _, marker := range []string{
		"goal is complete",
		"goal has been completed",
		"task is complete",
		"task has been completed",
		"work is complete",
		"work has been completed",
		"deliverable is complete",
		"deliverable has been completed",
		"all requirements are satisfied",
		"no required work remains",
		"already complete",
		"already completed",
		"目标已经完成",
		"目标已完成",
		"任务已经完成",
		"任务已完成",
		"工作已经完成",
		"工作已完成",
		"交付成果已经完成",
		"交付成果已完成",
		"所有要求都已满足",
		"所有要求已经满足",
		"已经完成",
		"已完成",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func claimsFinalGoalWorkComplete(text string) bool {
	for _, marker := range []string{
		"goal is complete",
		"goal has been completed",
		"task is complete",
		"task has been completed",
		"work is complete",
		"work has been completed",
		"deliverable is complete",
		"deliverable has been completed",
		"all requirements are satisfied",
		"no required work remains",
		"completed and verified",
		"done and verified",
		"目标已经完成",
		"目标已完成",
		"任务已经完成",
		"任务已完成",
		"工作已经完成",
		"工作已完成",
		"交付成果已经完成",
		"交付成果已完成",
		"所有要求都已满足",
		"所有要求已经满足",
		"所有阶段已完成",
		"全部阶段已完成",
		"已完成并验证",
		"完成并验证",
		"已完成并可用",
		"完成并可用",
		"已交付",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func messageContentBlocks(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return cloneBlockSlice(typed)
	case []any:
		blocks := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			blocks = append(blocks, cloneMap(block))
		}
		return blocks
	default:
		return nil
	}
}
