package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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
)

// SubagentTask 表示父会话里可见的后台子 Agent 任务。
type SubagentTask struct {
	TaskID         string         `json:"task_id"`
	SessionKey     string         `json:"session_key,omitempty"`
	AgentID        string         `json:"agent_id,omitempty"`
	AgentType      string         `json:"agent_type,omitempty"`
	Description    string         `json:"description,omitempty"`
	Model          string         `json:"model,omitempty"`
	Name           string         `json:"name,omitempty"`
	ParentTaskID   string         `json:"parent_task_id,omitempty"`
	RoundID        string         `json:"round_id,omitempty"`
	Status         string         `json:"status"`
	TeamName       string         `json:"team_name,omitempty"`
	ToolUseID      string         `json:"tool_use_id,omitempty"`
	OutputFile     string         `json:"output_file,omitempty"`
	TranscriptPath string         `json:"transcript_path,omitempty"`
	Usage          map[string]any `json:"usage,omitempty"`
	StartedAt      int64          `json:"started_at,omitempty"`
	UpdatedAt      int64          `json:"updated_at,omitempty"`
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
func (s *Service) ListSubagentTasks(ctx context.Context, rawSessionKey string) ([]SubagentTask, error) {
	sessionKey, _, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, err
	}
	messages, err := s.GetSessionMessages(ctx, sessionKey)
	if err != nil {
		return nil, err
	}
	return buildSubagentTasks(sessionKey, messages), nil
}

// GetSubagentTaskMessages 读取指定 subagent task 的 transcript 和 output 摘要。
func (s *Service) GetSubagentTaskMessages(ctx context.Context, rawSessionKey string, taskID string) (*SubagentTaskMessages, error) {
	task, err := s.getSubagentTask(ctx, rawSessionKey, taskID)
	if err != nil {
		return nil, err
	}
	workspacePath := s.subagentTaskWorkspacePath(ctx, *task)
	messages, err := s.history.ReadTranscriptPathMessages(task.TranscriptPath, workspacePath, task.SessionKey, task.AgentID)
	if err != nil && !errors.Is(err, os.ErrNotExist) && strings.TrimSpace(task.TranscriptPath) != "" {
		return nil, err
	}
	output, err := readSubagentOutputFile(task.OutputFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return &SubagentTaskMessages{Task: *task, Messages: messages, Output: output}, nil
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
	if _, err := s.getSubagentTask(ctx, sessionKey, taskID); err != nil {
		return nil, err
	}
	if s.runtime == nil {
		return nil, ErrSubagentRuntimeUnavailable
	}
	if err := s.runtime.StopTask(ctx, sessionKey, taskID); err != nil {
		if runtimectx.IsRuntimeTransportClosedError(err) {
			return nil, fmt.Errorf("%w: %v", ErrSubagentRuntimeUnavailable, err)
		}
		return nil, err
	}
	return &SubagentTaskStopResult{Success: true, TaskID: taskID, Status: "stopped"}, nil
}

// SendSubagentTaskMessage 向 running subagent task 排队一条后续消息。
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
	if strings.TrimSpace(task.Status) != "running" {
		return nil, fmt.Errorf("%w: %s", ErrSubagentTaskNotRunning, task.Status)
	}
	if s.runtime == nil {
		return nil, ErrSubagentRuntimeUnavailable
	}
	if err := s.runtime.SendTaskMessage(ctx, sessionKey, taskID, message, subagentTaskMessageSummary(message)); err != nil {
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
	tasks, err := s.ListSubagentTasks(ctx, rawSessionKey)
	if err != nil {
		return nil, err
	}
	for index := range tasks {
		if tasks[index].TaskID == taskID {
			return &tasks[index], nil
		}
	}
	return nil, ErrSubagentTaskNotFound
}

func (s *Service) subagentTaskWorkspacePath(ctx context.Context, task SubagentTask) string {
	agentID := strings.TrimSpace(task.AgentID)
	if agentID == "" {
		if parsed := protocol.ParseSessionKey(task.SessionKey); parsed.AgentID != "" {
			agentID = parsed.AgentID
		}
	}
	if agentID == "" {
		return ""
	}
	workspacePaths, err := s.resolveWorkspacePaths(ctx, agentID)
	if err != nil || len(workspacePaths) == 0 {
		return ""
	}
	return workspacePaths[0]
}

func buildSubagentTasks(sessionKey string, messages []protocol.Message) []SubagentTask {
	tasks := map[string]*SubagentTask{}
	order := make([]string, 0)
	for _, message := range messages {
		metadata := subagentTaskMetadata(message)
		subtype := stringFromAny(metadata["subtype"])
		if subtype == "task_started" || subtype == "task_notification" || subtype == "task_updated" {
			taskID := stringFromAny(metadata["task_id"])
			if taskID != "" {
				task := ensureSubagentTask(tasks, &order, sessionKey, taskID)
				mergeSubagentTaskMessage(task, message, metadata, subtype)
			}
		}

		for _, block := range subagentTaskProgressBlocks(message) {
			taskID := stringFromAny(block["task_id"])
			if taskID == "" {
				continue
			}
			task := ensureSubagentTask(tasks, &order, sessionKey, taskID)
			mergeSubagentTaskProgress(task, message, block)
		}
	}
	results := make([]SubagentTask, 0, len(order))
	for _, taskID := range order {
		results = append(results, *tasks[taskID])
	}
	return results
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
	setSubagentTaskString(&task.AgentType, metadata, "agent_type")
	setSubagentTaskString(&task.Description, metadata, "description")
	if task.Description == "" {
		setSubagentTaskString(&task.Description, message, "content")
	}
	setSubagentTaskString(&task.Model, metadata, "model")
	setSubagentTaskString(&task.Name, metadata, "name")
	setSubagentTaskString(&task.ParentTaskID, metadata, "parent_task_id")
	setSubagentTaskString(&task.TeamName, metadata, "team_name")
	setSubagentTaskString(&task.OutputFile, metadata, "output_file")
	setSubagentTaskString(&task.TranscriptPath, metadata, "transcript_path")
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
	content, ok := message["content"].([]any)
	if !ok {
		return nil
	}
	blocks := make([]map[string]any, 0)
	for _, item := range content {
		block := mapFromAny(item)
		if stringFromAny(block["type"]) == "task_progress" {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

func inferSubagentTaskProgressStatus(text string) string {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return ""
	}
	for _, marker := range []string{"completed", "complete", "finished", "done", "已完成", "完成"} {
		if strings.Contains(normalized, marker) {
			return "completed"
		}
	}
	for _, marker := range []string{"failed", "error", "失败", "错误"} {
		if strings.Contains(normalized, marker) {
			return "failed"
		}
	}
	for _, marker := range []string{"running", "in_progress", "in progress", "正在", "处理中"} {
		if strings.Contains(normalized, marker) {
			return "running"
		}
	}
	return ""
}

func setSubagentTaskString(target *string, source map[string]any, key string) {
	if target == nil || strings.TrimSpace(*target) != "" {
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
