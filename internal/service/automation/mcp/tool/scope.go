package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/contract"
)

// ensureJobOwnedByCaller 校验 jobID 对应任务是否属于当前调用方。
// 主智能体豁免；普通 agent 仅可访问/修改自己 agent_id 的任务。
func ensureJobOwnedByCaller(ctx context.Context, svc contract.Service, sctx contract.ServerContext, jobID string) error {
	if sctx.IsMainAgent {
		return nil
	}
	caller := strings.TrimSpace(sctx.CurrentAgentID)
	if caller == "" {
		return fmt.Errorf("missing caller agent context")
	}
	job, err := svc.GetTask(ctx, jobID)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("scheduled task %s not found", jobID)
	}
	if strings.TrimSpace(job.AgentID) != caller {
		return fmt.Errorf("scheduled task %s belongs to another agent; only its owner or main agent can manage it", jobID)
	}
	return nil
}

// resolveListAgentID 决定 list_scheduled_tasks 的过滤条件。
// 主智能体支持显式过滤或全部列出；普通 agent 强制限定为自己。
func resolveListAgentID(sctx contract.ServerContext, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	caller := strings.TrimSpace(sctx.CurrentAgentID)
	if sctx.IsMainAgent {
		return requested, nil
	}
	if caller == "" {
		return "", fmt.Errorf("missing caller agent context")
	}
	if requested != "" && requested != caller {
		return "", fmt.Errorf("agent %s cannot list scheduled tasks of another agent", caller)
	}
	return caller, nil
}

// resolveCreateAgentID 决定 create_scheduled_task 的归属 agent_id。
// 主智能体可任意指定；普通 agent 强制为自己。
func resolveCreateAgentID(sctx contract.ServerContext, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	caller := strings.TrimSpace(sctx.CurrentAgentID)
	if sctx.IsMainAgent {
		if requested != "" {
			return requested, nil
		}
		if caller == "" {
			return "", fmt.Errorf("agent_id is required")
		}
		return caller, nil
	}
	if caller == "" {
		return "", fmt.Errorf("missing caller agent context")
	}
	if requested != "" && requested != caller {
		return "", fmt.Errorf("agent %s cannot create scheduled tasks for another agent", caller)
	}
	return caller, nil
}
