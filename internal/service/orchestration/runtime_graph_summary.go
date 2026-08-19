// INPUT: Bridge provider-neutral lifecycle 事件与当前强类型 runtime 消息。
// OUTPUT: 有界、脱敏的 NodeRun 结果/错误摘要、耗时与 exact retry 关联线索。
// POS: Bridge 原始内容与持久 Runtime Graph 之间的安全观测投影；不推断或触发 Agent 路线。
package orchestration

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/toolpolicy"
)

const runtimeGraphSummaryRuneLimit = 600

var (
	runtimeGraphBearerPattern      = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*(?:bearer\s+)?)[^\s,;]+`)
	runtimeGraphSecretPattern      = regexp.MustCompile(`(?i)(["']?(?:api[_-]?key|access[_-]?token|refresh[_-]?token|secret|password)["']?\s*[:=]\s*["']?)[^\s,"';}]+`)
	runtimeGraphQuerySecretPattern = regexp.MustCompile(`(?i)([?&](?:api[_-]?key|access[_-]?token|token|key|secret|password)=)[^&#\s]+`)
	runtimeGraphPEMPattern         = regexp.MustCompile(`(?s)-----BEGIN [^-]+-----.*?-----END [^-]+-----`)
	runtimeGraphInternalSentinel   = regexp.MustCompile(`(?i)__nexus_[a-z0-9_]+__`)
)

type runtimeGraphNodeEvidence struct {
	resultSummary    string
	errorCode        string
	errorSummary     string
	summaryTruncated bool
	durationMS       int64
	retryOfSubjectID string
	mutationOutcome  protocol.MutationResultOutcome
	executionID      string
	changed          []string
	semanticFailed   bool
	commandIdentity  protocol.RuntimeCommandResultIdentity
}

// runtimeGraphLifecycleEvents 保留 Bridge 的 canonical lifecycle，并把
// provider 已明确关联的 ToolUseSummary 投影为只读 progress 事实。
func runtimeGraphLifecycleEvents(
	message sdkprotocol.ReceivedMessage,
) []sdkprotocol.RuntimeLifecycleEvent {
	events := message.RuntimeLifecycle
	if len(events) == 0 {
		events = sdkprotocol.DeriveRuntimeLifecycleEvents(message)
	}
	result := append([]sdkprotocol.RuntimeLifecycleEvent(nil), events...)
	for index := range result {
		annotateRuntimeGraphCommandTransport(message, &result[index])
	}
	result = append(result, runtimeGraphSubagentToolEvents(message)...)
	if message.ToolUseSummary == nil || strings.TrimSpace(message.ToolUseSummary.Summary) == "" {
		return result
	}
	for _, toolUseID := range message.ToolUseSummary.PrecedingToolUseIDs {
		toolUseID = strings.TrimSpace(toolUseID)
		if toolUseID == "" {
			continue
		}
		result = append(result, sdkprotocol.RuntimeLifecycleEvent{
			EventID:     firstNonEmpty(message.UUID, message.SessionID, "runtime") + ":tool:summary:" + toolUseID,
			NodeKind:    sdkprotocol.RuntimeLifecycleNodeTool,
			Phase:       sdkprotocol.RuntimeLifecycleProgress,
			SubjectID:   toolUseID,
			Description: strings.TrimSpace(message.ToolUseSummary.Summary),
			Status:      "running",
			Metadata:    map[string]string{"summary_kind": "provider"},
		})
	}
	return result
}

// runtimeGraphSubagentToolEvents recovers exact child ToolUse / ToolResult
// identities carried in the SDK's structured Agent attachment. The attachment
// is already part of the observed parent stream; only low-sensitivity
// lifecycle fields are projected, never the nested prompt, input, or output.
func runtimeGraphSubagentToolEvents(
	message sdkprotocol.ReceivedMessage,
) []sdkprotocol.RuntimeLifecycleEvent {
	if message.Attachment == nil {
		return nil
	}
	data, ok := message.Attachment.Data.(map[string]any)
	if !ok {
		return nil
	}
	parentToolUseID := firstNonEmpty(
		strings.TrimSpace(message.Attachment.ToolUseID),
		mapString(data, "toolUseId"),
		mapString(data, "tool_use_id"),
	)
	if parentToolUseID == "" {
		return nil
	}
	rawMessages, ok := data["messages"].([]any)
	if !ok || len(rawMessages) == 0 {
		return nil
	}
	result := make([]sdkprotocol.RuntimeLifecycleEvent, 0)
	for _, rawMessage := range rawMessages {
		payload, ok := rawMessage.(map[string]any)
		if !ok {
			continue
		}
		nested, err := sdkprotocol.DecodeMessage(payload)
		if err != nil {
			continue
		}
		for _, event := range sdkprotocol.DeriveRuntimeLifecycleEvents(nested) {
			annotateRuntimeGraphCommandTransport(nested, &event)
			if strings.TrimSpace(event.ParentSubjectID) == "" {
				event.ParentSubjectID = parentToolUseID
			}
			result = append(result, event)
		}
	}
	return result
}

// annotateRuntimeGraphCommandTransport projects only a boolean semantic fact
// from the exact ToolUse input. The strict parser rejects pipelines, chaining,
// substitutions, arbitrary executables and non-managed input files; raw command
// text and Tool input never enter Runtime Graph persistence.
func annotateRuntimeGraphCommandTransport(
	message sdkprotocol.ReceivedMessage,
	event *sdkprotocol.RuntimeLifecycleEvent,
) {
	if event == nil || event.NodeKind != sdkprotocol.RuntimeLifecycleNodeTool ||
		message.Assistant == nil {
		return
	}
	for _, block := range message.Assistant.Message.Content {
		toolUse, ok := sdkprotocol.AsToolUseBlock(block)
		if !ok || strings.TrimSpace(toolUse.ID) != strings.TrimSpace(event.SubjectID) {
			continue
		}
		invocation, ok := toolpolicy.NexusRuntimeCLIInvocation(sdkpermission.Request{
			ToolName:  toolUse.Name,
			Input:     toolUse.InputMap(),
			ToolUseID: toolUse.ID,
		})
		if !ok || invocation.Domain != "goal" && invocation.Domain != "execution" {
			return
		}
		if event.Metadata == nil {
			event.Metadata = make(map[string]string)
		}
		event.Metadata[runtimeGraphCommandTransportMetadataKey] = "true"
		event.Metadata[runtimeGraphCommandDomainMetadataKey] = invocation.Domain
		event.Metadata[runtimeGraphCommandActionMetadataKey] = invocation.Action
		return
	}
}

func runtimeGraphEvidenceForEvent(
	message sdkprotocol.ReceivedMessage,
	event sdkprotocol.RuntimeLifecycleEvent,
) runtimeGraphNodeEvidence {
	evidence := runtimeGraphNodeEvidence{
		retryOfSubjectID: firstNonEmpty(
			strings.TrimSpace(event.Metadata["retry_of_subject_id"]),
			strings.TrimSpace(event.Metadata["retry_of_tool_use_id"]),
			strings.TrimSpace(event.Metadata["retry_of_task_id"]),
		),
	}
	if event.NodeKind == sdkprotocol.RuntimeLifecycleNodeTool {
		if progress := message.ToolProgress; progress != nil &&
			strings.TrimSpace(progress.ToolUseID) == strings.TrimSpace(event.SubjectID) &&
			progress.ElapsedTimeSeconds > 0 {
			evidence.durationMS = int64(math.Round(progress.ElapsedTimeSeconds * 1000))
		}
		if summary := message.ToolUseSummary; summary != nil &&
			containsTrimmedString(summary.PrecedingToolUseIDs, event.SubjectID) {
			evidence.resultSummary, evidence.summaryTruncated = compactRuntimeGraphSummary(summary.Summary)
		}
		if event.Phase == sdkprotocol.RuntimeLifecycleFinished {
			applyRuntimeToolResultEvidence(&evidence, message, event.SubjectID)
		}
	}
	if event.NodeKind == sdkprotocol.RuntimeLifecycleNodeSubagent &&
		event.Phase == sdkprotocol.RuntimeLifecycleFinished {
		summary, truncated := compactRuntimeGraphSummary(event.Description)
		evidence.summaryTruncated = evidence.summaryTruncated || truncated
		if event.Failed {
			evidence.errorSummary = summary
		} else {
			evidence.resultSummary = summary
		}
	}
	return evidence
}

func applyRuntimeToolResultEvidence(
	evidence *runtimeGraphNodeEvidence,
	message sdkprotocol.ReceivedMessage,
	toolUseID string,
) {
	if evidence == nil || message.User == nil {
		return
	}
	for _, block := range message.User.Message.Content {
		toolResult, ok := sdkprotocol.AsToolResultBlock(block)
		if !ok || strings.TrimSpace(toolResult.ToolUseID) != strings.TrimSpace(toolUseID) {
			continue
		}
		raw := toolResult.RawPayload()
		mutationResult, hasMutationResult := protocol.ParseMutationResultEnvelope(
			message.User.ToolUseResult,
			message.Raw["toolUseResult"],
			raw["structured_output"],
			raw["content"],
			toolResult.Content,
		)
		if hasMutationResult {
			evidence.mutationOutcome = mutationResult.Outcome
			evidence.executionID = mutationResult.ExecutionID
			evidence.changed = mutationResult.Changed
		}
		if identity, ok := protocol.ParseRuntimeCommandResultIdentity(
			message.User.ToolUseResult,
			message.Raw["toolUseResult"],
			raw["structured_output"],
			raw["content"],
			toolResult.Content,
		); ok {
			evidence.commandIdentity = identity
		}
		evidence.errorCode = firstNonEmpty(
			mutationResult.ReasonCode,
			mapString(raw, "error_code"),
			mapString(raw, "code"),
		)
		summary, truncated := compactRuntimeGraphSummary(runtimeGraphResultText(toolResult.Content))
		if mutationResult.Message != "" {
			summary, truncated = compactRuntimeGraphSummary(mutationResult.Message)
		}
		evidence.summaryTruncated = evidence.summaryTruncated || truncated
		if toolResult.IsError {
			evidence.errorSummary = summary
			if evidence.errorSummary == "" {
				evidence.errorSummary = "Tool execution failed"
			}
		} else if mutationResult.Outcome == protocol.MutationResultRejected {
			evidence.semanticFailed = true
			evidence.errorSummary = summary
			if evidence.errorSummary == "" {
				evidence.errorSummary = "Tool request was rejected"
			}
		} else if summary != "" {
			evidence.resultSummary = summary
		}
		return
	}
}

func runtimeGraphResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return runtimeGraphReadableValue(value, 0)
}

func runtimeGraphReadableValue(value any, depth int) string {
	if depth > 2 {
		return ""
	}
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			var nested any
			if json.Unmarshal([]byte(trimmed), &nested) == nil {
				if readable := runtimeGraphReadableValue(nested, depth+1); readable != "" {
					return readable
				}
			}
		}
		return typed
	case []any:
		parts := make([]string, 0, min(len(typed), 4))
		for _, item := range typed {
			if part := runtimeGraphReadableValue(item, depth+1); part != "" {
				parts = append(parts, part)
			}
			if len(parts) == 4 {
				break
			}
		}
		return strings.Join(parts, " ")
	case map[string]any:
		for _, key := range []string{"summary", "error", "message", "result", "text", "content"} {
			if part := runtimeGraphReadableValue(typed[key], depth+1); part != "" {
				return part
			}
		}
	case json.Number:
		return typed.String()
	case float64, bool:
		return fmt.Sprint(typed)
	}
	return ""
}

func compactRuntimeGraphSummary(value string) (string, bool) {
	value = runtimeGraphInternalSentinel.ReplaceAllString(value, " ")
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "", false
	}
	value = runtimeGraphPEMPattern.ReplaceAllString(value, "<redacted-private-key>")
	value = runtimeGraphBearerPattern.ReplaceAllString(value, "${1}<redacted>")
	value = runtimeGraphSecretPattern.ReplaceAllString(value, "${1}<redacted>")
	value = runtimeGraphQuerySecretPattern.ReplaceAllString(value, "${1}<redacted>")
	runes := []rune(value)
	if len(runes) <= runtimeGraphSummaryRuneLimit {
		return value, false
	}
	return strings.TrimSpace(string(runes[:runtimeGraphSummaryRuneLimit])) + "…", true
}

func containsTrimmedString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func mapString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}
