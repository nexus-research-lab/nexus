// INPUT: 当前认证 owner/session 与 Session 服务的受限 Subagent ToolRun 历史。
// OUTPUT: Orchestration WorkGraph 兼容投影需要的脱敏 Tool lifecycle port。
// POS: app 装配适配器；不承载 Tool 可见性、归属或 Execution 业务规则。
package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
)

type executionSubagentToolHistory struct {
	sessions *sessionsvc.Service
}

func (adapter executionSubagentToolHistory) ListRuntimeGraphSubagentToolHistory(
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
