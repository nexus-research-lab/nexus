// INPUT: 当前 owner/session 下已聚合的 Subagent task 与其受限 transcript。
// OUTPUT: 只包含 exact ToolUse identity、名称、状态和时间的脱敏 ToolRun 历史。
// POS: Session 历史到 WorkGraph 兼容投影的只读边界；不返回 prompt、input 或 output。
package session

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const subagentToolRunProjectionLimit = 256

// SubagentToolRun 是从已有 Subagent transcript 恢复的最小 Tool lifecycle。
// ParentToolUseID 精确指向父 Agent 启动该 Subagent 的 ToolUse。
type SubagentToolRun struct {
	ParentToolUseID string `json:"parent_tool_use_id"`
	TaskID          string `json:"task_id"`
	AgentID         string `json:"agent_id,omitempty"`
	ToolUseID       string `json:"tool_use_id"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	StartedAt       int64  `json:"started_at,omitempty"`
	FinishedAt      int64  `json:"finished_at,omitempty"`
}

// ListSubagentToolRuns 从现有 task transcript 恢复 exact Tool lifecycle。
// 该读取用于旧执行的只读兼容投影；实时执行仍由 Bridge lifecycle 写入运行图。
func (s *Service) ListSubagentToolRuns(
	ctx context.Context,
	rawSessionKey string,
) ([]SubagentToolRun, error) {
	list, err := s.ListSubagentTasks(ctx, rawSessionKey)
	if err != nil {
		return nil, err
	}
	result := make([]SubagentToolRun, 0)
	for _, task := range list.Items {
		if strings.TrimSpace(task.ToolUseID) == "" {
			continue
		}
		workspacePath := s.subagentTaskWorkspacePath(ctx, task)
		messages, _, readErr := s.readOwnerSubagentTaskThread(ctx, task, workspacePath)
		if readErr != nil {
			return nil, readErr
		}
		remaining := subagentToolRunProjectionLimit - len(result)
		if remaining <= 0 {
			break
		}
		runs := subagentToolRunsFromMessages(task, messages)
		if len(runs) > remaining {
			runs = runs[:remaining]
		}
		result = append(result, runs...)
	}
	return result, nil
}

func subagentToolRunsFromMessages(
	task SubagentTask,
	messages []protocol.Message,
) []SubagentToolRun {
	result := make([]SubagentToolRun, 0)
	indexByToolUseID := make(map[string]int)
	for _, message := range messages {
		timestamp := protocol.Int64FromAny(message["timestamp"])
		for _, block := range subagentTaskContentBlocks(message) {
			switch strings.ToLower(stringFromAny(block["type"])) {
			case "tool_use":
				toolUseID := stringFromAny(block["id"])
				name := stringFromAny(block["name"])
				if toolUseID == "" || name == "" {
					continue
				}
				if index, exists := indexByToolUseID[toolUseID]; exists {
					if result[index].Name == "" {
						result[index].Name = name
					}
					if result[index].StartedAt == 0 {
						result[index].StartedAt = timestamp
					}
					continue
				}
				indexByToolUseID[toolUseID] = len(result)
				result = append(result, SubagentToolRun{
					ParentToolUseID: strings.TrimSpace(task.ToolUseID),
					TaskID:          strings.TrimSpace(task.TaskID),
					AgentID:         strings.TrimSpace(task.AgentID),
					ToolUseID:       toolUseID,
					Name:            name,
					Status:          "running",
					StartedAt:       timestamp,
				})
			case "tool_result":
				toolUseID := stringFromAny(block["tool_use_id"])
				index, exists := indexByToolUseID[toolUseID]
				if !exists {
					continue
				}
				result[index].Status = "succeeded"
				if failed, _ := block["is_error"].(bool); failed {
					result[index].Status = "failed"
				}
				result[index].FinishedAt = timestamp
			}
		}
	}
	return result
}
