// INPUT: bridge 已归一化的 task runtime 消息与累计 task usage。
// OUTPUT: durable/ephemeral task 投影及可供 Goal 去重的 child token 快照。
// POS: runtime 后台任务消息到 Nexus task 语义的统一投影层。
package message

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

const shellToolProgressThrottleSeconds = 30

// SubagentTaskUsage 表示单个 nxs child task 的累计 token 快照。
type SubagentTaskUsage struct {
	TaskID      string
	TotalTokens int64
}

// SubagentTaskUsageSnapshots 从 durable task 消息读取全部有正数证据的 nxs child token 快照。
// system task 消息把 usage 放在 metadata，assistant 快照则可能同时携带多个
// task_progress block；同一 task 在单条消息中只保留最大的累计值。Agents SDK
// 会用 total_tokens=0 表示缺少 usage，因此零值不能作为 provider 结算证据。
func SubagentTaskUsageSnapshots(message protocol.Message) []SubagentTaskUsage {
	totals := make(map[string]int64)
	observe := func(taskID string, usage any) {
		taskID = strings.TrimSpace(taskID)
		totalRaw, present := mapValue(usage)["total_tokens"]
		totalTokens, valid := normalizeInt64Value(totalRaw)
		previous, observed := totals[taskID]
		if taskID == "" || !present || !valid || totalTokens <= 0 ||
			(observed && totalTokens <= previous) {
			return
		}
		totals[taskID] = totalTokens
	}

	metadata := mapValue(message["metadata"])
	observe(normalizeString(metadata["task_id"]), metadata["usage"])
	observeTaskProgress := func(block map[string]any) {
		if normalizeString(block["type"]) != "task_progress" {
			return
		}
		observe(normalizeString(block["task_id"]), block["usage"])
	}
	switch content := message["content"].(type) {
	case []map[string]any:
		for _, block := range content {
			observeTaskProgress(block)
		}
	case []any:
		for _, rawBlock := range content {
			observeTaskProgress(mapValue(rawBlock))
		}
	}

	if len(totals) == 0 {
		return nil
	}
	taskIDs := make([]string, 0, len(totals))
	for taskID := range totals {
		taskIDs = append(taskIDs, taskID)
	}
	sort.Strings(taskIDs)
	snapshots := make([]SubagentTaskUsage, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		snapshots = append(snapshots, SubagentTaskUsage{
			TaskID:      taskID,
			TotalTokens: totals[taskID],
		})
	}
	return snapshots
}

// SubagentTaskUsageSnapshot 保留单 task 调用方的兼容入口。
func SubagentTaskUsageSnapshot(message protocol.Message) (string, int64, bool) {
	snapshots := SubagentTaskUsageSnapshots(message)
	if len(snapshots) == 0 {
		return "", 0, false
	}
	return snapshots[0].TaskID, snapshots[0].TotalTokens, true
}

func (p *Processor) projectTaskProgress(progress sdkprotocol.TaskProgressMessage) *protocol.Message {
	toolName := strings.TrimSpace(progress.LastToolName)
	description := firstNonEmpty(progress.Summary, progress.Description)
	if description == "" && toolName != "" {
		description = toolName + " 正在执行"
	}
	return p.buildTaskProgressMessage(
		firstNonEmpty(progress.TaskID, progress.ToolUseID),
		firstNonEmpty(description, "后台任务正在执行"),
		progress.ToolUseID,
		toolName,
		taskUsageMap(progress.Usage, progress.Additional["usage"]),
		mergeTaskEventMetadata(progress.Additional, map[string]string{
			"agent_id":       progress.AgentID,
			"agent_type":     progress.AgentType,
			"description":    progress.Description,
			"last_tool_name": progress.LastToolName,
			"parent_task_id": progress.ParentTaskID,
			"summary":        progress.Summary,
		}),
	)
}

func (p *Processor) processToolProgressMessage(progress sdkprotocol.ToolProgressMessage) (*protocol.Message, bool) {
	data := mapValue(progress.Additional["data"])
	progressType := normalizeString(data["type"])
	if shellType := resolveShellProgressType(progressType, progress.ToolName); shellType != "" {
		trackingID := firstNonEmpty(
			normalizePointerString(progress.ParentToolUseID),
			progress.ToolUseID,
		)
		if trackingID == "" || !p.shouldEmitShellToolProgress(trackingID, progress.ElapsedTimeSeconds) {
			return nil, true
		}
		toolName := firstNonEmpty(progress.ToolName, shellProgressToolName(shellType))
		elapsedSeconds := int(math.Max(0, progress.ElapsedTimeSeconds))
		description := fmt.Sprintf("%s 已运行 %d 秒", toolName, elapsedSeconds)
		return p.buildEphemeralTaskProgressMessage(
			firstNonEmpty(progress.TaskID, "tool_progress_"+trackingID),
			description,
			trackingID,
			toolName,
			map[string]any{"duration_ms": elapsedSeconds * 1000},
			mergeTaskEventMetadata(data, map[string]string{
				"progress_type": shellType,
			}),
		), true
	}
	if progressType != "agent_progress" {
		return nil, false
	}
	taskID := firstNonEmpty(
		progress.TaskID,
		normalizeString(data["agent_id"]),
		progress.ToolUseID,
	)
	description := firstNonEmpty(
		normalizeString(data["description"]),
		normalizeString(data["agent_type"]),
		"子 Agent 正在执行",
	)
	agentID := firstNonEmpty(normalizeString(data["agent_id"]), taskID)
	metadata := mergeTaskEventMetadata(data, map[string]string{
		"agent_id": agentID,
		"child_session_id": firstNonEmpty(
			normalizeString(data["child_session_id"]),
			normalizeString(data["childSessionId"]),
			agentID,
		),
		"task_type": "local_agent",
	})
	return p.buildTaskProgressMessage(
		taskID,
		description,
		firstNonEmpty(normalizePointerString(progress.ParentToolUseID), progress.ToolUseID),
		firstNonEmpty(agentProgressLastToolName(data), progress.ToolName),
		mapValue(data["usage"]),
		metadata,
	), false
}

func (p *Processor) processSubagentAttachmentMessage(attachment sdkprotocol.AttachmentMessage) *protocol.Message {
	if !strings.EqualFold(strings.TrimSpace(attachment.Type), "structured_output") {
		return nil
	}
	data := mapValue(attachment.Data)
	agentID := firstNonEmpty(
		normalizeString(data["agent_id"]),
		normalizeString(data["agentId"]),
	)
	toolUseID := firstNonEmpty(
		attachment.ToolUseID,
		normalizeString(data["tool_use_id"]),
		normalizeString(data["toolUseId"]),
	)
	if agentID == "" || toolUseID == "" {
		return nil
	}
	status := normalizeSubagentAttachmentStatus(firstNonEmpty(
		normalizeString(data["task_status"]),
		normalizeString(data["taskStatus"]),
		normalizeString(data["status"]),
	))
	description := firstNonEmpty(
		normalizeString(data["description"]),
		normalizeString(data["agent_type"]),
		normalizeString(data["agentType"]),
	)
	return p.buildTaskNotificationMessage(
		agentID,
		firstNonEmpty(description, "子 Agent 状态已更新"),
		toolUseID,
		status,
		firstNonEmpty(normalizeString(data["output_file"]), normalizeString(data["outputFile"])),
		mapValue(data["usage"]),
		map[string]any{
			"agent_id": agentID,
			"agent_type": firstNonEmpty(
				normalizeString(data["agent_type"]),
				normalizeString(data["agentType"]),
			),
			"child_session_id": firstNonEmpty(
				normalizeString(data["child_session_id"]),
				normalizeString(data["childSessionId"]),
				agentID,
			),
			"description": description,
			"task_type":   "local_agent",
		},
	)
}

func normalizeSubagentAttachmentStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "completed", "complete", "success", "succeeded":
		return "completed"
	case "failed", "error":
		return "failed"
	case "killed", "cancelled", "canceled", "stopped", "interrupted":
		return "killed"
	case "pending", "running", "in_progress":
		return "running"
	default:
		return status
	}
}

func resolveShellProgressType(progressType string, toolName string) string {
	switch progressType {
	case "bash_progress", "powershell_progress":
		return progressType
	}
	toolName = strings.TrimSpace(toolName)
	switch {
	case strings.EqualFold(toolName, "Bash"):
		return "bash_progress"
	case strings.EqualFold(toolName, "PowerShell"):
		return "powershell_progress"
	default:
		return ""
	}
}

func (p *Processor) shouldEmitShellToolProgress(trackingID string, elapsedSeconds float64) bool {
	lastElapsed, exists := p.toolProgressElapsed[trackingID]
	if exists && elapsedSeconds-lastElapsed < shellToolProgressThrottleSeconds {
		return false
	}
	p.toolProgressElapsed[trackingID] = elapsedSeconds
	return true
}

func shellProgressToolName(progressType string) string {
	if progressType == "powershell_progress" {
		return "PowerShell"
	}
	return "Bash"
}

func (p *Processor) projectTaskStarted(started sdkprotocol.TaskStartedMessage) *protocol.Message {
	return p.buildTaskStartedMessage(
		firstNonEmpty(started.TaskID, started.ToolUseID),
		firstNonEmpty(started.Description, started.Prompt, "任务已开始"),
		started.TaskType,
		started.ToolUseID,
		mergeTaskEventMetadata(started.Additional, map[string]string{
			"agent_id":       started.AgentID,
			"agent_type":     started.AgentType,
			"description":    started.Description,
			"output_file":    started.OutputFile,
			"parent_task_id": started.ParentTaskID,
			"prompt":         started.Prompt,
			"task_type":      started.TaskType,
			"workflow_name":  started.WorkflowName,
		}),
	)
}

func (p *Processor) projectTaskNotification(notification sdkprotocol.TaskNotificationMessage) *protocol.Message {
	return p.buildTaskNotificationMessage(
		firstNonEmpty(notification.TaskID, notification.ToolUseID),
		firstNonEmpty(notification.Summary, taskNotificationDefaultContent(notification.Status)),
		notification.ToolUseID,
		notification.Status,
		notification.OutputFile,
		taskUsageMap(notification.Usage, notification.Additional["usage"]),
		mergeTaskEventMetadata(notification.Additional, map[string]string{
			"agent_id":        notification.AgentID,
			"agent_type":      notification.AgentType,
			"output_file":     notification.OutputFile,
			"parent_task_id":  notification.ParentTaskID,
			"summary":         notification.Summary,
			"transcript_path": notification.TranscriptPath,
		}),
	)
}

func (p *Processor) projectTaskUpdated(updated sdkprotocol.TaskUpdatedMessage) *protocol.Message {
	taskID := strings.TrimSpace(updated.TaskID)
	if taskID == "" {
		return nil
	}
	status := strings.TrimSpace(updated.Status)
	payload := baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		fmt.Sprintf("system_task_updated_%s_%s_%s", p.ctx.RoundID, taskID, firstNonEmpty(status, "patch")),
		"system",
	)
	payload["content"] = taskUpdatedContent(status)
	payload["metadata"] = map[string]any{
		"subtype": "task_updated",
		"task_id": taskID,
		"status":  emptyToNil(status),
		"patch":   taskUpdatedPatchMap(updated.Patch),
	}
	copyTaskEventMetadata(payload["metadata"].(map[string]any), updated.Additional)
	messageValue := protocol.Message(payload)
	return &messageValue
}

func taskUpdatedPatchMap(patch sdkprotocol.TaskUpdatedPatch) map[string]any {
	values := mergeTaskEventMetadata(patch.Additional, map[string]string{
		"description": patch.Description,
		"error":       patch.Error,
		"status":      patch.Status,
	})
	if patch.EndTime != 0 {
		values["end_time"] = patch.EndTime
	}
	if patch.TotalPausedMS != 0 {
		values["total_paused_ms"] = patch.TotalPausedMS
	}
	if _, present := values["is_backgrounded"]; present || patch.IsBackgrounded {
		values["is_backgrounded"] = patch.IsBackgrounded
	}
	return values
}

func (p *Processor) buildTaskStartedMessage(taskID string, content string, taskType string, toolUseID string, additional map[string]any) *protocol.Message {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	payload := baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		fmt.Sprintf("system_task_started_%s_%s", p.ctx.RoundID, taskID),
		"system",
	)
	payload["content"] = firstNonEmpty(content, "任务已开始")
	payload["metadata"] = map[string]any{
		"subtype":     "task_started",
		"task_id":     taskID,
		"task_type":   emptyToNil(taskType),
		"tool_use_id": emptyToNil(toolUseID),
	}
	copyTaskEventMetadata(payload["metadata"].(map[string]any), additional)
	messageValue := protocol.Message(payload)
	return &messageValue
}

func (p *Processor) buildTaskProgressMessage(
	taskID string,
	description string,
	toolUseID string,
	lastToolName string,
	usage map[string]any,
	additional map[string]any,
) *protocol.Message {
	if !p.appendTaskProgress(taskID, description, toolUseID, lastToolName, usage, additional) {
		return nil
	}
	return p.buildAssistantDurableMessage(false, false, p.parentToolUseID)
}

func (p *Processor) buildEphemeralTaskProgressMessage(
	taskID string,
	description string,
	toolUseID string,
	lastToolName string,
	usage map[string]any,
	additional map[string]any,
) *protocol.Message {
	progress := newTaskProgressBlock(taskID, description, toolUseID, lastToolName, usage, additional)
	if progress == nil {
		return nil
	}
	// Shell 进度只构造运行态快照，不写回主 segment；否则下一条最终
	// assistant 会把临时进度块带进 transcript。
	temporary := p.segment
	temporary.content = cloneBlockSlice(p.segment.content)
	temporary.usage = cloneMap(p.segment.usage)
	temporary.AppendTaskProgress(progress)
	payload := protocol.Message(temporary.BuildAssistantMessage(p.ctx, p.sessionID, false))
	delete(payload, "stop_reason")
	payload["is_complete"] = false
	if parentID := strings.TrimSpace(p.parentToolUseID); parentID != "" {
		payload["parent_id"] = parentID
		payload["parent_tool_use_id"] = parentID
	}
	return &payload
}

func (p *Processor) appendTaskProgress(
	taskID string,
	description string,
	toolUseID string,
	lastToolName string,
	usage map[string]any,
	additional map[string]any,
) bool {
	progress := newTaskProgressBlock(taskID, description, toolUseID, lastToolName, usage, additional)
	if progress == nil {
		return false
	}
	p.segment.AppendTaskProgress(progress)
	return true
}

func newTaskProgressBlock(
	taskID string,
	description string,
	toolUseID string,
	lastToolName string,
	usage map[string]any,
	additional map[string]any,
) map[string]any {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	progress := map[string]any{
		"type":           "task_progress",
		"task_id":        taskID,
		"description":    description,
		"tool_use_id":    emptyToNil(toolUseID),
		"last_tool_name": emptyToNil(lastToolName),
		"usage":          firstNonNilMap(usage, map[string]any{}),
	}
	copyTaskEventMetadata(progress, additional)
	return progress
}

func (p *Processor) buildTaskNotificationMessage(taskID string, content string, toolUseID string, status string, outputFile string, usage map[string]any, additional map[string]any) *protocol.Message {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	payload := baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		fmt.Sprintf("system_task_notification_%s_%s", p.ctx.RoundID, taskID),
		"system",
	)
	payload["content"] = firstNonEmpty(content, "任务状态已更新")
	payload["metadata"] = map[string]any{
		"subtype":     "task_notification",
		"task_id":     taskID,
		"tool_use_id": emptyToNil(toolUseID),
		"status":      emptyToNil(status),
		"output_file": emptyToNil(outputFile),
		"usage":       firstNonNilMap(usage, map[string]any{}),
	}
	copyTaskEventMetadata(payload["metadata"].(map[string]any), additional)
	messageValue := protocol.Message(payload)
	return &messageValue
}

func copyTaskEventMetadata(metadata map[string]any, additional map[string]any) {
	for _, key := range []string{
		"agent_id", "agent_type", "child_session_id", "description", "last_tool_name", "model", "name",
		"output_file", "parent_task_id", "prompt", "summary", "task_type", "team_name",
		"transcript_path", "workflow_name", "progress_type",
	} {
		if value := normalizeString(additional[key]); value != "" {
			metadata[key] = value
		}
	}
}

func mergeTaskEventMetadata(additional map[string]any, fields map[string]string) map[string]any {
	metadata := cloneMapOrEmpty(additional)
	for key, value := range fields {
		if normalized := strings.TrimSpace(value); normalized != "" {
			metadata[key] = normalized
		}
	}
	return metadata
}

// taskUsageMap 只借助 Additional 区分显式 0 与字段缺失；业务值以 typed usage 为主。
func taskUsageMap(usage sdkprotocol.TaskUsage, rawUsage any) map[string]any {
	values := map[string]any{}
	if usage.TotalTokens > 0 {
		values["total_tokens"] = usage.TotalTokens
	}
	if usage.ToolUses > 0 {
		values["tool_uses"] = usage.ToolUses
	}
	if usage.DurationMS > 0 {
		values["duration_ms"] = usage.DurationMS
	}
	raw := mapValue(rawUsage)
	for _, key := range []string{"total_tokens", "tool_uses", "duration_ms"} {
		value, present := raw[key]
		normalized, valid := normalizeInt64Value(value)
		if present && valid && normalized >= 0 {
			values[key] = normalized
		}
	}
	return values
}

func agentProgressLastToolName(data map[string]any) string {
	message := mapValue(data["message"])
	if normalizeString(message["type"]) != "assistant" {
		return ""
	}
	envelope := mapValue(message["message"])
	items, ok := envelope["content"].([]any)
	if !ok {
		return ""
	}
	for index := len(items) - 1; index >= 0; index-- {
		block := mapValue(items[index])
		if normalizeString(block["type"]) != "tool_use" {
			continue
		}
		if name := normalizeString(block["name"]); name != "" {
			return name
		}
	}
	return ""
}

func taskNotificationDefaultContent(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "success", "done":
		return "任务已完成"
	case "stopped", "cancelled", "canceled", "killed", "interrupted":
		return "任务已停止"
	case "failed", "error":
		return "任务执行失败"
	default:
		return "任务状态已更新"
	}
}
