// INPUT: 当前 owner/session 下已聚合的 Subagent task 与其受限 transcript。
// OUTPUT: 只包含 exact ToolUse identity、名称、状态和时间的脱敏 ToolRun 历史。
// POS: 版本化 WorkGraph 迁移的 Session 只读边界；普通图读取不调用，不返回 prompt、input 或 output。
package session

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

// ReadSubagentHistoryForMigration 从现有 task transcript 恢复 exact Tool lifecycle。
// 仅用于版本化历史回填；不截断迁移输入，实时执行由 Bridge lifecycle 写入运行图。
func (s *Service) ReadSubagentHistoryForMigration(
	ctx context.Context,
	rawSessionKey string,
) ([]SubagentTask, []SubagentToolRun, error) {
	list, err := s.ListSubagentTasks(ctx, rawSessionKey)
	if err != nil {
		return nil, nil, err
	}
	result := make([]SubagentToolRun, 0)
	for _, task := range list.Items {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(task.ToolUseID) == "" {
			continue
		}
		workspacePath := s.subagentTaskWorkspacePath(ctx, task)
		messages, readErr := s.readSubagentTaskThreadForMigration(ctx, task, workspacePath)
		if readErr != nil {
			return nil, nil, readErr
		}
		runs := subagentToolRunsFromMessages(task, messages)
		result = append(result, runs...)
	}
	return list.Items, result, nil
}

// Interactive history tolerates missing child files; a one-time migration must
// retain the pending marker when an advertised path, link or exact ID cannot be read.
func (s *Service) readSubagentTaskThreadForMigration(ctx context.Context, task SubagentTask, workspacePath string) ([]protocol.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	owner, agent := authctx.OwnerUserID(ctx), subagentTaskHostAgentID(task)
	if path := strings.TrimSpace(task.TranscriptPath); path != "" {
		return s.history.ReadTranscriptPathMessagesForOwner(owner, path, workspacePath, task.SessionKey, agent)
	}
	if strings.EqualFold(task.TaskType, "local_agent") && task.OutputFile != "" {
		return s.history.ReadTranscriptLinkMessagesForOwner(owner, task.OutputFile, workspacePath, task.SessionKey, agent)
	}
	if id := subagentTaskTranscriptSessionID(task); id != "" {
		return s.history.ForOwner(owner).ReadTranscriptSessionMessages(workspacePath, id, task.SessionKey, agent)
	}
	return nil, nil
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
