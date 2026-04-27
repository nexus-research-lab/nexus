package tool

import (
	"context"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"

	"github.com/nexus-research-lab/nexus/internal/service/connectors/mcp/contract"
)

func list(svc contract.Service, sctx contract.ServerContext) agentclient.MCPTool {
	return agentclient.MCPTool{
		Name:        "connector_list",
		Description: "列出当前用户已连接的 connector。",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Annotations: &agentclient.MCPToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (agentclient.MCPToolResult, error) {
			items, err := svc.ListActiveConnections(ctx, sctx.OwnerUserID)
			if err != nil {
				return errorResult(err), nil
			}
			result := make([]map[string]string, 0, len(items))
			for _, item := range items {
				result = append(result, map[string]string{
					"connector_id": item.ConnectorID,
					"auth_type":    item.AuthType,
					"api_base_url": item.APIBaseURL,
				})
			}
			return jsonResult(result), nil
		},
	}
}
