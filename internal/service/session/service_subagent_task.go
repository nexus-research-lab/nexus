package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

const maxSubagentOutputBytes = 2 * 1024 * 1024

var (
	// ErrSubagentTaskNotFound 表示指定 subagent task 不存在。
	ErrSubagentTaskNotFound = errors.New("subagent task not found")
	// ErrSubagentTaskNotRunning 表示 subagent task 已经不在可交互运行态。
	ErrSubagentTaskNotRunning = errors.New("subagent task is not running")
	// ErrSubagentRuntimeUnavailable 表示 task 所属 runtime 当前不可停止。
	ErrSubagentRuntimeUnavailable = errors.New("subagent runtime unavailable")
	// ErrSubagentOperationUnsupported 表示当前 runtime 不支持指定 task 操作。
	ErrSubagentOperationUnsupported = errors.New("subagent operation unsupported")
)

const (
	subagentRuntimeMixed   = "mixed"
	subagentRuntimeUnknown = "unknown"
)

// SubagentTaskCapabilities 描述当前 runtime 对 subagent task 暴露的能力。
type SubagentTaskCapabilities struct {
	Observe     bool `json:"observe"`
	Transcript  bool `json:"transcript"`
	Stop        bool `json:"stop"`
	SendMessage bool `json:"send_message"`
	Resume      bool `json:"resume"`
}

// SubagentTaskList 是 task 列表及其 runtime 能力契约。
type SubagentTaskList struct {
	RuntimeKind  string                   `json:"runtime_kind"`
	Capabilities SubagentTaskCapabilities `json:"capabilities"`
	Items        []SubagentTask           `json:"items"`
}

// SubagentTask 表示父会话里可见的后台子 Agent 任务。
type SubagentTask struct {
	TaskID     string `json:"task_id"`
	SessionKey string `json:"session_key,omitempty"`
	// AgentID 是 SDK subagent/thread recipient，保留既有 API 语义。
	AgentID string `json:"agent_id,omitempty"`
	// HostAgentID 是承载 runtime 的 Nexus Agent，用于 Room slot 路由。
	HostAgentID    string                   `json:"host_agent_id,omitempty"`
	AgentType      string                   `json:"agent_type,omitempty"`
	ChildSessionID string                   `json:"child_session_id,omitempty"`
	Description    string                   `json:"description,omitempty"`
	Summary        string                   `json:"summary,omitempty"`
	LastToolName   string                   `json:"last_tool_name,omitempty"`
	Model          string                   `json:"model,omitempty"`
	Name           string                   `json:"name,omitempty"`
	ParentTaskID   string                   `json:"parent_task_id,omitempty"`
	RoundID        string                   `json:"round_id,omitempty"`
	Status         string                   `json:"status"`
	TeamName       string                   `json:"team_name,omitempty"`
	TaskType       string                   `json:"task_type,omitempty"`
	ToolUseID      string                   `json:"tool_use_id,omitempty"`
	OutputFile     string                   `json:"output_file,omitempty"`
	TranscriptPath string                   `json:"transcript_path,omitempty"`
	Usage          map[string]any           `json:"usage,omitempty"`
	StartedAt      int64                    `json:"started_at,omitempty"`
	UpdatedAt      int64                    `json:"updated_at,omitempty"`
	RuntimeKind    string                   `json:"runtime_kind"`
	Capabilities   SubagentTaskCapabilities `json:"capabilities"`
}

// SubagentTaskMessages 表示 task 详情页需要的只读消息。
type SubagentTaskMessages struct {
	Task     SubagentTask       `json:"task"`
	Messages []protocol.Message `json:"messages"`
	Output   string             `json:"output,omitempty"`
}

// SubagentTaskStopResult 表示停止 task 的结果。
type SubagentTaskStopResult struct {
	Success bool   `json:"success"`
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
}

// SubagentTaskMessageResult 表示投递 subagent 后续消息的结果。
type SubagentTaskMessageResult struct {
	Success bool   `json:"success"`
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
}

// ListSubagentTasks 从当前会话历史中聚合 subagent task。
func (s *Service) ListSubagentTasks(ctx context.Context, rawSessionKey string) (*SubagentTaskList, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, err
	}
	messages, err := s.GetSessionMessages(ctx, sessionKey)
	if err != nil {
		return nil, err
	}
	defaultRuntimeKind := s.subagentSessionRuntimeKind(ctx, sessionKey, parsed)
	items := buildSubagentTasksWithRuntime(sessionKey, messages, defaultRuntimeKind)
	s.resolveSubagentTaskRuntimeKinds(items, defaultRuntimeKind)
	runtimeKind, capabilities := summarizeSubagentTaskRuntime(items, defaultRuntimeKind)
	return &SubagentTaskList{
		RuntimeKind:  runtimeKind,
		Capabilities: capabilities,
		Items:        items,
	}, nil
}

// GetSubagentTaskMessages 读取指定 subagent task 的 transcript 和 output 摘要。
func (s *Service) GetSubagentTaskMessages(ctx context.Context, rawSessionKey string, taskID string) (*SubagentTaskMessages, error) {
	task, err := s.getSubagentTask(ctx, rawSessionKey, taskID)
	if err != nil {
		return nil, err
	}
	workspacePath := s.subagentTaskWorkspacePath(ctx, *task)
	messages, outputIsTranscript, err := s.readSubagentTaskThread(*task, workspacePath)
	if err != nil {
		return nil, err
	}
	output := ""
	if !outputIsTranscript {
		output, err = readSubagentOutputFile(task.OutputFile)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return &SubagentTaskMessages{Task: *task, Messages: messages, Output: output}, nil
}

func (s *Service) readSubagentTaskThread(
	task SubagentTask,
	workspacePath string,
) ([]protocol.Message, bool, error) {
	agentID := subagentTaskHostAgentID(task)
	transcriptPath := strings.TrimSpace(task.TranscriptPath)
	if transcriptPath != "" {
		messages, err := s.history.ReadTranscriptPathMessages(
			transcriptPath,
			workspacePath,
			task.SessionKey,
			agentID,
		)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, false, err
		}
		return messages, false, nil
	}

	// Claude Code 的 local_agent 把 output_file 建成 child transcript 的符号链接，
	// 没有单独回传 transcript_path。先尝试按 transcript 投影；普通文本或损坏的
	// JSONL 会自然回退为 output，不把兼容性差异升级成 500。
	if strings.EqualFold(strings.TrimSpace(task.TaskType), "local_agent") {
		outputPath := strings.TrimSpace(task.OutputFile)
		if outputPath != "" {
			messages, err := s.history.ReadTranscriptPathMessages(
				outputPath,
				workspacePath,
				task.SessionKey,
				agentID,
			)
			if err == nil && len(messages) > 0 {
				return messages, true, nil
			}
		}
	}
	return []protocol.Message{}, false, nil
}

// StopSubagentTask 停止指定 subagent task。
func (s *Service) StopSubagentTask(ctx context.Context, rawSessionKey string, taskID string) (*SubagentTaskStopResult, error) {
	sessionKey, _, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, ErrSubagentTaskNotFound
	}
	task, err := s.getSubagentTask(ctx, sessionKey, taskID)
	if err != nil {
		return nil, err
	}
	if !task.Capabilities.Stop {
		return nil, fmt.Errorf("%w: stop on %s", ErrSubagentOperationUnsupported, task.RuntimeKind)
	}
	if s.runtime == nil {
		return nil, ErrSubagentRuntimeUnavailable
	}
	runtimeSessionKey := subagentTaskRuntimeSessionKey(*task)
	if runtimeSessionKey == "" {
		return nil, ErrSubagentRuntimeUnavailable
	}
	if err := s.runtime.StopTask(ctx, runtimeSessionKey, taskID); err != nil {
		if runtimectx.IsRuntimeTransportClosedError(err) {
			return nil, fmt.Errorf("%w: %v", ErrSubagentRuntimeUnavailable, err)
		}
		return nil, err
	}
	return &SubagentTaskStopResult{Success: true, TaskID: taskID, Status: "stopped"}, nil
}

// SendSubagentTaskMessage 向 subagent task 排队一条后续消息。
// nxs 会用同一个 task ID 唤醒已完成 task；CC 在进入 wire 前明确拒绝。
func (s *Service) SendSubagentTaskMessage(ctx context.Context, rawSessionKey string, taskID string, message string) (*SubagentTaskMessageResult, error) {
	sessionKey, _, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, err
	}
	taskID = strings.TrimSpace(taskID)
	message = strings.TrimSpace(message)
	if taskID == "" || message == "" {
		return nil, ErrSubagentTaskNotFound
	}
	task, err := s.getSubagentTask(ctx, sessionKey, taskID)
	if err != nil {
		return nil, err
	}
	if !task.Capabilities.SendMessage || !task.Capabilities.Resume {
		return nil, fmt.Errorf("%w: send_message on %s", ErrSubagentOperationUnsupported, task.RuntimeKind)
	}
	if strings.EqualFold(strings.TrimSpace(task.Status), "deleted") {
		return nil, fmt.Errorf("%w: %s", ErrSubagentTaskNotRunning, task.Status)
	}
	if s.runtime == nil {
		return nil, ErrSubagentRuntimeUnavailable
	}
	runtimeSessionKey := subagentTaskRuntimeSessionKey(*task)
	if runtimeSessionKey == "" {
		return nil, ErrSubagentRuntimeUnavailable
	}
	if err := s.runtime.SendTaskMessage(ctx, runtimeSessionKey, taskID, message, subagentTaskMessageSummary(message)); err != nil {
		if runtimectx.IsRuntimeTransportClosedError(err) {
			return nil, fmt.Errorf("%w: %v", ErrSubagentRuntimeUnavailable, err)
		}
		return nil, err
	}
	return &SubagentTaskMessageResult{Success: true, TaskID: taskID, Status: "queued"}, nil
}

func subagentTaskMessageSummary(message string) string {
	message = strings.TrimSpace(message)
	runes := []rune(message)
	if len(runes) <= 80 {
		return message
	}
	return string(runes[:80])
}

func (s *Service) getSubagentTask(ctx context.Context, rawSessionKey string, taskID string) (*SubagentTask, error) {
	taskID = strings.TrimSpace(taskID)
	list, err := s.ListSubagentTasks(ctx, rawSessionKey)
	if err != nil {
		return nil, err
	}
	for index := range list.Items {
		if list.Items[index].TaskID == taskID {
			return &list.Items[index], nil
		}
	}
	return nil, ErrSubagentTaskNotFound
}

func (s *Service) subagentTaskWorkspacePath(ctx context.Context, task SubagentTask) string {
	agentID := subagentTaskHostAgentID(task)
	if agentID == "" {
		return ""
	}
	workspacePaths, err := s.resolveWorkspacePaths(ctx, agentID)
	if err != nil || len(workspacePaths) == 0 {
		return ""
	}
	return workspacePaths[0]
}

func subagentTaskHostAgentID(task SubagentTask) string {
	if agentID := strings.TrimSpace(task.HostAgentID); agentID != "" {
		return agentID
	}
	return strings.TrimSpace(protocol.ParseSessionKey(task.SessionKey).AgentID)
}

func subagentTaskRuntimeSessionKey(task SubagentTask) string {
	parsed := protocol.ParseSessionKey(task.SessionKey)
	if parsed.Kind != protocol.SessionKeyKindRoom {
		return strings.TrimSpace(task.SessionKey)
	}
	hostAgentID := subagentTaskHostAgentID(task)
	if parsed.ConversationID == "" || hostAgentID == "" {
		return ""
	}
	return protocol.BuildRoomAgentSessionKey(parsed.ConversationID, hostAgentID, protocol.RoomTypeGroup)
}

func (s *Service) subagentSessionRuntimeKind(
	ctx context.Context,
	sessionKey string,
	parsed protocol.SessionKey,
) string {
	if s.runtime != nil {
		if kind := s.runtime.RuntimeKind(sessionKey); kind != "" {
			return normalizeSubagentRuntimeKind(string(kind))
		}
	}
	if parsed.Kind == protocol.SessionKeyKindAgent {
		if sessionValue, err := s.GetSession(ctx, sessionKey); err == nil && sessionValue != nil {
			if kind := stringFromAny(sessionValue.Options[protocol.OptionRuntimeKind]); kind != "" {
				return normalizeSubagentRuntimeKind(kind)
			}
		}
	}
	return subagentRuntimeUnknown
}

func normalizeSubagentRuntimeKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "claude", "cc":
		return string(agentclient.RuntimeClaude)
	case "nxs":
		return string(agentclient.RuntimeNXS)
	case subagentRuntimeMixed:
		return subagentRuntimeMixed
	default:
		return subagentRuntimeUnknown
	}
}

func subagentTaskCapabilities(runtimeKind string) SubagentTaskCapabilities {
	capabilities := SubagentTaskCapabilities{
		Observe:    true,
		Transcript: true,
	}
	switch normalizeSubagentRuntimeKind(runtimeKind) {
	case string(agentclient.RuntimeNXS):
		capabilities.Stop = true
		capabilities.SendMessage = true
		capabilities.Resume = true
	case string(agentclient.RuntimeClaude):
		capabilities.Stop = true
	}
	return capabilities
}

func (s *Service) resolveSubagentTaskRuntimeKinds(tasks []SubagentTask, defaultRuntimeKind string) {
	for index := range tasks {
		task := &tasks[index]
		resolvedKind := ""
		if s.runtime != nil {
			if runtimeSessionKey := subagentTaskRuntimeSessionKey(*task); runtimeSessionKey != "" {
				resolvedKind = string(s.runtime.RuntimeKind(runtimeSessionKey))
			}
		}
		if resolvedKind == "" {
			resolvedKind = task.RuntimeKind
		}
		if normalizeSubagentRuntimeKind(resolvedKind) == subagentRuntimeUnknown {
			resolvedKind = defaultRuntimeKind
		}
		task.RuntimeKind = normalizeSubagentRuntimeKind(resolvedKind)
		task.Capabilities = subagentTaskCapabilities(task.RuntimeKind)
	}
}

func summarizeSubagentTaskRuntime(
	tasks []SubagentTask,
	defaultRuntimeKind string,
) (string, SubagentTaskCapabilities) {
	runtimeKind := normalizeSubagentRuntimeKind(defaultRuntimeKind)
	if len(tasks) == 0 {
		return runtimeKind, subagentTaskCapabilities(runtimeKind)
	}
	runtimeKind = normalizeSubagentRuntimeKind(tasks[0].RuntimeKind)
	for _, task := range tasks[1:] {
		if normalizeSubagentRuntimeKind(task.RuntimeKind) != runtimeKind {
			return subagentRuntimeMixed, subagentTaskCapabilities(subagentRuntimeMixed)
		}
	}
	return runtimeKind, subagentTaskCapabilities(runtimeKind)
}

func buildSubagentTasks(sessionKey string, messages []protocol.Message) []SubagentTask {
	return buildSubagentTasksWithRuntime(sessionKey, messages, string(agentclient.RuntimeNXS))
}

func buildSubagentTasksWithRuntime(sessionKey string, messages []protocol.Message, defaultRuntimeKind string) []SubagentTask {
	tasks := map[string]*SubagentTask{}
	order := make([]string, 0)
	for _, message := range messages {
		metadata := subagentTaskMetadata(message)
		subtype := stringFromAny(metadata["subtype"])
		if subtype == "task_started" || subtype == "task_progress" || subtype == "task_notification" || subtype == "task_updated" {
			taskID := stringFromAny(metadata["task_id"])
			if taskID != "" {
				task := tasks[taskID]
				if task == nil && metadataIdentifiesSubagentTask(metadata) {
					task = ensureSubagentTask(tasks, &order, sessionKey, taskID)
				}
				if task != nil {
					mergeSubagentTaskMessage(task, message, metadata, subtype)
				}
			}
		}

		for _, block := range subagentTaskProgressBlocks(message) {
			taskID := stringFromAny(block["task_id"])
			if taskID == "" {
				continue
			}
			task := tasks[taskID]
			if task == nil && !progressBlockMayIdentifySubagentTask(block) {
				continue
			}
			if task == nil {
				task = ensureSubagentTask(tasks, &order, sessionKey, taskID)
			}
			mergeSubagentTaskProgress(task, message, block)
		}
	}
	results := make([]SubagentTask, 0, len(order))
	for _, taskID := range order {
		task := tasks[taskID]
		if isNonSubagentTaskType(task.TaskType) {
			continue
		}
		if task.HostAgentID == "" {
			task.HostAgentID = strings.TrimSpace(protocol.ParseSessionKey(sessionKey).AgentID)
		}
		if task.RuntimeKind == "" {
			task.RuntimeKind = normalizeSubagentRuntimeKind(defaultRuntimeKind)
		}
		task.Capabilities = subagentTaskCapabilities(task.RuntimeKind)
		results = append(results, *task)
	}
	return results
}

func metadataIdentifiesSubagentTask(metadata map[string]any) bool {
	taskType := strings.ToLower(stringFromAny(metadata["task_type"]))
	if isNonSubagentTaskType(taskType) {
		return false
	}
	if taskType != "" {
		return taskType == "local_agent"
	}
	return stringFromAny(metadata["agent_id"]) != "" ||
		stringFromAny(metadata["agent_type"]) != ""
}

func progressBlockMayIdentifySubagentTask(block map[string]any) bool {
	if metadataIdentifiesSubagentTask(block) {
		return true
	}
	// 旧版 CC 只在 assistant content 中给 task_progress，且没有 task_type/agent identity。
	return stringFromAny(block["task_type"]) == ""
}

func isNonSubagentTaskType(taskType string) bool {
	return strings.EqualFold(strings.TrimSpace(taskType), "local_shell")
}

func ensureSubagentTask(tasks map[string]*SubagentTask, order *[]string, sessionKey string, taskID string) *SubagentTask {
	task := tasks[taskID]
	if task != nil {
		return task
	}
	task = &SubagentTask{TaskID: taskID, SessionKey: sessionKey, Status: "running"}
	tasks[taskID] = task
	*order = append(*order, taskID)
	return task
}

func mergeSubagentTaskMessage(task *SubagentTask, message protocol.Message, metadata map[string]any, subtype string) {
	timestamp := int64FromAny(message["timestamp"])
	if task.RoundID == "" {
		task.RoundID = stringFromAny(message["round_id"])
	}
	if subtype == "task_started" && task.StartedAt == 0 {
		task.StartedAt = timestamp
	}
	if timestamp > 0 {
		task.UpdatedAt = timestamp
	}
	setSubagentTaskString(&task.ToolUseID, metadata, "tool_use_id")
	setSubagentTaskString(&task.AgentID, metadata, "agent_id")
	setSubagentTaskString(&task.HostAgentID, message, "agent_id")
	setSubagentTaskString(&task.AgentType, metadata, "agent_type")
	setSubagentTaskString(&task.ChildSessionID, metadata, "child_session_id")
	setSubagentTaskString(&task.Description, metadata, "description")
	if task.Description == "" {
		setSubagentTaskString(&task.Description, message, "content")
	}
	setSubagentTaskString(&task.Model, metadata, "model")
	setSubagentTaskString(&task.Name, metadata, "name")
	setSubagentTaskString(&task.ParentTaskID, metadata, "parent_task_id")
	updateSubagentTaskString(&task.Summary, metadata, "summary")
	updateSubagentTaskString(&task.LastToolName, metadata, "last_tool_name")
	setSubagentTaskString(&task.TaskType, metadata, "task_type")
	setSubagentTaskString(&task.TeamName, metadata, "team_name")
	setSubagentTaskString(&task.OutputFile, metadata, "output_file")
	setSubagentTaskString(&task.TranscriptPath, metadata, "transcript_path")
	if runtimeKind := stringFromAny(metadata["runtime_kind"]); runtimeKind != "" {
		task.RuntimeKind = normalizeSubagentRuntimeKind(runtimeKind)
	}
	if usage := mapFromAny(metadata["usage"]); len(usage) > 0 {
		task.Usage = usage
	}
	if status := stringFromAny(metadata["status"]); status != "" {
		task.Status = status
	}
	if task.Status == "" || task.Status == "running" {
		if patchStatus := stringFromAny(mapFromAny(metadata["patch"])["status"]); patchStatus != "" {
			task.Status = patchStatus
		}
	}
}

func mergeSubagentTaskProgress(task *SubagentTask, message protocol.Message, block map[string]any) {
	timestamp := int64FromAny(message["timestamp"])
	if task.RoundID == "" {
		task.RoundID = stringFromAny(message["round_id"])
	}
	if timestamp > 0 {
		task.UpdatedAt = timestamp
	}
	setSubagentTaskString(&task.ToolUseID, block, "tool_use_id")
	setSubagentTaskString(&task.Description, block, "description")
	updateSubagentTaskString(&task.Summary, block, "summary")
	updateSubagentTaskString(&task.LastToolName, block, "last_tool_name")
	setSubagentTaskString(&task.TaskType, block, "task_type")
	setSubagentTaskString(&task.AgentID, block, "agent_id")
	setSubagentTaskString(&task.HostAgentID, message, "agent_id")
	setSubagentTaskString(&task.AgentType, block, "agent_type")
	setSubagentTaskString(&task.ChildSessionID, block, "child_session_id")
	setSubagentTaskString(&task.ParentTaskID, block, "parent_task_id")
	if usage := mapFromAny(block["usage"]); len(usage) > 0 {
		task.Usage = usage
	}
	if status := inferSubagentTaskProgressStatus(
		stringFromAny(block["last_tool_name"]) + " " + stringFromAny(block["description"]),
	); status != "" {
		task.Status = status
	}
}

func subagentTaskMetadata(message protocol.Message) map[string]any {
	return mapFromAny(message["metadata"])
}

func subagentTaskProgressBlocks(message protocol.Message) []map[string]any {
	blocks := make([]map[string]any, 0)
	switch content := message["content"].(type) {
	case []any:
		for _, item := range content {
			block := mapFromAny(item)
			if stringFromAny(block["type"]) == "task_progress" {
				blocks = append(blocks, block)
			}
		}
	case []map[string]any:
		for _, block := range content {
			if stringFromAny(block["type"]) == "task_progress" {
				blocks = append(blocks, block)
			}
		}
	}
	return blocks
}

func inferSubagentTaskProgressStatus(text string) string {
	normalized := normalizeSubagentTaskProgressStatusText(text)
	if normalized == "" {
		return ""
	}
	for _, marker := range []string{"failed to complete", "failed to finish", "could not complete", "could not finish"} {
		if containsSubagentTaskStatusMarker(normalized, marker) {
			return "failed"
		}
	}
	for _, marker := range []string{"incomplete", "unfinished", "not completed", "not complete", "not done", "not finished", "not yet completed", "not yet complete", "not yet done", "not yet finished", "未完成", "没完成"} {
		if containsSubagentTaskStatusMarker(normalized, marker) {
			return ""
		}
	}
	for _, marker := range []string{"completed", "complete", "finished", "done", "已完成", "完成"} {
		if containsSubagentTaskStatusMarker(normalized, marker) {
			return "completed"
		}
	}
	for _, marker := range []string{"failed", "error", "失败", "错误"} {
		if containsSubagentTaskStatusMarker(normalized, marker) {
			return "failed"
		}
	}
	for _, marker := range []string{"running", "in progress", "正在", "处理中"} {
		if containsSubagentTaskStatusMarker(normalized, marker) {
			return "running"
		}
	}
	return ""
}

func normalizeSubagentTaskProgressStatusText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	text = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return r
		}
		return ' '
	}, text)
	return strings.Join(strings.Fields(text), " ")
}

func containsSubagentTaskStatusMarker(text string, marker string) bool {
	if marker == "" {
		return false
	}
	for _, r := range marker {
		if r > 127 {
			return strings.Contains(text, marker)
		}
	}
	return strings.Contains(" "+text+" ", " "+marker+" ")
}

func setSubagentTaskString(target *string, source map[string]any, key string) {
	if target == nil || strings.TrimSpace(*target) != "" {
		return
	}
	if value := stringFromAny(source[key]); value != "" {
		*target = value
	}
}

func updateSubagentTaskString(target *string, source map[string]any, key string) {
	if target == nil {
		return
	}
	if value := stringFromAny(source[key]); value != "" {
		*target = value
	}
}

func readSubagentOutputFile(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSubagentOutputBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxSubagentOutputBytes {
		data = data[:maxSubagentOutputBytes]
	}
	return string(data), nil
}

func mapFromAny(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func int64FromAny(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}
