package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/automation/mcp/contract"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/render"
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
)

func list(svc contract.Service) agentclient.MCPTool {
	return agentclient.MCPTool{
		Name:        "list_scheduled_tasks",
		Description: "列出某个智能体或全部定时任务。未传 agent_id 则列全部。",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"agent_id": map[string]any{"type": "string"}},
		},
		Annotations: &agentclient.MCPToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (agentclient.MCPToolResult, error) {
			jobs, err := svc.ListTasks(ctx, argx.String(args, "agent_id"))
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(jobs, "")), nil
		},
	}
}
