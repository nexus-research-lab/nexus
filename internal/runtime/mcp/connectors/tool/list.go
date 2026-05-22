package tool

import (
	"context"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/connectors/contract"
)

func list(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "connector_list",
		Description: "列出当前用户已连接的 connector。",
		SearchHint:  searchHintConnectorList,
		AlwaysLoad:  true,
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
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
