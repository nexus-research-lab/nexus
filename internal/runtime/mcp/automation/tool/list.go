package tool

import (
	"context"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/render"
)

func list(svc contract.Service, sctx contract.ServerContext) agentclient.MCPTool {
	return agentclient.MCPTool{
		Name:        "list_scheduled_tasks",
		Description: "列出定时任务。普通 agent 只能看到自己 agent_id 名下的任务；主智能体可传 agent_id 过滤或不传以列全部。",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"agent_id": map[string]any{"type": "string"}},
		},
		Annotations: &agentclient.MCPToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (agentclient.MCPToolResult, error) {
			filterAgentID, err := resolveListAgentID(sctx, argx.String(args, "agent_id"))
			if err != nil {
				return render.Error(err), nil
			}
			jobs, err := svc.ListTasks(ctx, filterAgentID)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(jobs, "")), nil
		},
	}
}
