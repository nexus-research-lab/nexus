// INPUT: 当前认证 owner/session 与 Session 服务的受限 Subagent task/ToolRun 历史。
// OUTPUT: 版本化 WorkGraph 回填需要的脱敏 task 与 Tool lifecycle；每个 Session transcript 只读一次。
// POS: 迁移的 Session 读取适配器；不承载 Tool 可见性、归属或 Execution 业务规则。
package migration

import (
	"context"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
)

// readWorkGraphHistory loads a session once and each child transcript once.
func readWorkGraphHistory(ctx context.Context, sessions *sessionsvc.Service, ownerUserID, sessionKey string) ([]orchestrationsvc.RuntimeGraphSubagentTaskHistory, []orchestrationsvc.RuntimeGraphSubagentToolHistory, error) {
	if sessions == nil || ownerUserID == "" || authctx.OwnerUserID(ctx) != ownerUserID {
		return nil, nil, fmt.Errorf("WorkGraph history owner/session service unavailable")
	}
	tasks, runs, err := sessions.ReadSubagentHistoryForMigration(ctx, sessionKey)
	if err != nil {
		return nil, nil, err
	}
	taskHistory := make([]orchestrationsvc.RuntimeGraphSubagentTaskHistory, 0, len(tasks))
	for _, task := range tasks {
		taskHistory = append(taskHistory, orchestrationsvc.RuntimeGraphSubagentTaskHistory{
			TaskID: task.TaskID, AgentID: task.AgentID, AgentType: task.AgentType,
			ChildSessionID: task.ChildSessionID, Description: task.Description,
			Summary: task.Summary, Name: task.Name, Status: task.Status,
			ToolUseID: task.ToolUseID, StartedAt: task.StartedAt, UpdatedAt: task.UpdatedAt,
		})
	}
	toolHistory := make([]orchestrationsvc.RuntimeGraphSubagentToolHistory, 0, len(runs))
	for _, run := range runs {
		toolHistory = append(toolHistory, orchestrationsvc.RuntimeGraphSubagentToolHistory{
			ParentToolUseID: run.ParentToolUseID, TaskID: run.TaskID, AgentID: run.AgentID,
			ToolUseID: run.ToolUseID, Name: run.Name, Status: run.Status,
			StartedAt: run.StartedAt, FinishedAt: run.FinishedAt,
		})
	}
	return taskHistory, toolHistory, nil
}
