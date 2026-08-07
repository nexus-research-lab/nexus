// INPUT: 无模型可控 scope。
// OUTPUT: 服务端实时解析的 authority、允许操作及排除边界。
// POS: nexus_manager 首选能力发现工具。
package tool

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/mcp/manager/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func inspectCapabilities(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name: "inspect_nexus_manager",
		Description: "读取当前对话真正获得的 Nexus 资源管理边界。" +
			"主智能体私有 DM 可查 owner 资源并创建基础 Room/对话；普通 Agent 只读自身 workspace；Room 成员只读当前 Room/对话。" +
			"配置修改和 Agent 创建请使用 nexus_config；本服务不提供删除、凭据、用户/Auth、Automation 或 raw nexusctl。",
		SearchHint:  "Nexus manager resources agents rooms conversations sessions workspace permission boundary",
		AlwaysLoad:  true,
		InputSchema: emptySchema(),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, _ map[string]any) (sdktool.ToolResult, error) {
			result, err := svc.InspectCapabilities(ctx, sctx.Actor())
			if err != nil {
				return errorResult(err), nil
			}
			return jsonResult(result), nil
		},
	}
}
