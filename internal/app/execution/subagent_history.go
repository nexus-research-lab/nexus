// INPUT: 当前认证 owner/session 与 Session 服务的受限 Subagent task/ToolRun 历史。
// OUTPUT: Orchestration WorkGraph 兼容投影需要的脱敏 task 与 Tool lifecycle port。
// POS: app 装配适配器；不承载 Tool 可见性、归属或 Execution 业务规则。
package execution

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
)

type subagentToolHistory struct {
	sessions *sessionsvc.Service
}

// NewSubagentToolHistory 创建 runtime graph 使用的 Subagent 历史投影器。
func NewSubagentToolHistory(sessions *sessionsvc.Service) *subagentToolHistory {
	return &subagentToolHistory{sessions: sessions}
}

func (adapter subagentToolHistory) ListRuntimeGraphSubagentTaskHistory(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) ([]orchestrationsvc.RuntimeGraphSubagentTaskHistory, error) {
	if adapter.sessions == nil {
		return nil, fmt.Errorf("session service is nil")
	}
	if strings.TrimSpace(ownerUserID) == "" || authctx.OwnerUserID(ctx) != strings.TrimSpace(ownerUserID) {
		return nil, fmt.Errorf("subagent task history owner mismatch")
	}
	list, err := adapter.sessions.ListSubagentTasks(ctx, sessionKey)
	if err != nil {
		return nil, err
	}
	result := make([]orchestrationsvc.RuntimeGraphSubagentTaskHistory, 0, len(list.Items))
	for _, task := range list.Items {
		result = append(result, orchestrationsvc.RuntimeGraphSubagentTaskHistory{
			TaskID:         task.TaskID,
			AgentID:        task.AgentID,
			AgentType:      task.AgentType,
			ChildSessionID: task.ChildSessionID,
			Description:    task.Description,
			Summary:        task.Summary,
			Name:           task.Name,
			Status:         task.Status,
			ToolUseID:      task.ToolUseID,
			StartedAt:      task.StartedAt,
			UpdatedAt:      task.UpdatedAt,
		})
	}
	return result, nil
}

func (adapter subagentToolHistory) ListRuntimeGraphSubagentToolHistory(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) ([]orchestrationsvc.RuntimeGraphSubagentToolHistory, error) {
	if adapter.sessions == nil {
		return nil, fmt.Errorf("session service is nil")
	}
	if strings.TrimSpace(ownerUserID) == "" || authctx.OwnerUserID(ctx) != strings.TrimSpace(ownerUserID) {
		return nil, fmt.Errorf("subagent Tool history owner mismatch")
	}
	runs, err := adapter.sessions.ListSubagentToolRuns(ctx, sessionKey)
	if err != nil {
		return nil, err
	}
	result := make([]orchestrationsvc.RuntimeGraphSubagentToolHistory, 0, len(runs))
	for _, run := range runs {
		result = append(result, orchestrationsvc.RuntimeGraphSubagentToolHistory{
			ParentToolUseID: run.ParentToolUseID,
			TaskID:          run.TaskID,
			AgentID:         run.AgentID,
			ToolUseID:       run.ToolUseID,
			Name:            run.Name,
			Status:          run.Status,
			StartedAt:       run.StartedAt,
			FinishedAt:      run.FinishedAt,
		})
	}
	return result, nil
}
